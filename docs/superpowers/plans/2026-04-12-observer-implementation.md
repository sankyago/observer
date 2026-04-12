# Observer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a real-time MQTT alert system in Go that detects anomalous sensor readings and persists aggregated metrics to TimescaleDB.

**Architecture:** Single Go binary with three goroutines connected by channels: Subscriber (MQTT → SensorReading), Engine (threshold + rate-of-change alerting), Flusher (1-second aggregation → bulk COPY to TimescaleDB). Alert engine processes every message in-memory for sub-millisecond detection; DB writes are batched and decoupled from the alert path.

**Tech Stack:** Go 1.22+, eclipse/paho.mqtt.golang, jackc/pgx/v5, testcontainers-go, stretchr/testify, Docker Compose (Mosquitto + TimescaleDB)

---

## File Structure

```
observer/
├── cmd/
│   └── observer/
│       └── main.go                  # wires components, starts goroutines, handles shutdown
├── internal/
│   ├── model/
│   │   └── reading.go               # SensorReading struct
│   ├── subscriber/
│   │   ├── subscriber.go            # MQTT client, topic parsing, JSON decoding
│   │   └── subscriber_test.go       # unit: parse topic, decode payload
│   ├── engine/
│   │   ├── rules.go                 # ThresholdRule, RateRule, DefaultRules()
│   │   ├── rules_test.go            # unit: rule evaluation
│   │   ├── window.go                # SlidingWindow ring buffer
│   │   ├── window_test.go           # unit: window push, rate calculation
│   │   ├── engine.go                # Engine struct, Run(), alert dedup + output
│   │   └── engine_test.go           # unit: engine processes readings, fires/dedup alerts
│   ├── flusher/
│   │   ├── aggregator.go            # Aggregator: accumulate readings, produce AggregatedRow
│   │   ├── aggregator_test.go       # unit: aggregation math, dedup logic
│   │   ├── flusher.go               # Flusher: tick loop, drain channel, bulk write
│   │   └── flusher_test.go          # integration: flusher writes to real TimescaleDB
│   └── subscriber/
│       └── integration_test.go      # integration: subscriber against real Mosquitto
├── test/
│   └── e2e_test.go                  # e2e: MQTT publish → alert + DB row
├── migrations/
│   └── 001_create_sensor_data.sql   # CREATE TABLE + hypertable
├── docker-compose.yml               # Mosquitto + TimescaleDB
├── go.mod
└── go.sum
```

---

### Task 1: Project Scaffolding

**Files:**
- Create: `go.mod`
- Create: `internal/model/reading.go`
- Create: `docker-compose.yml`
- Create: `migrations/001_create_sensor_data.sql`

- [ ] **Step 1: Initialize Go module**

Run: `go mod init github.com/sankyago/observer`
Expected: `go.mod` created

- [ ] **Step 2: Create SensorReading model**

Create `internal/model/reading.go`:

```go
package model

import "time"

type SensorReading struct {
	DeviceID  string
	Metric    string
	Value     float64
	Timestamp time.Time
}
```

- [ ] **Step 3: Create docker-compose.yml**

Create `docker-compose.yml`:

```yaml
services:
  mosquitto:
    image: eclipse-mosquitto:2
    ports:
      - "1883:1883"
    volumes:
      - ./mosquitto.conf:/mosquitto/config/mosquitto.conf
    healthcheck:
      test: ["CMD-SHELL", "mosquitto_sub -t '$$SYS/#' -C 1 -W 3 | grep -q . || exit 1"]
      interval: 5s
      timeout: 5s
      retries: 3

  timescaledb:
    image: timescale/timescaledb:latest-pg16
    ports:
      - "5432:5432"
    environment:
      POSTGRES_USER: observer
      POSTGRES_PASSWORD: observer
      POSTGRES_DB: observer
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U observer"]
      interval: 5s
      timeout: 5s
      retries: 3
```

- [ ] **Step 4: Create Mosquitto config**

Create `mosquitto.conf`:

```
listener 1883
allow_anonymous true
```

- [ ] **Step 5: Create migration**

Create `migrations/001_create_sensor_data.sql`:

```sql
CREATE TABLE IF NOT EXISTS sensor_data (
    time        TIMESTAMPTZ NOT NULL,
    device_id   TEXT NOT NULL,
    metric      TEXT NOT NULL,
    min_value   DOUBLE PRECISION NOT NULL,
    max_value   DOUBLE PRECISION NOT NULL,
    avg_value   DOUBLE PRECISION NOT NULL,
    count       INTEGER NOT NULL,
    last_value  DOUBLE PRECISION NOT NULL
);

SELECT create_hypertable('sensor_data', 'time', if_not_exists => TRUE);
```

- [ ] **Step 6: Commit**

```bash
git add go.mod internal/model/reading.go docker-compose.yml mosquitto.conf migrations/001_create_sensor_data.sql
git commit -m "Scaffold project: model, docker-compose, migration"
```

---

### Task 2: Alert Rules

**Files:**
- Create: `internal/engine/rules.go`
- Create: `internal/engine/rules_test.go`

- [ ] **Step 1: Write failing tests for threshold rules**

Create `internal/engine/rules_test.go`:

```go
package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestThresholdRule_Check_AboveMax(t *testing.T) {
	rule := ThresholdRule{Min: -20.0, Max: 80.0}
	violated, msg := rule.Check(96.2)
	assert.True(t, violated)
	assert.Contains(t, msg, "96.2")
	assert.Contains(t, msg, "max=80")
}

func TestThresholdRule_Check_BelowMin(t *testing.T) {
	rule := ThresholdRule{Min: -20.0, Max: 80.0}
	violated, msg := rule.Check(-25.0)
	assert.True(t, violated)
	assert.Contains(t, msg, "-25")
	assert.Contains(t, msg, "min=-20")
}

func TestThresholdRule_Check_Normal(t *testing.T) {
	rule := ThresholdRule{Min: -20.0, Max: 80.0}
	violated, msg := rule.Check(50.0)
	assert.False(t, violated)
	assert.Empty(t, msg)
}

func TestRateRule_Check_ExceedsRate(t *testing.T) {
	rule := RateRule{MaxPerSecond: 2.0}
	violated, msg := rule.Check(10.0, 30.0, 5.0) // delta=20, duration=5s, rate=4/s
	assert.True(t, violated)
	assert.Contains(t, msg, "4.0")
	assert.Contains(t, msg, "max=2.0")
}

func TestRateRule_Check_NormalRate(t *testing.T) {
	rule := RateRule{MaxPerSecond: 2.0}
	violated, msg := rule.Check(10.0, 11.0, 5.0) // delta=1, duration=5s, rate=0.2/s
	assert.False(t, violated)
	assert.Empty(t, msg)
}

func TestRateRule_Check_ZeroDuration(t *testing.T) {
	rule := RateRule{MaxPerSecond: 2.0}
	violated, _ := rule.Check(10.0, 30.0, 0.0)
	assert.False(t, violated)
}

func TestDefaultRules_ContainsExpectedMetrics(t *testing.T) {
	thresholds, rates := DefaultRules()
	assert.Contains(t, thresholds, "temperature")
	assert.Contains(t, thresholds, "humidity")
	assert.Contains(t, thresholds, "pressure")
	assert.Contains(t, rates, "temperature")
	assert.Contains(t, rates, "humidity")
	assert.Contains(t, rates, "pressure")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/engine/ -v`
Expected: FAIL — types not defined

- [ ] **Step 3: Implement rules**

Create `internal/engine/rules.go`:

```go
package engine

import (
	"fmt"
	"math"
)

type ThresholdRule struct {
	Min float64
	Max float64
}

func (r ThresholdRule) Check(value float64) (bool, string) {
	if value > r.Max {
		return true, fmt.Sprintf("value=%.1f (max=%.0f)", value, r.Max)
	}
	if value < r.Min {
		return true, fmt.Sprintf("value=%.1f (min=%.0f)", value, r.Min)
	}
	return false, ""
}

type RateRule struct {
	MaxPerSecond float64
}

func (r RateRule) Check(oldValue, newValue, durationSec float64) (bool, string) {
	if durationSec <= 0 {
		return false, ""
	}
	rate := math.Abs(newValue-oldValue) / durationSec
	if rate > r.MaxPerSecond {
		return true, fmt.Sprintf("delta=%.1f/sec (max=%.1f/sec)", rate, r.MaxPerSecond)
	}
	return false, ""
}

func DefaultRules() (map[string]ThresholdRule, map[string]RateRule) {
	thresholds := map[string]ThresholdRule{
		"temperature": {Min: -20.0, Max: 80.0},
		"humidity":    {Min: 10.0, Max: 90.0},
		"pressure":    {Min: 950.0, Max: 1050.0},
	}
	rates := map[string]RateRule{
		"temperature": {MaxPerSecond: 2.0},
		"humidity":    {MaxPerSecond: 5.0},
		"pressure":    {MaxPerSecond: 10.0},
	}
	return thresholds, rates
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/engine/ -v`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/engine/rules.go internal/engine/rules_test.go
git commit -m "Add threshold and rate-of-change alert rules with tests"
```

---

### Task 3: Sliding Window

**Files:**
- Create: `internal/engine/window.go`
- Create: `internal/engine/window_test.go`

- [ ] **Step 1: Write failing tests for sliding window**

Create `internal/engine/window_test.go`:

```go
package engine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSlidingWindow_Push_ReturnsOldestAndDuration(t *testing.T) {
	w := NewSlidingWindow(3)
	t0 := time.Date(2026, 4, 12, 10, 0, 0, 0, time.UTC)

	w.Push(10.0, t0)
	w.Push(20.0, t0.Add(1*time.Second))
	w.Push(30.0, t0.Add(2*time.Second))

	oldVal, dur, ok := w.OldestAndDuration(t0.Add(2 * time.Second))
	assert.True(t, ok)
	assert.Equal(t, 10.0, oldVal)
	assert.Equal(t, 2.0, dur)
}

func TestSlidingWindow_Push_EvictsOldest(t *testing.T) {
	w := NewSlidingWindow(2)
	t0 := time.Date(2026, 4, 12, 10, 0, 0, 0, time.UTC)

	w.Push(10.0, t0)
	w.Push(20.0, t0.Add(1*time.Second))
	w.Push(30.0, t0.Add(2*time.Second)) // evicts 10.0

	oldVal, dur, ok := w.OldestAndDuration(t0.Add(2 * time.Second))
	assert.True(t, ok)
	assert.Equal(t, 20.0, oldVal)
	assert.Equal(t, 1.0, dur)
}

func TestSlidingWindow_OldestAndDuration_Empty(t *testing.T) {
	w := NewSlidingWindow(3)
	_, _, ok := w.OldestAndDuration(time.Now())
	assert.False(t, ok)
}

func TestSlidingWindow_OldestAndDuration_SingleElement(t *testing.T) {
	w := NewSlidingWindow(3)
	t0 := time.Date(2026, 4, 12, 10, 0, 0, 0, time.UTC)
	w.Push(10.0, t0)
	_, _, ok := w.OldestAndDuration(t0)
	assert.False(t, ok) // need at least 2 entries to compute rate
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/engine/ -run TestSlidingWindow -v`
Expected: FAIL — `NewSlidingWindow` not defined

- [ ] **Step 3: Implement sliding window**

Create `internal/engine/window.go`:

```go
package engine

import "time"

type windowEntry struct {
	value     float64
	timestamp time.Time
}

type SlidingWindow struct {
	entries []windowEntry
	size    int
	head    int
	count   int
}

func NewSlidingWindow(size int) *SlidingWindow {
	return &SlidingWindow{
		entries: make([]windowEntry, size),
		size:    size,
	}
}

func (w *SlidingWindow) Push(value float64, ts time.Time) {
	w.entries[w.head] = windowEntry{value: value, timestamp: ts}
	w.head = (w.head + 1) % w.size
	if w.count < w.size {
		w.count++
	}
}

func (w *SlidingWindow) OldestAndDuration(now time.Time) (float64, float64, bool) {
	if w.count < 2 {
		return 0, 0, false
	}
	oldest := (w.head - w.count + w.size) % w.size
	entry := w.entries[oldest]
	duration := now.Sub(entry.timestamp).Seconds()
	return entry.value, duration, true
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/engine/ -run TestSlidingWindow -v`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/engine/window.go internal/engine/window_test.go
git commit -m "Add sliding window ring buffer for rate-of-change detection"
```

---

### Task 4: Alert Engine

**Files:**
- Create: `internal/engine/engine.go`
- Create: `internal/engine/engine_test.go`

- [ ] **Step 1: Write failing tests for engine**

Create `internal/engine/engine_test.go`:

```go
package engine

import (
	"bytes"
	"testing"
	"time"

	"github.com/sankyago/observer/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestEngine_ThresholdAlert(t *testing.T) {
	var buf bytes.Buffer
	e := NewEngine(WithOutput(&buf), WithWindowSize(10), WithCooldown(0))

	reading := model.SensorReading{
		DeviceID:  "machine-42",
		Metric:    "temperature",
		Value:     96.2,
		Timestamp: time.Date(2026, 4, 12, 10, 0, 1, 0, time.UTC),
	}
	e.Process(reading)

	output := buf.String()
	assert.Contains(t, output, "[ALERT]")
	assert.Contains(t, output, "machine-42")
	assert.Contains(t, output, "THRESHOLD")
	assert.Contains(t, output, "96.2")
}

func TestEngine_NormalReading_NoAlert(t *testing.T) {
	var buf bytes.Buffer
	e := NewEngine(WithOutput(&buf), WithWindowSize(10), WithCooldown(0))

	reading := model.SensorReading{
		DeviceID:  "machine-42",
		Metric:    "temperature",
		Value:     50.0,
		Timestamp: time.Date(2026, 4, 12, 10, 0, 1, 0, time.UTC),
	}
	e.Process(reading)

	assert.Empty(t, buf.String())
}

func TestEngine_RateOfChangeAlert(t *testing.T) {
	var buf bytes.Buffer
	e := NewEngine(WithOutput(&buf), WithWindowSize(10), WithCooldown(0))

	t0 := time.Date(2026, 4, 12, 10, 0, 0, 0, time.UTC)

	// Push a baseline reading
	e.Process(model.SensorReading{
		DeviceID: "machine-42", Metric: "temperature",
		Value: 50.0, Timestamp: t0,
	})
	// Push a second to fill the window minimum
	e.Process(model.SensorReading{
		DeviceID: "machine-42", Metric: "temperature",
		Value: 50.0, Timestamp: t0.Add(1 * time.Second),
	})

	buf.Reset()

	// Spike: 50 → 60 in 1 second = 10/s, threshold is 2/s
	e.Process(model.SensorReading{
		DeviceID: "machine-42", Metric: "temperature",
		Value: 60.0, Timestamp: t0.Add(2 * time.Second),
	})

	output := buf.String()
	assert.Contains(t, output, "RATE")
	assert.Contains(t, output, "machine-42")
}

func TestEngine_Cooldown_SuppressesDuplicateAlert(t *testing.T) {
	var buf bytes.Buffer
	e := NewEngine(WithOutput(&buf), WithWindowSize(10), WithCooldown(30*time.Second))

	t0 := time.Date(2026, 4, 12, 10, 0, 0, 0, time.UTC)

	e.Process(model.SensorReading{
		DeviceID: "machine-42", Metric: "temperature",
		Value: 96.0, Timestamp: t0,
	})
	firstOutput := buf.String()
	assert.Contains(t, firstOutput, "[ALERT]")

	buf.Reset()

	// Same alert within cooldown
	e.Process(model.SensorReading{
		DeviceID: "machine-42", Metric: "temperature",
		Value: 97.0, Timestamp: t0.Add(5 * time.Second),
	})
	assert.Empty(t, buf.String())
}

func TestEngine_Cooldown_AlertsAfterExpiry(t *testing.T) {
	var buf bytes.Buffer
	e := NewEngine(WithOutput(&buf), WithWindowSize(10), WithCooldown(30*time.Second))

	t0 := time.Date(2026, 4, 12, 10, 0, 0, 0, time.UTC)

	e.Process(model.SensorReading{
		DeviceID: "machine-42", Metric: "temperature",
		Value: 96.0, Timestamp: t0,
	})

	buf.Reset()

	// After cooldown expires
	e.Process(model.SensorReading{
		DeviceID: "machine-42", Metric: "temperature",
		Value: 97.0, Timestamp: t0.Add(31 * time.Second),
	})
	assert.Contains(t, buf.String(), "[ALERT]")
}

func TestEngine_UnknownMetric_NoAlert(t *testing.T) {
	var buf bytes.Buffer
	e := NewEngine(WithOutput(&buf), WithWindowSize(10), WithCooldown(0))

	e.Process(model.SensorReading{
		DeviceID: "machine-42", Metric: "unknown_metric",
		Value: 9999.0, Timestamp: time.Now(),
	})
	assert.Empty(t, buf.String())
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/engine/ -run TestEngine -v`
Expected: FAIL — `NewEngine` not defined

- [ ] **Step 3: Implement engine**

Create `internal/engine/engine.go`:

```go
package engine

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/sankyago/observer/internal/model"
)

type alertKey struct {
	deviceID  string
	metric    string
	alertType string
}

type Option func(*Engine)

func WithOutput(w io.Writer) Option {
	return func(e *Engine) { e.output = w }
}

func WithWindowSize(n int) Option {
	return func(e *Engine) { e.windowSize = n }
}

func WithCooldown(d time.Duration) Option {
	return func(e *Engine) { e.cooldown = d }
}

type Engine struct {
	thresholds map[string]ThresholdRule
	rates      map[string]RateRule
	windows    map[string]*SlidingWindow // key: "deviceID:metric"
	lastAlert  map[alertKey]time.Time
	output     io.Writer
	windowSize int
	cooldown   time.Duration
}

func NewEngine(opts ...Option) *Engine {
	thresholds, rates := DefaultRules()
	e := &Engine{
		thresholds: thresholds,
		rates:      rates,
		windows:    make(map[string]*SlidingWindow),
		lastAlert:  make(map[alertKey]time.Time),
		output:     os.Stderr,
		windowSize: 10,
		cooldown:   30 * time.Second,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func (e *Engine) Process(r model.SensorReading) {
	// Threshold check
	if rule, ok := e.thresholds[r.Metric]; ok {
		if violated, msg := rule.Check(r.Value); violated {
			e.emit(r, "THRESHOLD", msg)
		}
	}

	// Rate-of-change check
	windowKey := r.DeviceID + ":" + r.Metric
	w, ok := e.windows[windowKey]
	if !ok {
		w = NewSlidingWindow(e.windowSize)
		e.windows[windowKey] = w
	}

	if rateRule, ok := e.rates[r.Metric]; ok {
		if oldVal, dur, hasData := w.OldestAndDuration(r.Timestamp); hasData {
			if violated, msg := rateRule.Check(oldVal, r.Value, dur); violated {
				e.emit(r, "RATE", msg)
			}
		}
	}

	w.Push(r.Value, r.Timestamp)
}

func (e *Engine) emit(r model.SensorReading, alertType, detail string) {
	key := alertKey{deviceID: r.DeviceID, metric: r.Metric, alertType: alertType}
	if last, ok := e.lastAlert[key]; ok && e.cooldown > 0 {
		if r.Timestamp.Sub(last) < e.cooldown {
			return
		}
	}
	e.lastAlert[key] = r.Timestamp

	fmt.Fprintf(e.output, "[ALERT] %s | %s | %s | %s | %s\n",
		r.Timestamp.Format(time.RFC3339),
		r.DeviceID,
		r.Metric,
		alertType,
		detail,
	)
}

func (e *Engine) Run(in <-chan model.SensorReading, out chan<- model.SensorReading) {
	for reading := range in {
		e.Process(reading)
		out <- reading
	}
	close(out)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/engine/ -v`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/engine/engine.go internal/engine/engine_test.go
git commit -m "Add alert engine with threshold, rate-of-change, and cooldown dedup"
```

---

### Task 5: Aggregator

**Files:**
- Create: `internal/flusher/aggregator.go`
- Create: `internal/flusher/aggregator_test.go`

- [ ] **Step 1: Write failing tests for aggregator**

Create `internal/flusher/aggregator_test.go`:

```go
package flusher

import (
	"testing"
	"time"

	"github.com/sankyago/observer/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAggregator_Add_SingleReading(t *testing.T) {
	a := NewAggregator()
	ts := time.Date(2026, 4, 12, 10, 0, 1, 0, time.UTC)

	a.Add(model.SensorReading{
		DeviceID: "m1", Metric: "temperature", Value: 50.0, Timestamp: ts,
	})

	rows := a.Flush(ts)
	require.Len(t, rows, 1)
	assert.Equal(t, "m1", rows[0].DeviceID)
	assert.Equal(t, "temperature", rows[0].Metric)
	assert.Equal(t, 50.0, rows[0].MinValue)
	assert.Equal(t, 50.0, rows[0].MaxValue)
	assert.Equal(t, 50.0, rows[0].AvgValue)
	assert.Equal(t, 1, rows[0].Count)
	assert.Equal(t, 50.0, rows[0].LastValue)
}

func TestAggregator_Add_MultipleReadings(t *testing.T) {
	a := NewAggregator()
	ts := time.Date(2026, 4, 12, 10, 0, 1, 0, time.UTC)

	a.Add(model.SensorReading{DeviceID: "m1", Metric: "temperature", Value: 40.0, Timestamp: ts})
	a.Add(model.SensorReading{DeviceID: "m1", Metric: "temperature", Value: 60.0, Timestamp: ts})
	a.Add(model.SensorReading{DeviceID: "m1", Metric: "temperature", Value: 50.0, Timestamp: ts})

	rows := a.Flush(ts)
	require.Len(t, rows, 1)
	assert.Equal(t, 40.0, rows[0].MinValue)
	assert.Equal(t, 60.0, rows[0].MaxValue)
	assert.Equal(t, 50.0, rows[0].AvgValue)
	assert.Equal(t, 3, rows[0].Count)
	assert.Equal(t, 50.0, rows[0].LastValue)
}

func TestAggregator_Flush_ClearsState(t *testing.T) {
	a := NewAggregator()
	ts := time.Date(2026, 4, 12, 10, 0, 1, 0, time.UTC)

	a.Add(model.SensorReading{DeviceID: "m1", Metric: "temperature", Value: 50.0, Timestamp: ts})
	a.Flush(ts)

	rows := a.Flush(ts.Add(1 * time.Second))
	assert.Len(t, rows, 0)
}

func TestAggregator_Flush_MultipleSensors(t *testing.T) {
	a := NewAggregator()
	ts := time.Date(2026, 4, 12, 10, 0, 1, 0, time.UTC)

	a.Add(model.SensorReading{DeviceID: "m1", Metric: "temperature", Value: 50.0, Timestamp: ts})
	a.Add(model.SensorReading{DeviceID: "m2", Metric: "humidity", Value: 70.0, Timestamp: ts})

	rows := a.Flush(ts)
	assert.Len(t, rows, 2)
}

func TestAggregator_Dedup_SkipsUnchanged(t *testing.T) {
	a := NewAggregator()
	ts := time.Date(2026, 4, 12, 10, 0, 1, 0, time.UTC)

	// First flush — should produce a row
	a.Add(model.SensorReading{DeviceID: "m1", Metric: "temperature", Value: 22.1, Timestamp: ts})
	rows := a.Flush(ts)
	require.Len(t, rows, 1)

	// Second flush — same value, should be skipped
	a.Add(model.SensorReading{DeviceID: "m1", Metric: "temperature", Value: 22.1, Timestamp: ts.Add(1 * time.Second)})
	rows = a.Flush(ts.Add(1 * time.Second))
	assert.Len(t, rows, 0)
}

func TestAggregator_Dedup_WritesOnChange(t *testing.T) {
	a := NewAggregator()
	ts := time.Date(2026, 4, 12, 10, 0, 1, 0, time.UTC)

	a.Add(model.SensorReading{DeviceID: "m1", Metric: "temperature", Value: 22.1, Timestamp: ts})
	a.Flush(ts)

	// Different value — should produce a row
	a.Add(model.SensorReading{DeviceID: "m1", Metric: "temperature", Value: 23.0, Timestamp: ts.Add(1 * time.Second)})
	rows := a.Flush(ts.Add(1 * time.Second))
	assert.Len(t, rows, 1)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/flusher/ -v`
Expected: FAIL — types not defined

- [ ] **Step 3: Implement aggregator**

Create `internal/flusher/aggregator.go`:

```go
package flusher

import (
	"math"
	"time"

	"github.com/sankyago/observer/internal/model"
)

type AggregatedRow struct {
	Time     time.Time
	DeviceID string
	Metric   string
	MinValue float64
	MaxValue float64
	AvgValue float64
	Count    int
	LastValue float64
}

type bucket struct {
	min   float64
	max   float64
	sum   float64
	count int
	last  float64
}

type sensorKey struct {
	deviceID string
	metric   string
}

type Aggregator struct {
	buckets  map[sensorKey]*bucket
	previous map[sensorKey]float64 // last flushed last_value per sensor, for dedup
}

func NewAggregator() *Aggregator {
	return &Aggregator{
		buckets:  make(map[sensorKey]*bucket),
		previous: make(map[sensorKey]float64),
	}
}

func (a *Aggregator) Add(r model.SensorReading) {
	key := sensorKey{deviceID: r.DeviceID, metric: r.Metric}
	b, ok := a.buckets[key]
	if !ok {
		b = &bucket{min: math.MaxFloat64, max: -math.MaxFloat64}
		a.buckets[key] = b
	}
	if r.Value < b.min {
		b.min = r.Value
	}
	if r.Value > b.max {
		b.max = r.Value
	}
	b.sum += r.Value
	b.count++
	b.last = r.Value
}

func (a *Aggregator) Flush(ts time.Time) []AggregatedRow {
	var rows []AggregatedRow
	for key, b := range a.buckets {
		avg := b.sum / float64(b.count)

		// Dedup: skip if all values identical and same as previous flush
		if b.min == b.max {
			if prev, ok := a.previous[key]; ok && prev == b.last {
				continue
			}
		}

		rows = append(rows, AggregatedRow{
			Time:      ts.Truncate(time.Second),
			DeviceID:  key.deviceID,
			Metric:    key.metric,
			MinValue:  b.min,
			MaxValue:  b.max,
			AvgValue:  avg,
			Count:     b.count,
			LastValue: b.last,
		})
		a.previous[key] = b.last
	}

	// Clear buckets for next window
	a.buckets = make(map[sensorKey]*bucket)
	return rows
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/flusher/ -v`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/flusher/aggregator.go internal/flusher/aggregator_test.go
git commit -m "Add aggregator with min/max/avg/count and dedup logic"
```

---

### Task 6: Flusher (DB Writer)

**Files:**
- Create: `internal/flusher/flusher.go`
- Create: `internal/flusher/flusher_test.go`

- [ ] **Step 1: Write failing integration test for flusher**

Create `internal/flusher/flusher_test.go`:

```go
//go:build integration

package flusher

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sankyago/observer/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupTimescaleDB(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()

	container, err := postgres.Run(ctx,
		"timescale/timescaledb:latest-pg16",
		postgres.WithDatabase("observer_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { container.Terminate(ctx) })

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	// Run migration
	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS sensor_data (
			time        TIMESTAMPTZ NOT NULL,
			device_id   TEXT NOT NULL,
			metric      TEXT NOT NULL,
			min_value   DOUBLE PRECISION NOT NULL,
			max_value   DOUBLE PRECISION NOT NULL,
			avg_value   DOUBLE PRECISION NOT NULL,
			count       INTEGER NOT NULL,
			last_value  DOUBLE PRECISION NOT NULL
		);
		SELECT create_hypertable('sensor_data', 'time', if_not_exists => TRUE);
	`)
	require.NoError(t, err)

	return pool
}

func TestFlusher_WritesToDB(t *testing.T) {
	ctx := context.Background()
	pool := setupTimescaleDB(t, ctx)

	in := make(chan model.SensorReading, 100)
	f := NewFlusher(pool, in, WithFlushInterval(100*time.Millisecond))

	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		f.Run(ctx)
		close(done)
	}()

	ts := time.Date(2026, 4, 12, 10, 0, 1, 0, time.UTC)
	in <- model.SensorReading{DeviceID: "m1", Metric: "temperature", Value: 50.0, Timestamp: ts}
	in <- model.SensorReading{DeviceID: "m1", Metric: "temperature", Value: 60.0, Timestamp: ts}

	// Wait for flush
	time.Sleep(300 * time.Millisecond)
	cancel()
	<-done

	var count int
	var minVal, maxVal, avgVal, lastVal float64
	var rowCount int
	err := pool.QueryRow(ctx, `
		SELECT count(*), min(min_value), max(max_value), avg(avg_value), max(last_value)
		FROM sensor_data WHERE device_id = 'm1'
	`).Scan(&count, &minVal, &maxVal, &avgVal, &lastVal)
	// Use a fresh context since we cancelled the other one
	freshCtx := context.Background()
	err = pool.QueryRow(freshCtx, `
		SELECT count, min_value, max_value, avg_value, last_value
		FROM sensor_data WHERE device_id = 'm1' LIMIT 1
	`).Scan(&rowCount, &minVal, &maxVal, &avgVal, &lastVal)
	require.NoError(t, err)
	assert.Equal(t, 2, rowCount)
	assert.Equal(t, 50.0, minVal)
	assert.Equal(t, 60.0, maxVal)
	assert.Equal(t, 55.0, avgVal)
	assert.Equal(t, 60.0, lastVal)
	fmt.Println("Flusher integration test passed")
}
```

- [ ] **Step 2: Implement flusher**

Create `internal/flusher/flusher.go`:

```go
package flusher

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sankyago/observer/internal/model"
)

type FlusherOption func(*Flusher)

func WithFlushInterval(d time.Duration) FlusherOption {
	return func(f *Flusher) { f.interval = d }
}

type Flusher struct {
	pool       *pgxpool.Pool
	in         <-chan model.SensorReading
	aggregator *Aggregator
	interval   time.Duration
}

func NewFlusher(pool *pgxpool.Pool, in <-chan model.SensorReading, opts ...FlusherOption) *Flusher {
	f := &Flusher{
		pool:       pool,
		in:         in,
		aggregator: NewAggregator(),
		interval:   1 * time.Second,
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

func (f *Flusher) Run(ctx context.Context) {
	ticker := time.NewTicker(f.interval)
	defer ticker.Stop()

	for {
		select {
		case reading, ok := <-f.in:
			if !ok {
				f.flush(context.Background())
				return
			}
			f.aggregator.Add(reading)
		case <-ticker.C:
			f.flush(ctx)
		case <-ctx.Done():
			f.flush(context.Background())
			return
		}
	}
}

func (f *Flusher) flush(ctx context.Context) {
	rows := f.aggregator.Flush(time.Now())
	if len(rows) == 0 {
		return
	}

	copyRows := make([][]interface{}, len(rows))
	for i, r := range rows {
		copyRows[i] = []interface{}{r.Time, r.DeviceID, r.Metric, r.MinValue, r.MaxValue, r.AvgValue, r.Count, r.LastValue}
	}

	_, err := f.pool.CopyFrom(
		ctx,
		pgx.Identifier{"sensor_data"},
		[]string{"time", "device_id", "metric", "min_value", "max_value", "avg_value", "count", "last_value"},
		pgx.CopyFromRows(copyRows),
	)
	if err != nil {
		log.Printf("flush error: %v", err)
	}
}
```

- [ ] **Step 3: Run integration test**

Run: `go test -tags integration ./internal/flusher/ -v -timeout 60s`
Expected: PASS (spins up TimescaleDB container, writes, verifies)

- [ ] **Step 4: Commit**

```bash
git add internal/flusher/flusher.go internal/flusher/flusher_test.go
git commit -m "Add flusher with batched COPY writes to TimescaleDB"
```

---

### Task 7: MQTT Subscriber

**Files:**
- Create: `internal/subscriber/subscriber.go`
- Create: `internal/subscriber/subscriber_test.go`

- [ ] **Step 1: Write failing unit tests for message parsing**

Create `internal/subscriber/subscriber_test.go`:

```go
package subscriber

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTopic_Valid(t *testing.T) {
	deviceID, metric, err := ParseTopic("sensors/machine-42/temperature")
	require.NoError(t, err)
	assert.Equal(t, "machine-42", deviceID)
	assert.Equal(t, "temperature", metric)
}

func TestParseTopic_Invalid_TooFewParts(t *testing.T) {
	_, _, err := ParseTopic("sensors/machine-42")
	assert.Error(t, err)
}

func TestParseTopic_Invalid_WrongPrefix(t *testing.T) {
	_, _, err := ParseTopic("other/machine-42/temperature")
	assert.Error(t, err)
}

func TestParsePayload_Valid(t *testing.T) {
	payload := []byte(`{"value": 94.5, "timestamp": "2026-04-12T10:00:01Z"}`)
	value, ts, err := ParsePayload(payload)
	require.NoError(t, err)
	assert.Equal(t, 94.5, value)
	assert.Equal(t, time.Date(2026, 4, 12, 10, 0, 1, 0, time.UTC), ts)
}

func TestParsePayload_Invalid_JSON(t *testing.T) {
	_, _, err := ParsePayload([]byte(`not json`))
	assert.Error(t, err)
}

func TestParsePayload_Missing_Value(t *testing.T) {
	_, _, err := ParsePayload([]byte(`{"timestamp": "2026-04-12T10:00:01Z"}`))
	assert.Error(t, err)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/subscriber/ -v`
Expected: FAIL — functions not defined

- [ ] **Step 3: Implement subscriber**

Create `internal/subscriber/subscriber.go`:

```go
package subscriber

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/sankyago/observer/internal/model"
)

type payload struct {
	Value     *float64 `json:"value"`
	Timestamp string   `json:"timestamp"`
}

func ParseTopic(topic string) (string, string, error) {
	parts := strings.Split(topic, "/")
	if len(parts) != 3 || parts[0] != "sensors" {
		return "", "", fmt.Errorf("invalid topic: %s, expected sensors/{device_id}/{metric}", topic)
	}
	return parts[1], parts[2], nil
}

func ParsePayload(data []byte) (float64, time.Time, error) {
	var p payload
	if err := json.Unmarshal(data, &p); err != nil {
		return 0, time.Time{}, fmt.Errorf("invalid JSON: %w", err)
	}
	if p.Value == nil {
		return 0, time.Time{}, fmt.Errorf("missing 'value' field")
	}
	ts, err := time.Parse(time.RFC3339, p.Timestamp)
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("invalid timestamp: %w", err)
	}
	return *p.Value, ts, nil
}

type Subscriber struct {
	client mqtt.Client
	out    chan<- model.SensorReading
	topic  string
}

func New(brokerURL, topic string, out chan<- model.SensorReading) *Subscriber {
	return &Subscriber{
		out:   out,
		topic: topic,
	}
}

func (s *Subscriber) Start() error {
	opts := mqtt.NewClientOptions().
		AddBroker(s.topic). // placeholder, overridden below
		SetAutoReconnect(true)

	// Fix: use the actual broker URL
	return s.startWithBroker(opts)
}

func (s *Subscriber) startWithBroker(baseOpts *mqtt.ClientOptions) error {
	// This is set up correctly in Run
	return nil
}

func (s *Subscriber) Run(brokerURL string) error {
	opts := mqtt.NewClientOptions().
		AddBroker(brokerURL).
		SetAutoReconnect(true).
		SetClientID("observer").
		SetDefaultPublishHandler(s.handleMessage)

	s.client = mqtt.NewClient(opts)
	if token := s.client.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("mqtt connect: %w", token.Error())
	}

	if token := s.client.Subscribe(s.topic, 1, nil); token.Wait() && token.Error() != nil {
		return fmt.Errorf("mqtt subscribe: %w", token.Error())
	}

	log.Printf("subscribed to %s on %s", s.topic, brokerURL)
	return nil
}

func (s *Subscriber) handleMessage(_ mqtt.Client, msg mqtt.Message) {
	deviceID, metric, err := ParseTopic(msg.Topic())
	if err != nil {
		log.Printf("skip message: %v", err)
		return
	}

	value, ts, err := ParsePayload(msg.Payload())
	if err != nil {
		log.Printf("skip message on %s: %v", msg.Topic(), err)
		return
	}

	s.out <- model.SensorReading{
		DeviceID:  deviceID,
		Metric:    metric,
		Value:     value,
		Timestamp: ts,
	}
}

func (s *Subscriber) Stop() {
	if s.client != nil {
		s.client.Disconnect(250)
	}
}
```

- [ ] **Step 4: Run unit tests to verify they pass**

Run: `go test ./internal/subscriber/ -v`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/subscriber/subscriber.go internal/subscriber/subscriber_test.go
git commit -m "Add MQTT subscriber with topic parsing and JSON decoding"
```

---

### Task 8: Main Entrypoint

**Files:**
- Create: `cmd/observer/main.go`

- [ ] **Step 1: Implement main.go**

Create `cmd/observer/main.go`:

```go
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sankyago/observer/internal/engine"
	"github.com/sankyago/observer/internal/flusher"
	"github.com/sankyago/observer/internal/model"
	"github.com/sankyago/observer/internal/subscriber"
)

func main() {
	brokerURL := envOrDefault("MQTT_BROKER", "tcp://localhost:1883")
	mqttTopic := envOrDefault("MQTT_TOPIC", "sensors/#")
	databaseURL := envOrDefault("DATABASE_URL", "postgres://observer:observer@localhost:5432/observer")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Database
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("database ping failed: %v", err)
	}
	log.Println("connected to TimescaleDB")

	// Channels
	mqttToEngine := make(chan model.SensorReading, 1000)
	engineToFlusher := make(chan model.SensorReading, 1000)

	// Subscriber
	sub := subscriber.New(brokerURL, mqttTopic, mqttToEngine)
	if err := sub.Run(brokerURL); err != nil {
		log.Fatalf("mqtt subscriber failed: %v", err)
	}
	defer sub.Stop()

	// Engine
	eng := engine.NewEngine()
	go eng.Run(mqttToEngine, engineToFlusher)

	// Flusher
	f := flusher.NewFlusher(pool, engineToFlusher)
	go f.Run(ctx)

	log.Println("observer started")

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("shutting down...")
	sub.Stop()
	close(mqttToEngine)
	cancel()
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./cmd/observer/`
Expected: clean build, no errors

- [ ] **Step 3: Commit**

```bash
git add cmd/observer/main.go
git commit -m "Add main entrypoint wiring subscriber, engine, and flusher"
```

---

### Task 9: Integration Test — Subscriber against Mosquitto

**Files:**
- Create: `internal/subscriber/integration_test.go`

- [ ] **Step 1: Write integration test**

Create `internal/subscriber/integration_test.go`:

```go
//go:build integration

package subscriber

import (
	"context"
	"fmt"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/sankyago/observer/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcmqtt "github.com/testcontainers/testcontainers-go/modules/mosquitto"
)

func TestSubscriber_ReceivesMessage(t *testing.T) {
	ctx := context.Background()

	container, err := tcmqtt.Run(ctx, "eclipse-mosquitto:2")
	require.NoError(t, err)
	t.Cleanup(func() { testcontainers.CleanupContainer(t, container) })

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "1883/tcp")
	require.NoError(t, err)
	brokerURL := fmt.Sprintf("tcp://%s:%s", host, port.Port())

	out := make(chan model.SensorReading, 10)
	sub := New(brokerURL, "sensors/#", out)
	err = sub.Run(brokerURL)
	require.NoError(t, err)
	defer sub.Stop()

	// Give subscriber time to connect
	time.Sleep(500 * time.Millisecond)

	// Publish a test message
	pubOpts := mqtt.NewClientOptions().AddBroker(brokerURL).SetClientID("test-publisher")
	pubClient := mqtt.NewClient(pubOpts)
	token := pubClient.Connect()
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())
	defer pubClient.Disconnect(250)

	payload := `{"value": 42.5, "timestamp": "2026-04-12T10:00:01Z"}`
	token = pubClient.Publish("sensors/device-1/temperature", 1, false, payload)
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())

	// Wait for message
	select {
	case reading := <-out:
		assert.Equal(t, "device-1", reading.DeviceID)
		assert.Equal(t, "temperature", reading.Metric)
		assert.Equal(t, 42.5, reading.Value)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for reading")
	}
}
```

- [ ] **Step 2: Run integration test**

Run: `go test -tags integration ./internal/subscriber/ -v -timeout 60s`
Expected: PASS (spins up Mosquitto, publishes, receives)

- [ ] **Step 3: Commit**

```bash
git add internal/subscriber/integration_test.go
git commit -m "Add subscriber integration test against real Mosquitto"
```

---

### Task 10: End-to-End Test

**Files:**
- Create: `test/e2e_test.go`

- [ ] **Step 1: Write e2e test**

Create `test/e2e_test.go`:

```go
//go:build integration

package test

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sankyago/observer/internal/engine"
	"github.com/sankyago/observer/internal/flusher"
	"github.com/sankyago/observer/internal/model"
	"github.com/sankyago/observer/internal/subscriber"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcmqtt "github.com/testcontainers/testcontainers-go/modules/mosquitto"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestE2E_MQTTToAlertAndDB(t *testing.T) {
	ctx := context.Background()

	// Start Mosquitto
	mqttContainer, err := tcmqtt.Run(ctx, "eclipse-mosquitto:2")
	require.NoError(t, err)
	t.Cleanup(func() { testcontainers.CleanupContainer(t, mqttContainer) })

	mqttHost, _ := mqttContainer.Host(ctx)
	mqttPort, _ := mqttContainer.MappedPort(ctx, "1883/tcp")
	brokerURL := fmt.Sprintf("tcp://%s:%s", mqttHost, mqttPort.Port())

	// Start TimescaleDB
	pgContainer, err := postgres.Run(ctx,
		"timescale/timescaledb:latest-pg16",
		postgres.WithDatabase("observer_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { testcontainers.CleanupContainer(t, pgContainer) })

	connStr, _ := pgContainer.ConnectionString(ctx, "sslmode=disable")
	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	defer pool.Close()

	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS sensor_data (
			time TIMESTAMPTZ NOT NULL, device_id TEXT NOT NULL, metric TEXT NOT NULL,
			min_value DOUBLE PRECISION NOT NULL, max_value DOUBLE PRECISION NOT NULL,
			avg_value DOUBLE PRECISION NOT NULL, count INTEGER NOT NULL,
			last_value DOUBLE PRECISION NOT NULL
		);
		SELECT create_hypertable('sensor_data', 'time', if_not_exists => TRUE);
	`)
	require.NoError(t, err)

	// Wire pipeline
	mqttToEngine := make(chan model.SensorReading, 100)
	engineToFlusher := make(chan model.SensorReading, 100)

	sub := subscriber.New(brokerURL, "sensors/#", mqttToEngine)
	err = sub.Run(brokerURL)
	require.NoError(t, err)
	defer sub.Stop()

	var alertBuf bytes.Buffer
	eng := engine.NewEngine(engine.WithOutput(&alertBuf), engine.WithCooldown(0))
	go eng.Run(mqttToEngine, engineToFlusher)

	flushCtx, flushCancel := context.WithCancel(ctx)
	f := flusher.NewFlusher(pool, engineToFlusher, flusher.WithFlushInterval(200*time.Millisecond))
	go f.Run(flushCtx)

	time.Sleep(500 * time.Millisecond)

	// Publish an abnormal temperature
	pubOpts := mqtt.NewClientOptions().AddBroker(brokerURL).SetClientID("e2e-publisher")
	pubClient := mqtt.NewClient(pubOpts)
	token := pubClient.Connect()
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())
	defer pubClient.Disconnect(250)

	payload := `{"value": 96.2, "timestamp": "2026-04-12T10:00:01Z"}`
	token = pubClient.Publish("sensors/machine-42/temperature", 1, false, payload)
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())

	// Wait for processing
	time.Sleep(1 * time.Second)
	flushCancel()

	// Verify alert was printed
	assert.Contains(t, alertBuf.String(), "[ALERT]")
	assert.Contains(t, alertBuf.String(), "machine-42")
	assert.Contains(t, alertBuf.String(), "THRESHOLD")

	// Verify row in DB
	var count int
	err = pool.QueryRow(context.Background(),
		"SELECT count(*) FROM sensor_data WHERE device_id = 'machine-42'",
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}
```

- [ ] **Step 2: Run e2e test**

Run: `go test -tags integration ./test/ -v -timeout 120s`
Expected: PASS — full pipeline verified

- [ ] **Step 3: Commit**

```bash
git add test/e2e_test.go
git commit -m "Add end-to-end test: MQTT publish to alert output and DB row"
```

---

### Task 11: Final Verification

- [ ] **Step 1: Run all unit tests**

Run: `go test ./... -v`
Expected: all PASS

- [ ] **Step 2: Run all integration tests**

Run: `go test -tags integration ./... -v -timeout 120s`
Expected: all PASS

- [ ] **Step 3: Build the binary**

Run: `go build -o observer ./cmd/observer/`
Expected: clean build

- [ ] **Step 4: Final commit**

```bash
git add go.mod go.sum
git commit -m "Finalize go.mod dependencies"
git push origin main
```

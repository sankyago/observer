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

	// Spike: 50 → 60 in 1 second = 10/s (oldest is 50 at t0, duration=2s, rate=5/s), threshold is 2/s
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

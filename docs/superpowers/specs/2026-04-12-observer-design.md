# Observer — Real-Time MQTT Alert System

## Overview

A Go single-binary system that subscribes to MQTT sensor data, detects anomalies in real time, and persists aggregated metrics to TimescaleDB.

## Architecture

```
MQTT Broker (external)
       │
       ▼
┌─────────────────┐
│   Subscriber     │  subscribes to: sensors/+/+
│   (goroutine)    │  parses JSON → SensorReading
└───────┬─────────┘
        │ chan SensorReading
        ▼
┌─────────────────┐
│   Engine         │  static thresholds + rate-of-change
│   (goroutine)    │  prints alerts to terminal
│                  │  30s cooldown dedup per (device, metric, type)
└───────┬─────────┘
        │ chan SensorReading
        ▼
┌─────────────────┐
│   Flusher        │  1-second aggregation buckets
│   (goroutine)    │  dedup unchanged values
│                  │  bulk COPY to TimescaleDB
└─────────────────┘
```

## MQTT Topic Convention

Pattern: `sensors/{device_id}/{metric}`

Example: `sensors/machine-42/temperature`

## Message Payload

```json
{"value": 94.5, "timestamp": "2026-04-12T10:00:01Z"}
```

## Data Model

```go
type SensorReading struct {
    DeviceID  string
    Metric    string
    Value     float64
    Timestamp time.Time
}
```

## Alert Engine

### Static Thresholds (hardcoded defaults)

| Metric      | Min    | Max    |
|-------------|--------|--------|
| temperature | -20.0  | 80.0   |
| humidity    | 10.0   | 90.0   |
| pressure    | 950.0  | 1050.0 |

### Rate-of-Change Detection

- Sliding window of last 10 readings per (device, metric)
- Compare current value against oldest in window
- Alert if `|current - oldest| / time_delta` exceeds threshold per second:
  - temperature: 2.0 per second
  - humidity: 5.0 per second
  - pressure: 10.0 per second

### Alert Output Format

```
[ALERT] 2026-04-12T10:00:01Z | machine-42 | temperature | THRESHOLD | value=96.2 (max=80.0)
[ALERT] 2026-04-12T10:00:01Z | machine-42 | temperature | RATE | delta=15.3/sec (max=2.0/sec)
```

### Deduplication

Cooldown of 30 seconds per (device_id, metric, alert_type) tuple.

## Storage

### TimescaleDB Schema

```sql
CREATE TABLE sensor_data (
    time        TIMESTAMPTZ NOT NULL,
    device_id   TEXT NOT NULL,
    metric      TEXT NOT NULL,
    min_value   DOUBLE PRECISION,
    max_value   DOUBLE PRECISION,
    avg_value   DOUBLE PRECISION,
    count       INTEGER,
    last_value  DOUBLE PRECISION
);

SELECT create_hypertable('sensor_data', 'time');
```

### Batching Strategy

- Buffer readings in memory, keyed by (device_id, metric)
- Flush every 1 second as aggregated rows (min, max, avg, count, last)
- Skip write if all values identical to previous bucket (dedup)
- Use Postgres COPY protocol via pgx for bulk insert

### Connection

Single pgx connection pool. Configured via `DATABASE_URL` environment variable.

## Project Structure

```
observer/
├── cmd/
│   └── observer/
│       └── main.go
├── internal/
│   ├── subscriber/
│   │   ├── subscriber.go
│   │   └── subscriber_test.go
│   ├── engine/
│   │   ├── engine.go
│   │   ├── rules.go
│   │   ├── window.go
│   │   ├── engine_test.go
│   │   ├── rules_test.go
│   │   └── window_test.go
│   ├── flusher/
│   │   ├── flusher.go
│   │   ├── aggregator.go
│   │   ├── flusher_test.go
│   │   └── aggregator_test.go
│   └── model/
│       └── reading.go
├── migrations/
│   └── 001_create_sensor_data.sql
├── docker-compose.yml
├── go.mod
└── go.sum
```

## Dependencies

- `eclipse/paho.mqtt.golang` — MQTT client
- `jackc/pgx/v5` — Postgres driver with COPY support
- `testcontainers/testcontainers-go` — containers for integration tests
- `stretchr/testify` — test assertions

## Testing Strategy

- **Unit tests** — alert engine, sliding window, aggregator, message parsing. Pure logic, no external deps.
- **Integration tests** — batch flusher against real TimescaleDB, subscriber against real Mosquitto. Both via testcontainers-go. Gated behind `//go:build integration` tag.
- **End-to-end test** — publish MQTT message, verify alert output + DB row. Also behind integration tag.

All tests runnable with `go test ./...` (unit) or `go test -tags integration ./...` (all).

## Configuration

Environment variables:
- `MQTT_BROKER` — MQTT broker URL (default: `tcp://localhost:1883`)
- `MQTT_TOPIC` — topic pattern (default: `sensors/#`)
- `DATABASE_URL` — TimescaleDB connection string (default: `postgres://observer:observer@localhost:5432/observer`)

## Docker Compose (local dev)

- Mosquitto broker on port 1883
- TimescaleDB on port 5432

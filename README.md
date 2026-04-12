# Observer

Real-time MQTT alert system in Go. Subscribes to sensor data, detects anomalies (static thresholds + rate-of-change), prints alerts to the terminal, and persists 1-second aggregated metrics to TimescaleDB.

## Architecture

```
MQTT Broker → Subscriber → Engine → Flusher → TimescaleDB
                              │
                              └─→ stderr (alerts)
```

Three goroutines connected by channels in a single binary:

- **Subscriber** — connects to MQTT, parses `sensors/{device_id}/{metric}` topics and JSON payloads
- **Engine** — checks thresholds + rate-of-change, emits alerts with 30s cooldown dedup
- **Flusher** — batches readings into 1-second `min/max/avg/count` aggregates, dedupes unchanged values, bulk writes via `COPY`

The alert path runs in-memory (sub-millisecond); the DB path is decoupled and batched.

## Quick Start

```bash
docker compose up -d           # start Mosquitto + TimescaleDB
go build ./cmd/observer/       # build binary
./observer                     # run
```

Publish a test message:

```bash
mosquitto_pub -h localhost -t sensors/machine-42/temperature \
  -m '{"value": 96.2, "timestamp": "2026-04-12T10:00:01Z"}'
```

You should see:

```
[ALERT] 2026-04-12T10:00:01Z | machine-42 | temperature | THRESHOLD | value=96.2 (max=80)
```

## MQTT Format

- Topic: `sensors/{device_id}/{metric}`
- Payload: `{"value": <float>, "timestamp": "<RFC3339>"}`

Default rules (per metric):

| Metric      | Min    | Max    | Max rate/sec |
|-------------|--------|--------|--------------|
| temperature | -20.0  | 80.0   | 2.0          |
| humidity    | 10.0   | 90.0   | 5.0          |
| pressure    | 950.0  | 1050.0 | 10.0         |

## Configuration

| Variable        | Default                                                |
|-----------------|--------------------------------------------------------|
| `MQTT_BROKER`   | `tcp://localhost:1883`                                 |
| `MQTT_TOPIC`    | `sensors/#`                                            |
| `DATABASE_URL`  | `postgres://observer:observer@localhost:5432/observer` |

## Testing

```bash
go test ./...                                    # unit tests
go test -tags integration ./... -timeout 120s    # all tests (requires Docker)
```

## Project Layout

```
cmd/observer/         main entrypoint
internal/subscriber/  MQTT client, topic + JSON parsing
internal/engine/      alert engine, rules, sliding window
internal/flusher/     aggregation + batched COPY to TimescaleDB
internal/model/       shared types
migrations/           SQL schema
test/                 end-to-end tests
```

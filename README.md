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
docker compose up -d            # start Mosquitto + Postgres
go build ./cmd/observer/
./observer                      # listens on :8080
```

Create a flow:

```bash
curl -X POST localhost:8080/api/flows -H 'content-type: application/json' -d '{
  "name": "demo",
  "enabled": true,
  "graph": {
    "nodes": [
      {"id":"src","type":"mqtt_source","position":{"x":0,"y":0},"data":{"broker":"tcp://localhost:1883","topic":"sensors/#"}},
      {"id":"th","type":"threshold","position":{"x":200,"y":0},"data":{"min":0,"max":80}},
      {"id":"sink","type":"debug_sink","position":{"x":400,"y":0},"data":{}}
    ],
    "edges": [
      {"id":"e1","source":"src","target":"th"},
      {"id":"e2","source":"th","target":"sink"}
    ]
  }
}'
```

Stream its events: `websocat ws://localhost:8080/api/flows/<id>/events`.

## API

| Method | Path                       |
|--------|----------------------------|
| GET    | /api/health                |
| GET    | /api/flows                 |
| POST   | /api/flows                 |
| GET    | /api/flows/:id             |
| PUT    | /api/flows/:id             |
| DELETE | /api/flows/:id             |
| GET    | /api/flows/:id/events (WS) |

## Node Types

- `mqtt_source` — `{broker, topic, username?, password?}`
- `threshold` — `{min, max}`
- `rate_of_change` — `{max_per_second, window_size}`
- `debug_sink` — `{}`

## Configuration

| Variable       | Default                                                |
|----------------|--------------------------------------------------------|
| `HTTP_ADDR`    | `:8080`                                                |
| `DATABASE_URL` | `postgres://observer:observer@localhost:5432/observer` |

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

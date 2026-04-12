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

## Running

### Docker (full stack)

```bash
docker compose up -d            # start EMQX + Postgres
go build ./cmd/observer/
./observer                      # listens on :8080
```

Register a device and create a flow:

```bash
# 1. Register a device, note the returned token and id
curl -X POST localhost:8080/api/devices -H 'content-type: application/json' \
  -d '{"name":"pump-42"}'
# { "id": "d52a8f7e-...", "name": "pump-42", "token": "AbC123..." }

# 2. Create a flow that watches that device
curl -X POST localhost:8080/api/flows -H 'content-type: application/json' -d '{
  "name": "demo",
  "enabled": true,
  "graph": {
    "nodes": [
      {"id":"src","type":"device_source","position":{"x":0,"y":0},"data":{"device_id":"<id>"}},
      {"id":"th","type":"threshold","position":{"x":200,"y":0},"data":{"min":0,"max":80}},
      {"id":"sink","type":"debug_sink","position":{"x":400,"y":0},"data":{}}
    ],
    "edges": [
      {"id":"e1","source":"src","target":"th"},
      {"id":"e2","source":"th","target":"sink"}
    ]
  }
}'

# 3. Publish telemetry via EMQX (token as MQTT username, empty password)
mosquitto_pub -h localhost -p 1883 -u "<token>" \
  -t "v1/devices/<id>/telemetry" \
  -m '{"temperature":23.5,"humidity":60,"ts":"2026-04-12T10:00:00Z"}'
```

Stream flow events: `websocat ws://localhost:8080/api/flows/<id>/events`.

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

## Devices

Register a device to get a token:

```bash
curl -X POST localhost:8080/api/devices -H 'content-type: application/json' \
  -d '{"name":"pump-42"}'
# { "id": "d52a8f7e-...", "name": "pump-42", "token": "AbC123..." }
```

The device publishes telemetry to EMQX (`mqtt.observer.io:1883` in production,
`localhost:1883` via docker-compose):

- **MQTT username:** the device token
- **Topic:** `v1/devices/{id}/telemetry`
- **Payload:** `{"temperature": 23.5, "humidity": 60, "ts": "2026-04-12T10:00:00Z"}`

Observer's EMQX broker authenticates on CONNECT and authorizes on PUBLISH
against Observer's `/api/mqtt/auth` and `/api/mqtt/acl` HTTP hooks.

| Method | Path                              | Purpose               |
|--------|-----------------------------------|-----------------------|
| GET    | /api/devices                      | list                  |
| POST   | /api/devices                      | create, returns token |
| GET    | /api/devices/:id                  | get                   |
| PUT    | /api/devices/:id                  | rename                |
| DELETE | /api/devices/:id                  | delete                |
| POST   | /api/devices/:id/regenerate-token | rotate token          |

## Node Types

- `device_source` — `{device_id?, metric?}` (empty = all devices / all metrics)
- `threshold` — `{min, max}`
- `rate_of_change` — `{max_per_second, window_size}`
- `debug_sink` — `{}`

## Configuration

| Variable              | Default                                                |
|-----------------------|--------------------------------------------------------|
| `HTTP_ADDR`           | `:8080`                                                |
| `DATABASE_URL`        | `postgres://observer:observer@localhost:5432/observer` |
| `EMQX_BROKER_URL`     | `tcp://emqx:1883`                                      |
| `EMQX_USERNAME`       | `observer-consumer`                                    |
| `EMQX_PASSWORD`       | (required)                                             |
| `EMQX_SHARED_GROUP`   | `observer`                                             |
| `MQTT_WEBHOOK_SECRET` | (required)                                             |

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

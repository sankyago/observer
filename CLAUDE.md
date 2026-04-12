# Observer

Real-time MQTT alert system in Go. Subscribes to sensor data, detects anomalies, persists aggregated metrics to TimescaleDB.

## Project Structure

- `cmd/observer/` — API server entrypoint
- `internal/api/` — HTTP + WebSocket handlers (chi + gorilla)
- `internal/flow/` — flow service, graph types/validation, nodes, runtime, store
  - `graph/` — JSON types, validation, cycle detection
  - `nodes/` — node implementations (`mqtt_source`, `threshold`, `rate_of_change`, `debug_sink`)
  - `runtime/` — `CompiledFlow`, `FlowManager`, `EventBus`
  - `store/` — `FlowRepo` (pgx)
- `internal/db/` — migration runner
- `internal/subscriber/` — MQTT parsing helpers (reused by `mqtt_source`)
- `internal/engine/` — threshold/rate rules + sliding window (reused by nodes)
- `internal/flusher/` — aggregation + TimescaleDB writer (currently unwired; reserved for `timescale_sink`)
- `internal/model/` — shared data types
- `migrations/` — SQL migrations (run on startup)
- `test/` — end-to-end tests

## Commands

- `go test ./...` — run unit tests
- `go test -tags integration ./... -timeout 120s` — run all tests (requires Docker)
- `go build ./cmd/observer/` — build binary
- `docker compose up -d` — start Mosquitto + TimescaleDB for local dev

## Conventions

- snake_case for file names
- Go idiom for code (camelCase locals, PascalCase exports)
- No Co-Authored-By lines in commits
- Integration tests gated behind `//go:build integration` tag
- Topic format: `sensors/{device_id}/{metric}`

## Environment Variables

- `MQTT_BROKER` — default `tcp://localhost:1883`
- `MQTT_TOPIC` — default `sensors/#`
- `DATABASE_URL` — default `postgres://observer:observer@localhost:5432/observer`

# Observer

Real-time MQTT alert system in Go. Subscribes to sensor data, detects anomalies, persists aggregated metrics to TimescaleDB.

## Project Structure

- `cmd/observer/` — main entrypoint
- `internal/subscriber/` — MQTT client, topic parsing, JSON decoding
- `internal/engine/` — alert engine (threshold + rate-of-change detection)
- `internal/flusher/` — batched aggregation and bulk writes to TimescaleDB
- `internal/model/` — shared data types
- `migrations/` — SQL migrations
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

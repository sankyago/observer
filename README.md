# Observer

Multi-tenant telemetry platform: MQTT ingest → TimescaleDB → real-time threshold rules → action execution.

## Repo layout

- `cmd/all` — monolith entry point (Mode A)
- `cmd/transport` — MQTT consumer + rule evaluator + Timescale writer
- `cmd/runner` — action job consumer
- `cmd/api` — HTTP control plane
- `cmd/migrate` — goose migration runner
- `pkg/` — shared libraries (`config`, `log`, `db`, `models`, `queue`)
- `migrations/` — goose SQL migrations
- `deploy/docker-compose.yml` — local Postgres/Timescale + EMQX
- `internal/testutil/` — test helpers (testcontainers)
- `docs/superpowers/` — design docs and implementation plans

## Quickstart

```sh
make up           # starts Postgres+Timescale and EMQX
make migrate      # applies all migrations
make test-short   # unit tests (no containers)
make test         # full test suite (uses testcontainers, slower)
go run ./cmd/all  # boots the monolith stub
```

Required env vars (set by `make migrate` / `make test` defaults; override as needed):

- `OBSERVER_DB_DSN` — Postgres connection string
- `OBSERVER_MQTT_URL` — EMQX broker URL
- `OBSERVER_LOG_LEVEL` — `debug` | `info` | `warn` | `error` (default `info`)

## Design

See `docs/superpowers/specs/2026-04-14-telemetry-anomaly-platform-design.md`.

# Observer

Multi-tenant telemetry platform: MQTT ingest → TimescaleDB → real-time threshold rules → action execution.

See `docs/superpowers/specs/` for design and `docs/superpowers/plans/` for implementation plans.

## Local development

```sh
docker compose -f deploy/docker-compose.yml up -d
make migrate
make test
go run ./cmd/all
```

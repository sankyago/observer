# Flow Engine — Design

**Date:** 2026-04-12
**Branch:** `feat/flow-engine`
**Status:** approved

## Context

Observer is pivoting from a single-binary CLI to an API platform (ThingsBoard-style, lighter UX). Users will visually wire devices and automations in React-Flow; the backend compiles each graph into a runtime DAG. This spec covers the **first** slice: a REST+WS API serving persisted flows and a runtime that executes them against user-provided MQTT sources.

Single-tenant. No auth, no multi-tenancy, not now, not later in scope of this PR.

## Scope

In:
- Postgres-backed `flows` entity (CRUD).
- REST API for flow management.
- WebSocket stream of per-flow runtime events (readings + alerts).
- Runtime `FlowManager` that compiles a flow's JSON graph into a DAG of goroutines and reacts to CRUD/enable changes.
- Four node types: `mqtt_source`, `threshold`, `rate_of_change`, `debug_sink`.
- Graph validation on write (known types, valid config, no dangling edges, no cycles).

Out:
- Auth, users, multi-tenancy.
- Flow versioning / history.
- Additional node types (webhook sinks, transforms, timescale write-back) — extension points only.
- Reattaching the existing `internal/flusher` TimescaleDB aggregation path. Code remains in tree, unwired. A future `timescale_sink` node will reuse it.

## Architecture

```
HTTP/WS (chi) ──► FlowService ──► FlowRepo (pgx) ──► Postgres (flows)
                         │
                         └──► FlowManager ──► CompiledFlow (DAG of node goroutines)
                                                │
                                                └──► EventBus ──► WS subscribers
```

- **FlowRepo** — pgx-backed CRUD on `flows`.
- **FlowService** — validates graphs, persists via repo, tells the manager to start/stop/replace.
- **FlowManager** — owns running `CompiledFlow` instances keyed by flow ID. On startup, loads all enabled flows. Exposes `Start(id)`, `Stop(id)`, `Replace(id, graph)`.
- **CompiledFlow** — the graph compiled into node instances wired by Go channels. Each node runs in its own goroutine and emits events to a per-flow `EventBus`.
- **EventBus** — fan-out of `FlowEvent` (reading-passed-through-node, alert, node-error) to any number of WS subscribers for that flow.

Each package is small and testable in isolation:
- `internal/flow/graph` — JSON schema, validation, compilation.
- `internal/flow/nodes` — node types (each a tiny struct implementing a `Node` interface).
- `internal/flow/runtime` — `CompiledFlow`, `FlowManager`, `EventBus`.
- `internal/flow/store` — `FlowRepo`.
- `internal/api` — HTTP/WS handlers wiring service → transport.

## Data Model

```sql
CREATE TABLE flows (
    id         UUID PRIMARY KEY,
    name       TEXT NOT NULL,
    graph      JSONB NOT NULL,
    enabled    BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

## Graph JSON

React-Flow-native shape. Example:

```json
{
  "nodes": [
    {"id": "n1", "type": "mqtt_source", "position": {"x": 0, "y": 0},
     "data": {"broker": "tcp://localhost:1883", "topic": "sensors/machine-42/temperature"}},
    {"id": "n2", "type": "threshold", "position": {"x": 200, "y": 0},
     "data": {"min": -20, "max": 80}},
    {"id": "n3", "type": "debug_sink", "position": {"x": 400, "y": 0}, "data": {}}
  ],
  "edges": [
    {"id": "e1", "source": "n1", "target": "n2"},
    {"id": "e2", "source": "n2", "target": "n3"}
  ]
}
```

`position` is stored but ignored by the runtime.

## Node Types (v1)

| Type             | Inputs | Outputs | Config                                                | Behavior                                                 |
|------------------|--------|---------|-------------------------------------------------------|----------------------------------------------------------|
| `mqtt_source`    | —      | reading | `broker` (string), `topic` (string), `username?`, `password?` | Connects to broker, parses `sensors/{device}/{metric}` topics and `{value,timestamp}` JSON, emits `Reading` downstream. |
| `threshold`      | reading | reading + alert | `min` (float), `max` (float)                   | Passes reading through; emits alert event when `value < min` or `value > max`. |
| `rate_of_change` | reading | reading + alert | `max_per_second` (float), `window_size` (int)   | Sliding window; emits alert when `|Δvalue|/Δt > max_per_second`. |
| `debug_sink`     | reading/alert | — | —                                                    | Logs to stderr. Stand-in for future action nodes.         |

Alerts surface as `FlowEvent{kind:"alert", node_id, reading, detail}` on the EventBus.

## Compilation & Validation

On `POST /api/flows` and `PUT /api/flows/:id`, the service calls `graph.Compile(json)` which:
1. Parses and checks all node types are known.
2. Checks each node's `data` against the type's config schema.
3. Verifies every edge references existing nodes.
4. Runs topological sort; rejects cycles.
5. Returns a `CompiledFlow` (zero-cost if only validating — compile is cheap).

If validation fails, the HTTP response is `400` with `{error, details}`.

## Runtime Lifecycle

- **Server start:** `FlowManager.LoadEnabled()` lists all `enabled=true` flows and starts each.
- **Create enabled flow:** validate → persist → `manager.Start(id)`.
- **Update flow (graph changed):** validate → persist → `manager.Replace(id)` (stops old, starts new; no attempt at graceful state migration).
- **Enable/disable toggle:** `manager.Start` / `manager.Stop`.
- **Delete:** `manager.Stop` then remove from DB.
- **Server shutdown:** `manager.StopAll()` with context cancellation.

Each `CompiledFlow` owns its MQTT client(s). Two flows subscribing to the same broker/topic are fully independent — we don't dedup subscriptions. Keeps isolation simple; optimization is for later.

## API Surface

| Method | Path                         | Purpose                            |
|--------|------------------------------|------------------------------------|
| GET    | `/api/health`                | Liveness                           |
| GET    | `/api/flows`                 | List flows                         |
| POST   | `/api/flows`                 | Create flow (`{name, graph, enabled?}`) |
| GET    | `/api/flows/:id`             | Get one                            |
| PUT    | `/api/flows/:id`             | Update (`{name?, graph?, enabled?}`)    |
| DELETE | `/api/flows/:id`             | Delete                             |
| GET    | `/api/flows/:id/events` (WS) | Stream `FlowEvent`s for this flow  |

WS frames are JSON: `{kind: "reading"|"alert"|"error", node_id, data: {...}, ts}`.

Router: `go-chi/chi` (std-lib-ish, no heavy framework). WS: `gorilla/websocket`.

## Configuration

Existing env vars stay. `DATABASE_URL` reused. Add:
- `HTTP_ADDR` — default `:8080`.

MQTT broker/topic env vars become **unused** in this binary (sources are per-flow). Leave them defined but note as deprecated in README follow-up.

## Testing

- **Unit:** graph parse + validation (cycles, unknown types, bad config), each node type in isolation, `CompiledFlow` wiring, `EventBus` fan-out, `FlowRepo` against ephemeral Postgres (testcontainers).
- **Integration (`//go:build integration`):** full flow — POST a flow with `mqtt_source → threshold → debug_sink`, publish to real Mosquitto, assert WS receives the reading + alert events, assert row persists across restarts.
- **HTTP handler tests:** table-driven with `httptest`.

## Migration Strategy

- Add `migrations/002_create_flows.sql` (keep `001_create_sensor_data.sql` intact — unused in this binary for now, but preserved).
- New `cmd/observer/main.go` becomes the API server. The previous wiring (subscriber → engine → flusher) is removed from `main` but the packages stay on disk.
- Running DB migrations: a small `internal/db/migrate.go` that runs `*.sql` files in order on startup (no external tool).

## Risks / Open Questions

- **Reconnect behavior of `mqtt_source`**: paho client auto-reconnects by default. We surface connect errors as `FlowEvent{kind:"error"}` but don't tear down the flow.
- **WS backpressure**: slow consumers get dropped events (bounded per-subscriber channel; drop-oldest). Acceptable for debug UI.
- **Graph size**: no explicit cap. In practice <100 nodes is fine; revisit if users push further.

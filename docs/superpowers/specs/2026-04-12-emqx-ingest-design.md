# EMQX-Backed Device Ingest — Design

**Date:** 2026-04-12
**Branch:** `feat/emqx-ingest` (stacked on `feat/flow-engine`)
**Status:** approved

## Context

The current `feat/flow-engine` branch lets each flow open its own outbound MQTT connection to a user-supplied broker (`mqtt_source` node). That model is fragile (N connections per flow × brokers), leaks broker credentials into graph JSON, and puts the burden of operating a broker on the user.

This spec pivots to a ThingsBoard-style ingest model: **Observer owns the broker.** Devices publish to `mqtt.observer.io` authenticated by a per-device token, Observer receives everything through one in-process stream, and flows subscribe to *devices* (not raw topics). The broker is **EMQX**, operated as a separate service in docker-compose for self-hosters and as a managed cluster in production.

Single-tenant (no users/orgs). No auth on the REST API. Device tokens authenticate *devices*, not users.

## Scope

In:
- EMQX added to `docker-compose.yml`.
- HTTP auth + ACL hooks: `POST /api/mqtt/auth`, `POST /api/mqtt/acl`. EMQX calls these on CONNECT and on PUBLISH.
- `devices` entity: CRUD API, UI-friendly fields, opaque token generation.
- Observer connects to EMQX as a shared-subscription consumer on startup and forwards parsed readings into an in-process ingest channel.
- `internal/ingest/` package: EMQX client, parser, router.
- New flow node `device_source` replaces `mqtt_source`. Selector shape: `{device_id?: string, all?: bool}`.
- Removal of `mqtt_source` node and `internal/subscriber/` (code moves to `internal/ingest/` where still useful).

Out (explicit deferrals):
- Wiring TimescaleDB persistence from the ingest stream. `internal/flusher/` stays unwired; future PR.
- Device groups / tags (selector is device-id or all, nothing more).
- Authentication / multi-tenancy.
- Sticky shared-subscription routing and per-device flow-state partitioning (single-replica for now).
- MQTT 5 features beyond what the default EMQX/paho configuration gives us.
- Redis or external state for flow windows/cooldowns. Single-replica assumption holds.
- Prometheus `/metrics` endpoint. Observability added in a later PR once the core is shipping.
- UI changes. Follow-up PR rebases the UI onto this branch (changes `mqtt_source` → `device_source` in palette, adds devices page).

## Architecture

```
                      ┌─────────────────────────────┐
                      │  EMQX                       │
Device ─── MQTT ────► │  :1883 (mqtt), :8083 (ws)   │
(token in CONNECT     │                             │
 username field)      │  Auth hook  ────────────┐   │
                      │  ACL hook   ────────────┤   │
                      └───────┬─────────────────┤───┘
                              │ shared sub      │ HTTP
                              │ $share/observer │ /api/mqtt/{auth,acl}
                              │  /v1/devices/+/telemetry
                              ▼                 │
                      ┌─────────────────────────▼───┐
                      │  Observer                   │
                      │                             │
                      │  internal/ingest/           │
                      │    ├─ mqtt_consumer.go      │
                      │    ├─ parser.go             │
                      │    └─ router.go             │
                      │                             │
                      │  internal/api/              │
                      │    ├─ mqtt_auth_handler.go  │
                      │    ├─ devices_handler.go    │
                      │    └─ …                     │
                      │                             │
                      │  internal/devices/          │
                      │    ├─ service.go            │
                      │    └─ store/repo.go         │
                      │                             │
                      │  internal/flow/…  (mostly   │
                      │  unchanged; mqtt_source     │
                      │  replaced by device_source) │
                      └─────────────────────────────┘
```

**Units and responsibilities:**

- `internal/ingest/mqtt_consumer.go` — connects to EMQX, subscribes to `$share/observer/v1/devices/+/telemetry`, forwards raw messages into the parser. Auto-reconnect.
- `internal/ingest/parser.go` — pure function `parse(topic, payload) → []model.SensorReading`. Extracts the device UUID directly from the topic (`v1/devices/{uuid}/telemetry`). No I/O, fully unit-testable.
- `internal/ingest/router.go` — receives parsed readings, fans out to subscribed flows. Holds the registry of "which running flows want readings from which devices."
- `internal/devices/service.go` + `store/repo.go` — pgx-backed CRUD, token generation (`crypto/rand`, base64url, 20 bytes).
- `internal/api/mqtt_auth_handler.go` — EMQX webhook endpoints. Validates tokens, returns `allow`/`deny`. Enforces ACL: a device may only publish to its own `devices/{its-uuid}/telemetry`.
- `internal/api/devices_handler.go` — REST CRUD for devices.

## Data Model

New table:

```sql
CREATE TABLE devices (
    id         UUID PRIMARY KEY,
    name       TEXT NOT NULL,
    token      TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX devices_token_idx ON devices (token);
```

Token: 20 random bytes → base64url (27 chars, no padding). Stored plaintext (it's a bearer credential, not a password). Users cannot pick tokens; server always generates.

## API

### Device CRUD (unchanged pattern from flows)

| Method | Path                 | Purpose                                       |
|--------|----------------------|-----------------------------------------------|
| GET    | `/api/devices`       | List                                          |
| POST   | `/api/devices`       | Create. Body: `{name}`. Response includes `token`. |
| GET    | `/api/devices/:id`   | Get                                           |
| PUT    | `/api/devices/:id`   | Update (name only; token is immutable)        |
| DELETE | `/api/devices/:id`   | Delete. Also removes any `device_source` references in flows? NO — just delete; flows keep stale selectors that silently match nothing. UI can warn. |
| POST   | `/api/devices/:id/regenerate-token` | Rotate token. |

### EMQX webhooks (called BY EMQX, not the UI)

**`POST /api/mqtt/auth`** — called on every CONNECT.

Request (EMQX body, we configure the shape via EMQX's `http` auth backend):
```json
{ "username": "U123ABC...", "password": "", "clientid": "..." }
```

Response:
```json
{ "result": "allow" }   // or "deny"
```

Logic:
1. Look up device where `token == username`.
2. If found: allow. (Observer itself connects with a service-account username `observer-consumer` hardcoded in config; accept that too.)
3. If not: deny.

**`POST /api/mqtt/acl`** — called on PUBLISH/SUBSCRIBE.

Request:
```json
{ "username": "U123...", "action": "publish", "topic": "devices/d52a8f7e-.../telemetry" }
```

Response: `{"result": "allow"}` if:
- `action == "publish"` AND `topic == "v1/devices/{uuid}/telemetry"` AND `{uuid}` matches the device whose token equals `username`.
- `action == "subscribe"` AND username is the Observer service account.

Everything else: `deny`.

The ACL handler keeps a 30-second in-memory cache of `token → device UUID` so this hot path does not hit the DB on every publish.

Both endpoints are mounted under `/api/mqtt/*` but do NOT require the JSON payload wrapping the UI uses. They're shaped for EMQX. A shared-secret header (`X-EMQX-Secret`) env-configurable protects them from random callers on the same network.

## MQTT Topic + Payload Contract

**Topic (device publishes):** `devices/{device-uuid}/telemetry`

The device knows its own UUID (returned when it was created via the REST API, alongside the token). The token authenticates the CONNECT; the UUID in the topic identifies which device the message is for. The ACL layer enforces that a device may only publish to the topic matching its own UUID.

**Payload:** JSON object, keys are metric names, values are numbers. Optional `ts` top-level ISO-8601 timestamp; if absent, server uses receive time.

```json
{ "temperature": 23.5, "humidity": 60.0, "ts": "2026-04-12T10:00:00Z" }
```

One message → N readings (one per metric key, excluding `ts`). Non-numeric values are silently dropped (a `parse_errors_total` counter increments).

## Flow Node: `device_source`

Replaces `mqtt_source`. Config JSON:

```json
{ "device_id": "uuid-or-empty", "metric": "temperature-or-empty" }
```

Semantics:
- `device_id` empty or absent → match all devices.
- `metric` empty or absent → emit every metric.
- Both set → emit only the specific device × metric combo.

Validation (`graph.Validate`):
- If `device_id` is a non-empty string, it must parse as a UUID. (We do NOT check existence at validate time — that's a runtime concern; deleting a device is allowed.)
- `metric` is a free-form string or empty.

The node's `Run` registers `(flowID, deviceID|*, metric|*)` with the router's subscription registry on start, and deregisters on stop. The router pushes matching readings into the node's output channel.

## Ingest Pipeline

```
EMQX ─► consumer ─► parser ─► router ─► flow input channels
                 (raw msg)   (readings)    (fan-out)
```

1. `consumer.Run(ctx)`:
   - Connects to `EMQX_BROKER_URL` with service-account credentials.
   - Subscribes to `$share/observer/v1/devices/+/telemetry`.
   - On each message: sends to parser input channel.
2. `parser`:
   - Pure function. Extracts the UUID from the topic (`v1/devices/{uuid}/telemetry`), parses JSON, yields `[]SensorReading{DeviceID, Metric, Value, Timestamp}`.
   - No DB lookup in the hot path — the UUID is directly in the topic, trust the ACL layer's earlier check.
   - Malformed topics or payloads are dropped; a parse-error is logged at debug.
   - Parser output goes to router.
3. `router`:
   - Holds `subscriptions map[flowID][]Subscription` and `reverse index map[(deviceID,metric)][]*Subscription`.
   - On reading: looks up matching subscriptions, pushes to each one's output channel (non-blocking, drop-on-full with a counter).
   - `Subscribe(flowID, deviceSel, metricSel) → chan Reading`.
   - `Unsubscribe(flowID)` — wipes all subs for that flow.

## Configuration

New env vars:

| Variable            | Default                              | Purpose                           |
|---------------------|--------------------------------------|-----------------------------------|
| `EMQX_BROKER_URL`   | `tcp://emqx:1883`                    | Where Observer connects           |
| `EMQX_USERNAME`     | `observer-consumer`                  | Service account on EMQX side      |
| `EMQX_PASSWORD`     | (required, no default)               | Service account password          |
| `EMQX_SHARED_GROUP` | `observer`                           | Shared-sub group name             |
| `MQTT_WEBHOOK_SECRET` | (required)                         | Shared secret EMQX sends in `X-EMQX-Secret` |

Existing `MQTT_BROKER` / `MQTT_TOPIC` env vars are removed.

## Error Handling

- **Device token invalid on CONNECT** → `/api/mqtt/auth` returns `deny`, counter increments, EMQX closes socket. No server-side log spam (log at debug).
- **Device publishes to wrong topic** → `/api/mqtt/acl` returns `deny`.
- **Message parse failure** → logged at debug, message dropped. Failure modes: bad JSON, no numeric values, topic that does not match `v1/devices/{uuid}/telemetry`.
- **Router output channel full** → reading dropped (do not block the ingest path). Logged at debug.
- **EMQX consumer disconnect** → paho auto-reconnects.
- **Observer HTTP handler down** → EMQX default `super-user`/`deny` semantics depend on configuration. We configure EMQX to **deny by default on webhook failure** so a dead Observer means no new device connections (safer than allowing).
- **Database down** → `/api/mqtt/auth` returns 500; EMQX denies. Devices queue locally and reconnect when we recover.

## Migration Strategy (feature branch only)

- Old `mqtt_source` node type is removed from `internal/flow/nodes/` and from `graph.KnownTypes`. Any persisted flows with `mqtt_source` nodes will fail validation on load — acceptable pre-prod. Add a one-line warning in the migration runner: flows with unknown node types are left in place but never started.
- `internal/subscriber/` is deleted (no longer needed — the ingest path does parsing).
- Fresh `migrations/003_create_devices.sql` added.
- `cmd/observer/main.go` wires up: db migrate → repo → ingest consumer → router → flow manager → http.

## Testing

Power-plant bar applies. Every function in `internal/ingest/`, `internal/devices/`, and the new API handlers gets unit coverage.

**Unit (no tag):**
- `parser.Parse` — table-driven: happy path, missing `ts`, non-numeric values dropped, multiple metrics, empty object, malformed JSON, missing token in topic.
- `router.Subscribe`/`Dispatch`/`Unsubscribe` — table-driven: match-all, match-device, match-device+metric, drop-on-full, unsubscribe wipes state.
- `devices.Service.Create` — generates unique token, populates row.
- `devices.Service.RegenerateToken` — new token differs, old token cache invalidated.
- `mqtt_auth_handler` — allow valid token, deny unknown, allow service account, reject missing secret.
- `acl_handler` — allow publish to `me/telemetry`, deny wrong topic, allow service-account subscribe.
**Integration (`//go:build integration`):**
- `devices/store/repo_test.go` — CRUD against ephemeral Timescale container.
- `ingest/integration_test.go` — spin up EMQX container + Postgres, register a device via API, publish from a paho client using the token, assert the reading flows out of the router into a subscribed flow.
- End-to-end `test/ingest_e2e_test.go` — full happy path through the API server + EMQX + WebSocket events, mirroring the existing `test/flow_e2e_test.go` pattern.

## Dev / Run Workflow

`docker-compose.yml` gains:

```yaml
emqx:
  image: emqx/emqx:5.7
  ports: ["1883:1883", "8083:8083", "18083:18083"]
  environment:
    EMQX_NAME: observer-emqx
    EMQX_ALLOW_ANONYMOUS: "false"
    EMQX_AUTH__HTTP__AUTH_REQ__URL: "http://app:8080/api/mqtt/auth"
    EMQX_AUTH__HTTP__ACL_REQ__URL: "http://app:8080/api/mqtt/acl"
    EMQX_AUTH__HTTP__AUTH_REQ__PARAMS: "username=%u,password=%P,clientid=%c"
    EMQX_AUTH__HTTP__ACL_REQ__PARAMS: "username=%u,action=%A,topic=%t"
    EMQX_LOG__CONSOLE_HANDLER__LEVEL: "warning"
```

(Exact env keys depend on EMQX 5.x — the plan will pin to the verified names.)

`docker compose up` brings the stack:
- emqx :1883 (mqtt), :8083 (mqtt-over-ws), :18083 (admin dashboard)
- observer app :8080
- postgres :5432

Local dev (no docker for the app): same as before, just `docker compose up emqx postgres -d` first.

## Risks

- **EMQX 5.x HTTP auth config is verbose.** Mitigation: verify exact env keys + working config in Task 1 of the plan before touching any Go code.
- **Shared subscriptions with a single replica** are just regular subscriptions with extra ceremony. That's fine — when we scale replicas later, no code change is needed. We pay zero cost today for tomorrow's optionality.
- **Token-in-username pattern** is a widespread MQTT convention (ThingsBoard, EMQX docs) but some client libraries trim credentials weirdly. Dev note: test with paho and mosquitto_pub both.
- **Cache invalidation on device delete** — if the 30s TTL feels too coarse for deletion, we can punch through the cache on delete; keep an eye on this during soak.
- **Deleted device's token stays valid for up to 30s** in the auth cache. Acceptable (an attacker can't steal a deleted token from nowhere; a regenerated token is the path for rotation).

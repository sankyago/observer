# Observer — Telemetry Anomaly Platform — Design

**Date:** 2026-04-14
**Status:** Design (pre-implementation)

## 1. Goal

Build a multi-tenant telemetry platform where customers point devices at `mqtt.observer.io`, we persist their data to TimescaleDB, evaluate threshold rules in real time, and trigger downstream actions (webhooks, email, etc.) when rules fire. Inspired by ThingsBoard, but intentionally smaller in scope.

## 2. Constraints (locked)

| Concern | Decision |
|---|---|
| Message profile | Small JSON telemetry (~200–500 B), e.g. `{"temperature": 50}` |
| Delivery semantics (telemetry) | At-most-once (QoS 0). Drops acceptable on overload. |
| Rule eval latency | Real-time, < 1 ms hot path |
| Rule complexity (v1) | Stateless threshold rules (`field op value`) |
| Action execution (v1) | `print('alert')` placeholder; webhook/email/etc. added later |
| Throughput target | Up to 100k msg/s sustained |
| Fleet shape | Flexible: 100k–1M devices, mixed publish rates |

## 3. Services

Four logical components. **One Go monorepo, one binary, multiple subcommands.** Deployed as a monolith for MVP, splittable into independent processes when scale demands.

| Name | Role |
|---|---|
| **`transport`** | MQTT consumer. Parses messages, evaluates rules in-process, batch-writes telemetry to Timescale, enqueues action jobs. |
| **`runner`** | Job consumer. Executes triggered actions (webhook, email, etc.), retries, dead-letters. |
| **`api`** | HTTP service. Tenant/device/rule/action CRUD; reads telemetry for dashboards; reads job status for admin views. |
| **`ui`** | React SPA. Separate deployment (static assets on CDN). Talks only to `api` over HTTPS/JSON. |

## 4. Deployment Modes

Single codebase, two ways to run it:

### Mode A — Monolith (MVP / small customers)
- One container running `observerctl all`.
- `transport`, `runner`, `api` live in the same Go process.
- In-memory `chan Job` between `transport` and `runner`.
- One Postgres (Timescale + metadata + river job tables), one EMQX node.
- Fits comfortably on one modest VM up to ~5–10k msg/s.

### Mode B — Split services (scale)
- Three separate Kubernetes deployments: `transport`, `runner`, `api`.
- `transport` → `runner` over `river` (Postgres-backed job queue).
- EMQX cluster (3+ nodes), Timescale primary + read replica.
- Independent scaling, deploys, failure blast radius.

The swap is driven by a config flag that selects an implementation of the `Queue` interface (`inmem` vs `river`). Business logic is identical.

## 5. Stack

| Layer | Choice | Rationale |
|---|---|---|
| **MQTT broker** | EMQX 5 (open source, clustered in Mode B) | Native MQTT 5 shared subscriptions; battle-tested at scale; Prometheus metrics; auth/ACL against Postgres. |
| **Service language** | Go | Cheap goroutines, predictable GC, single static binary. |
| **MQTT client** | `eclipse/paho.mqtt.golang` | Stable, default. |
| **Hot storage (telemetry)** | TimescaleDB 2.x on Postgres 16 | Hypertables, compression, continuous aggregates. |
| **Cold storage (metadata)** | Same Postgres instance, regular tables | Don't split until forced to. |
| **Job queue** | `riverqueue/river` (Postgres-backed) | Zero new infra; transactional enqueue; built-in retries/DLQ/UI. |
| **Cache invalidation** | Postgres `LISTEN/NOTIFY` | Workers `LISTEN rules_changed`, refresh in-memory rule cache. |
| **HTTP framework** | `chi` | Minimal, idiomatic. |
| **UI** | React 19 + Vite + TypeScript + Ant Design 5 + React Flow | React Flow for action-builder canvas. Static deploy. |
| **Metrics** | Prometheus + Grafana | EMQX and Go services export natively. |
| **Logs** | stdout → Vector → Loki | Standard. |
| **Container orchestration** | Kubernetes (managed) | HPA on broker pending depth drives autoscaling in Mode B. |
| **Autoscaling signal** | `emqx_subscriptions_shared_messages_pending` via Prometheus Adapter | Direct measurement of "are we behind", not CPU. |

### Explicitly skipped at v1
Kafka, Redis, separate rule-engine service, gRPC between internal services, multi-region, custom MQTT broker.

## 6. Data Flow

```
Device ──(MQTT pub QoS 0)──▶ EMQX ──$share/transport/...──▶ transport
                                                               │
                                                               ├─▶ Timescale (telemetry, batched COPY)
                                                               │
                                                               └─▶ Queue (river or inmem)
                                                                              │
                                                                              ▼
                                                                         runner ──▶ external action
                                                                                    (webhook, email, log)

User ──(HTTPS)──▶ ui (React SPA, CDN) ──(JSON)──▶ api ──▶ Postgres (CRUD)
                                                       └─▶ Timescale (chart reads)
                                                       └─▶ river (job status reads)
```

### `transport` hot path (per message, target < 1 ms)
1. Parse MQTT topic → `(tenant_id, device_id)`.
2. Parse JSON payload.
3. Stamp `message_id` (UUID v7) + `received_at` (server timestamp).
4. Lookup rule(s) for `device_id` in in-memory rule cache (`map[deviceID][]Rule`).
5. Evaluate threshold rules against payload fields. If any match → enqueue action job (non-blocking).
6. Append row to in-process Timescale write buffer (flushed every 1k rows or 100 ms via `COPY`).

### Rule cache lifecycle
- On startup, `transport` loads all rules from Postgres into memory.
- `transport` `LISTEN`s on Postgres channel `rules_changed`.
- When `api` writes a rule change: `INSERT/UPDATE/DELETE rules; NOTIFY rules_changed, '<device_id>';`
- `transport` reloads only the affected device's rules.
- Fallback: full reload every 5 min as belt-and-braces.

### Per-rule debug flag (stolen from ThingsBoard)
Each rule row has a `debug_until TIMESTAMPTZ NULL` column. While `now() < debug_until`, every message touching the rule (matched or not) is logged with full payload + eval result. Costs nothing when off; invaluable for "why didn't my rule fire?" support tickets.

## 7. Topic Conventions

```
tenants/<tenant_slug>/devices/<device_id>/telemetry
tenants/<tenant_slug>/devices/<device_id>/attributes
tenants/<tenant_slug>/devices/<device_id>/rpc/request
tenants/<tenant_slug>/devices/<device_id>/rpc/response
```

`transport` shared subscription: `$share/transport/tenants/+/devices/+/telemetry`

## 8. Auth

| Surface | Mechanism |
|---|---|
| Devices (MQTT) | EMQX built-in auth → Postgres `device_credentials` table. ACL restricts each device to its own topic prefix. |
| End users (UI/API) | Postgres + argon2 + JWT. Ory Kratos only if SSO/SAML appears. |
| Service-to-service (Mode B) | Cluster-internal network only; no auth between services in v1. |

## 9. Data Model (sketch)

```sql
-- Metadata (regular Postgres tables)
tenants            (id, slug, name, created_at, ...)
users              (id, tenant_id, email, password_hash, role, ...)
devices            (id, tenant_id, name, type, created_at, ...)
device_credentials (device_id, username, password_hash, ...)
rules              (id, tenant_id, device_id, field, op, value,
                    action_id, enabled, debug_until, created_at, ...)
actions            (id, tenant_id, kind, config_json, created_at, ...)
                   -- kind ∈ {webhook, email, log}; config_json shape per kind

-- Telemetry (Timescale hypertable, partitioned by time + device_id hash)
telemetry          (time TIMESTAMPTZ, tenant_id, device_id, message_id,
                    payload JSONB)
                   -- compression policy: compress chunks older than 7 days
                   -- retention policy: drop chunks older than 1 year (configurable per tenant)

-- Jobs (river-managed)
river_job          (managed by river)
```

## 10. Observability

Four-chart "am I healthy?" dashboard:

1. **Broker pending depth** — `emqx_subscriptions_shared_messages_pending{group="transport"}` (scale-out trigger when > 0 sustained for 30 s).
2. **`transport` p99 message processing time** — confirms worker-side bottleneck.
3. **Timescale p99 batch insert duration** — confirms DB-side bottleneck.
4. **Dropped messages/sec** — `transport_messages_dropped_total{reason=...}` (`buffer_full` / `parse_error` / `db_error`).

Alerting via Grafana Alerting on the four signals above. Diagnostic lookup table:

| Signal pattern | Bottleneck | Action |
|---|---|---|
| Broker pending > 0, transport CPU high | transport | Add transport replicas |
| Broker pending > 0, transport CPU low, flush duration climbing | Timescale | Tune Timescale, add write replica, larger batches |
| Broker connection count near node limit | Broker | Add EMQX node |
| `transport` memory climbing | rule cache or batch buffer leak | Investigate, don't autoscale |

## 10a. Batching & Pre-Aggregation in `transport`

At 100k msg/s, inserting every raw message individually is wasteful. `transport` runs **two parallel buffers** per flush cycle:

### Raw buffer (short retention)
- Append every parsed message.
- Flush every **1k rows or 100 ms** (whichever first) via single `COPY` into `telemetry_raw`.
- Retention: **7 days** (configurable). Compressed after 1 day.
- Purpose: debugging, recent inspection, replaying recent windows for new rules.

### Aggregation buffer (long retention)
- Keyed by `(tenant_id, device_id, field_name, bucket_start)`, where `bucket_start` is the message timestamp truncated to a fixed window (default **10 s**).
- For every numeric field in the payload, accumulate:
  - `count`
  - `sum`
  - `min`
  - `max`
  - `last_value` + `last_at`
- When a bucket's window closes, flush its row to `telemetry_aggregates` via `COPY`.
- Retention: **1 year** (configurable per tenant). Compressed after 7 days.

### Aggregate table shape

```sql
telemetry_aggregates (
  bucket_start TIMESTAMPTZ NOT NULL,
  tenant_id    UUID NOT NULL,
  device_id    UUID NOT NULL,
  field_name   TEXT NOT NULL,
  count        BIGINT NOT NULL,
  sum          DOUBLE PRECISION NOT NULL,
  min          DOUBLE PRECISION NOT NULL,
  max          DOUBLE PRECISION NOT NULL,
  last_value   DOUBLE PRECISION NOT NULL,
  last_at      TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (tenant_id, device_id, field_name, bucket_start)
);
SELECT create_hypertable('telemetry_aggregates', 'bucket_start');
```

This collapses 100k msg/s into roughly `device_count × numeric_fields / 10s` aggregate rows — orders of magnitude smaller than raw, while preserving everything needed to render charts and run historical queries.

### Why pre-aggregate in `transport` instead of using Timescale continuous aggregates?

Both have a place. We do **both**:

- **Pre-aggregation in `transport`** — bounded write rate to Timescale, predictable hot path, controls cost. This is the load-bearing one.
- **Timescale continuous aggregates** on top of `telemetry_aggregates` for hourly/daily roll-ups used by long-range dashboards. Cheap because they aggregate already-aggregated rows.

### Rule evaluation is unaffected

Rule eval still runs on **every raw message** in `transport` (real-time, < 1 ms). The buffers are write-side only — they do not gate or delay the rule path.

### Memory bound

The aggregation buffer is bounded by `active_devices × numeric_fields × concurrent_buckets`. At 1M devices × 5 fields × 2 open buckets, that's ~10M small structs (~1–2 GB per `transport` replica). If memory becomes a concern, shard buckets across replicas via consistent hashing on `device_id`. Out of scope for v1; flagged in §17.

## 11. Message Identity

Every ingested message gets:

- `message_id` — UUID v7 (time-ordered) stamped at `transport` ingress.
- `received_at` — server timestamp at `transport` ingress.
- `correlation_id` — same as `message_id` by default; propagated into Timescale row and into any enqueued action job.

This makes "show me the message that triggered alert X" a one-line SQL join.

## 12. Action Execution (`runner`)

- Consumes jobs from the queue (river in Mode B, in-mem channel in Mode A).
- v1 action kinds: `log` (just prints), `webhook` (HTTP POST with retry), `email` (SMTP).
- Retry policy: exponential backoff, max 5 attempts, then dead-letter.
- Action config is read from the `actions` table by ID; payload (device + rule + telemetry snapshot) comes from the job.
- Scales independently of `transport`: HPA on river job queue depth.

## 13. Tenant Isolation

- `tenant_id` carried in every MQTT topic, every Timescale row, every job payload, every rule cache entry.
- EMQX ACL prevents a device from publishing or subscribing outside its tenant prefix.
- API enforces tenant scope on every query via middleware extracting `tenant_id` from JWT.
- No cross-tenant data path exists.

## 14. UI Scope (v1)

- Tenant dashboard: device list, device detail, latest telemetry, recent alerts.
- Rule editor: create/edit/delete threshold rules per device.
- Action builder (React Flow canvas): linear flow `[Trigger: rule fires] → [Action: log | webhook | email]`. One trigger, one action chain. No branching in v1.
- Auth: login, password reset, basic user management within a tenant.

Out of scope for v1: device provisioning wizards, billing UI, multi-region selectors, RBAC beyond admin/member.

## 15. What We Borrowed from ThingsBoard (and What We Didn't)

| Pattern | Adopted? | Why |
|---|---|---|
| Monolith-first, splittable codebase | ✅ | Their actual default; matches our YAGNI stance. |
| Per-message tenant ID + correlation ID (`TbMsg`) | ✅ | Tracing alerts back to source messages is invaluable. |
| Per-rule debug flag | ✅ | Support tickets become trivial. |
| Custom MQTT server (Netty) | ❌ | EMQX is the production-ready version of this idea. |
| Per-device actor framework | ❌ | Our rules are stateless; revisit only if windowed rules appear. |
| Multi-broker abstraction (Kafka/RabbitMQ/Pulsar/in-mem) | ❌ | Pick `river` and stick. |
| Rule chains (DAGs of nodes) | ❌ for v1 | Flat threshold rules cover the use case. |
| Synchronous webhook/email in rule engine thread | ❌ explicitly avoided | This is *the* bottleneck we designed around — `runner` exists for this reason. |

## 16. Out of Scope (v1)

- Stateful / windowed rules (`avg over 5 min`).
- Cross-device correlated rules.
- User-defined JavaScript actions / sandbox.
- Edge devices / offline buffering / sync.
- Billing, usage metering.
- SSO/SAML.
- Multi-region.
- Per-tenant queue isolation.
- Versioning of rule chains (ThingsBoard's vc-executor).

Each of these has a clear extension point in the design above and can be added without restructuring.

## 17. Testing

**Unit tests are the only required test layer for v1.** No e2e — too slow and expensive at this stage.

### Coverage targets
- **Pure logic** (rule evaluation, payload parsing, topic parsing, aggregation buffer math, batch flushing decisions): **near-100% coverage**, table-driven Go tests.
- **DB layer**: integration tests against a real Postgres (Timescale) via `testcontainers-go`. Test queries, migrations, hypertable behavior. These run in CI but are still classed as unit-level (single process, single DB).
- **HTTP handlers (`api`)**: handler-level tests with `httptest`, no real network.
- **Queue (`river`) handlers**: tested with the `river` test harness — enqueue a job, run handler in-process, assert side effects.
- **MQTT consumer (`transport`)**: tested by injecting fake messages directly into the parse → eval → buffer pipeline, bypassing the network. The Paho client is treated as a boundary; we don't unit-test it.

### What we explicitly skip
- End-to-end tests that spin up EMQX + Postgres + UI together.
- Browser-driven UI tests (Cypress/Playwright).
- Load tests in CI (run separately, on demand, against staging).

### CI gate
- `go test ./...` must pass.
- Coverage report published, no hard threshold (resist Goodhart's law).
- Frontend: `vitest` for component logic, `tsc --noEmit` for types. No browser tests.

## 18. Resolved Decisions

1. **Aggregation field discovery: automatic.** Every numeric field in the payload is aggregated. Non-numeric fields are stored in raw only. Cardinality protection: per-device cap of 64 distinct field names (drop + log new fields beyond the cap).
2. **Raw retention: 7 days default**, configurable per tenant.
3. **Aggregation buffer sharding** deferred to post-v1 (revisit when a single `transport` replica's memory becomes the bottleneck).

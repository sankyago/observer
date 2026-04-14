# Observer — Sub-project 2: Backend Ingest — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development.

**Goal:** End-to-end backend data path. A device publishes telemetry to EMQX; `transport` consumes it, writes raw rows, evaluates rules, enqueues action jobs; `runner` executes actions and records them. Verifiable entirely from the command line (`mosquitto_pub` in, `psql` to inspect `telemetry_raw` + `fired_actions`).

**Architecture:** `transport` connects to EMQX as a shared-subscription MQTT client, listens on `$share/transport/tenants/+/devices/+/telemetry`, parses messages, writes to `telemetry_raw`, evaluates rules (stateless thresholds) against an in-memory cache kept fresh via Postgres `LISTEN rules_changed`, and enqueues `queue.Job` for matches. `runner` consumes from the in-memory queue and executes the action (log or webhook), recording each execution into a new `fired_actions` table.

**Tech Stack:** `eclipse/paho.mqtt.golang`, pgx/v5, `google/uuid`. No new infrastructure — reuses EMQX and Postgres from Plan 01.

**Reference spec:** `docs/superpowers/specs/2026-04-14-telemetry-anomaly-platform-design.md` (§6 data flow, §10 batching, §12 action execution, §11 message identity).

---

## Conventions

- Topic scheme for PoC: `tenants/<tenant_slug>/devices/<device_uuid>/telemetry`.
- EMQX anonymous access is enabled for PoC (real auth arrives with the API plan).
- Payload: JSON object with string/number fields, e.g. `{"temperature": 72}`.
- For the PoC, telemetry inserts are **one row per message** (no batching). Batching + aggregation come in a later plan.
- Fields in payloads that aren't valid JSON numbers are simply stored in `payload`; only numeric fields are eligible for rule evaluation.

---

## File Structure

```
D:/Work/Observer/
├── deploy/docker-compose.yml            # modified: add EMQX allow-anonymous
├── migrations/
│   └── 00005_fired_actions.sql          # new: fired_actions log table
├── pkg/
│   ├── topic/topic.go                   # new: parse tenants/<slug>/devices/<uuid>/telemetry
│   ├── topic/topic_test.go
│   ├── mqtt/mqtt.go                     # new: shared-sub consumer, MessageHandler signature
│   ├── mqtt/mqtt_test.go                # uses testcontainers-emqx or local stack
│   ├── rules/cache.go                   # new: in-memory rule cache keyed by device_id
│   ├── rules/cache_test.go
│   ├── rules/eval.go                    # new: threshold eval
│   ├── rules/eval_test.go
│   ├── rules/loader.go                  # new: load from DB + LISTEN/NOTIFY reload
│   ├── store/telemetry.go               # new: InsertRaw
│   ├── store/fired.go                   # new: InsertFired
│   ├── actions/action.go                # new: Action interface, registry
│   ├── actions/log.go                   # new: log action
│   └── actions/webhook.go               # new: webhook action
├── cmd/transport/main.go                # modified: wire full pipeline
├── cmd/runner/main.go                   # modified: wire queue consume + actions
```

---

## Task 1: Enable EMQX anonymous auth and add `mosquitto-clients` to the dev workflow

**Files:**
- Modify: `D:/Work/Observer/deploy/docker-compose.yml`

- [ ] **Step 1: Add anonymous access + retain default dashboard password**

Edit `deploy/docker-compose.yml`, replace the `emqx` service environment block with:

```yaml
    environment:
      EMQX_DASHBOARD__DEFAULT_PASSWORD: observer
      EMQX_ALLOW_ANONYMOUS: "true"
      EMQX_LISTENERS__TCP__DEFAULT__ENABLE_AUTHN: "false"
```

- [ ] **Step 2: Recreate emqx**

```bash
cd D:/Work/Observer
docker compose -f deploy/docker-compose.yml up -d --force-recreate emqx
```

Wait ~15s, confirm healthy:

```bash
docker compose -f deploy/docker-compose.yml ps emqx
```

- [ ] **Step 3: Verify anonymous publish works**

```bash
docker run --rm --network host eclipse-mosquitto:2 mosquitto_pub \
  -h localhost -p 1883 -t test/smoke -m "hello"
```

Expected: command exits 0 with no error.

- [ ] **Step 4: Commit**

```bash
cd D:/Work/Observer && git add deploy/docker-compose.yml && git commit -m "chore(deploy): enable EMQX anonymous access for PoC"
```

---

## Task 2: `pkg/topic` — parse MQTT topic into (tenant_slug, device_id)

**Files:**
- Create: `pkg/topic/topic.go`, `pkg/topic/topic_test.go`

- [ ] **Step 1: Write the failing test**

Create `D:/Work/Observer/pkg/topic/topic_test.go`:

```go
package topic

import (
	"testing"

	"github.com/google/uuid"
)

func TestParseTelemetry_OK(t *testing.T) {
	did := uuid.New()
	topic := "tenants/acme/devices/" + did.String() + "/telemetry"
	parsed, err := ParseTelemetry(topic)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if parsed.TenantSlug != "acme" {
		t.Errorf("tenant slug: got %q", parsed.TenantSlug)
	}
	if parsed.DeviceID != did {
		t.Errorf("device id: got %s want %s", parsed.DeviceID, did)
	}
}

func TestParseTelemetry_BadShape(t *testing.T) {
	cases := []string{
		"tenants/acme/devices",
		"tenants/acme/devices//telemetry",
		"tenants//devices/" + uuid.New().String() + "/telemetry",
		"wrong/acme/devices/" + uuid.New().String() + "/telemetry",
		"tenants/acme/devices/not-a-uuid/telemetry",
		"tenants/acme/devices/" + uuid.New().String() + "/attributes",
	}
	for _, tc := range cases {
		if _, err := ParseTelemetry(tc); err == nil {
			t.Errorf("expected error for %q", tc)
		}
	}
}
```

- [ ] **Step 2: Run to verify failure**

```bash
cd D:/Work/Observer && go test ./pkg/topic/...
```

Expected: build failure `undefined: ParseTelemetry`.

- [ ] **Step 3: Implement**

Create `D:/Work/Observer/pkg/topic/topic.go`:

```go
// Package topic parses Observer MQTT topic names.
package topic

import (
	"errors"
	"strings"

	"github.com/google/uuid"
)

type Telemetry struct {
	TenantSlug string
	DeviceID   uuid.UUID
}

var errBadTopic = errors.New("invalid telemetry topic")

// ParseTelemetry expects: tenants/<slug>/devices/<uuid>/telemetry
func ParseTelemetry(t string) (Telemetry, error) {
	parts := strings.Split(t, "/")
	if len(parts) != 5 {
		return Telemetry{}, errBadTopic
	}
	if parts[0] != "tenants" || parts[2] != "devices" || parts[4] != "telemetry" {
		return Telemetry{}, errBadTopic
	}
	if parts[1] == "" {
		return Telemetry{}, errBadTopic
	}
	id, err := uuid.Parse(parts[3])
	if err != nil {
		return Telemetry{}, errBadTopic
	}
	return Telemetry{TenantSlug: parts[1], DeviceID: id}, nil
}
```

- [ ] **Step 4: Run to verify pass**

```bash
cd D:/Work/Observer && go test ./pkg/topic/...
```

- [ ] **Step 5: Commit**

```bash
cd D:/Work/Observer && git add pkg/topic/ && git commit -m "feat(topic): parse tenants/<slug>/devices/<uuid>/telemetry"
```

---

## Task 3: `migrations/00005_fired_actions.sql`

**Files:**
- Create: `migrations/00005_fired_actions.sql`

- [ ] **Step 1: Write the migration**

Create `D:/Work/Observer/migrations/00005_fired_actions.sql`:

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE fired_actions (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    fired_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    tenant_id      UUID NOT NULL,
    device_id      UUID NOT NULL,
    rule_id        UUID NOT NULL,
    action_id      UUID NOT NULL,
    message_id     UUID NOT NULL,
    status         TEXT NOT NULL CHECK (status IN ('ok','error')),
    error          TEXT,
    payload        JSONB NOT NULL
);
CREATE INDEX fired_actions_tenant_time_idx ON fired_actions (tenant_id, fired_at DESC);
CREATE INDEX fired_actions_device_time_idx ON fired_actions (device_id, fired_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS fired_actions;
-- +goose StatementEnd
```

- [ ] **Step 2: Apply and verify**

```bash
cd D:/Work/Observer
OBSERVER_DB_DSN="postgres://observer:observer@localhost:5432/observer?sslmode=disable" \
OBSERVER_MQTT_URL="tcp://localhost:1883" \
go run ./cmd/migrate up

docker exec -i observer-postgres psql -U observer -d observer -c "\d fired_actions"
```

- [ ] **Step 3: Commit**

```bash
cd D:/Work/Observer && git add migrations/00005_fired_actions.sql && git commit -m "feat(migrate): fired_actions log table"
```

---

## Task 4: `pkg/store` — InsertRaw + InsertFired

**Files:**
- Create: `pkg/store/telemetry.go`, `pkg/store/fired.go`, `pkg/store/store_test.go`

- [ ] **Step 1: Write the failing test**

Create `D:/Work/Observer/pkg/store/store_test.go`:

```go
package store

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/observer-io/observer/internal/testutil"
)

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := testutil.StartTimescale(t)
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	applyMigrations(t, pool)
	return pool
}

func applyMigrations(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	// Minimal inline schema subset for store tests — the full migrations run in CI via goose
	// but for this isolated test we only need telemetry_raw + fired_actions.
	stmts := []string{
		`CREATE EXTENSION IF NOT EXISTS timescaledb`,
		`CREATE EXTENSION IF NOT EXISTS pgcrypto`,
		`CREATE TABLE telemetry_raw (
			time TIMESTAMPTZ NOT NULL,
			tenant_id UUID NOT NULL,
			device_id UUID NOT NULL,
			message_id UUID NOT NULL,
			payload JSONB NOT NULL
		)`,
		`SELECT create_hypertable('telemetry_raw','time')`,
		`CREATE TABLE fired_actions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			fired_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			tenant_id UUID NOT NULL,
			device_id UUID NOT NULL,
			rule_id UUID NOT NULL,
			action_id UUID NOT NULL,
			message_id UUID NOT NULL,
			status TEXT NOT NULL CHECK (status IN ('ok','error')),
			error TEXT,
			payload JSONB NOT NULL
		)`,
	}
	for _, s := range stmts {
		if _, err := pool.Exec(ctx, s); err != nil {
			t.Fatalf("schema: %v: %s", err, s)
		}
	}
}

func TestInsertRawAndFired(t *testing.T) {
	if testing.Short() {
		t.Skip("container test")
	}
	pool := newPool(t)
	ctx := context.Background()

	tenantID := uuid.New()
	deviceID := uuid.New()
	messageID := uuid.New()
	payload := json.RawMessage(`{"temperature":72}`)

	if err := InsertRaw(ctx, pool, RawRow{
		Time: time.Now().UTC(), TenantID: tenantID, DeviceID: deviceID,
		MessageID: messageID, Payload: payload,
	}); err != nil {
		t.Fatalf("InsertRaw: %v", err)
	}

	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM telemetry_raw WHERE message_id=$1`, messageID).Scan(&n); err != nil {
		t.Fatalf("query: %v", err)
	}
	if n != 1 {
		t.Fatalf("got %d rows, want 1", n)
	}

	ruleID := uuid.New()
	actionID := uuid.New()
	if err := InsertFired(ctx, pool, FiredRow{
		TenantID: tenantID, DeviceID: deviceID, RuleID: ruleID, ActionID: actionID,
		MessageID: messageID, Status: "ok", Payload: payload,
	}); err != nil {
		t.Fatalf("InsertFired: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM fired_actions WHERE message_id=$1`, messageID).Scan(&n); err != nil {
		t.Fatalf("query fired: %v", err)
	}
	if n != 1 {
		t.Fatalf("fired rows got %d want 1", n)
	}
}
```

- [ ] **Step 2: Run to verify failure**

```bash
cd D:/Work/Observer && go test ./pkg/store/...
```

Expected: build failure.

- [ ] **Step 3: Implement `pkg/store/telemetry.go`**

Create `D:/Work/Observer/pkg/store/telemetry.go`:

```go
// Package store provides database insert helpers for telemetry and fired actions.
package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RawRow struct {
	Time      time.Time
	TenantID  uuid.UUID
	DeviceID  uuid.UUID
	MessageID uuid.UUID
	Payload   []byte // JSON bytes
}

func InsertRaw(ctx context.Context, pool *pgxpool.Pool, r RawRow) error {
	_, err := pool.Exec(ctx,
		`INSERT INTO telemetry_raw (time, tenant_id, device_id, message_id, payload)
		 VALUES ($1, $2, $3, $4, $5)`,
		r.Time, r.TenantID, r.DeviceID, r.MessageID, r.Payload,
	)
	return err
}
```

- [ ] **Step 4: Implement `pkg/store/fired.go`**

Create `D:/Work/Observer/pkg/store/fired.go`:

```go
package store

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type FiredRow struct {
	TenantID  uuid.UUID
	DeviceID  uuid.UUID
	RuleID    uuid.UUID
	ActionID  uuid.UUID
	MessageID uuid.UUID
	Status    string // "ok" or "error"
	Error     string
	Payload   []byte
}

func InsertFired(ctx context.Context, pool *pgxpool.Pool, f FiredRow) error {
	var errText *string
	if f.Error != "" {
		errText = &f.Error
	}
	_, err := pool.Exec(ctx,
		`INSERT INTO fired_actions (tenant_id, device_id, rule_id, action_id, message_id, status, error, payload)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		f.TenantID, f.DeviceID, f.RuleID, f.ActionID, f.MessageID, f.Status, errText, f.Payload,
	)
	return err
}
```

- [ ] **Step 5: Run tests**

```bash
cd D:/Work/Observer && go test ./pkg/store/...
```

- [ ] **Step 6: Commit**

```bash
cd D:/Work/Observer && git add pkg/store/ && git commit -m "feat(store): InsertRaw and InsertFired"
```

---

## Task 5: `pkg/rules/eval` — threshold evaluation

**Files:**
- Create: `pkg/rules/eval.go`, `pkg/rules/eval_test.go`

- [ ] **Step 1: Write the failing test**

Create `D:/Work/Observer/pkg/rules/eval_test.go`:

```go
package rules

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/observer-io/observer/pkg/models"
)

func mkRule(field string, op models.RuleOp, v float64) models.Rule {
	return models.Rule{
		ID: uuid.New(), TenantID: uuid.New(), DeviceID: uuid.New(),
		Field: field, Op: op, Value: v,
		ActionID: uuid.New(), Enabled: true,
	}
}

func TestEvaluate_Matches(t *testing.T) {
	payload := json.RawMessage(`{"temperature": 85, "battery": 10}`)

	cases := []struct {
		name string
		rule models.Rule
		want bool
	}{
		{"gt true", mkRule("temperature", models.OpGT, 80), true},
		{"gt false", mkRule("temperature", models.OpGT, 100), false},
		{"lt true", mkRule("battery", models.OpLT, 20), true},
		{"eq true", mkRule("temperature", models.OpEQ, 85), true},
		{"neq true", mkRule("temperature", models.OpNEQ, 90), true},
		{"missing field", mkRule("missing", models.OpGT, 0), false},
	}
	for _, tc := range cases {
		got, err := Evaluate(tc.rule, payload)
		if err != nil {
			t.Errorf("%s: err %v", tc.name, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}

func TestEvaluate_NonNumericField_NoMatch(t *testing.T) {
	payload := json.RawMessage(`{"temperature": "hot"}`)
	match, err := Evaluate(mkRule("temperature", models.OpGT, 0), payload)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if match {
		t.Error("non-numeric field should not match")
	}
}
```

- [ ] **Step 2: Run to verify failure**

```bash
cd D:/Work/Observer && go test ./pkg/rules/...
```

- [ ] **Step 3: Implement `pkg/rules/eval.go`**

Create `D:/Work/Observer/pkg/rules/eval.go`:

```go
// Package rules provides rule caching and threshold evaluation.
package rules

import (
	"encoding/json"

	"github.com/observer-io/observer/pkg/models"
)

// Evaluate reports whether the rule matches the given JSON payload.
// Non-numeric or missing fields never match.
func Evaluate(r models.Rule, payload []byte) (bool, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(payload, &obj); err != nil {
		return false, err
	}
	raw, ok := obj[r.Field]
	if !ok {
		return false, nil
	}
	var v float64
	if err := json.Unmarshal(raw, &v); err != nil {
		return false, nil
	}
	switch r.Op {
	case models.OpGT:
		return v > r.Value, nil
	case models.OpLT:
		return v < r.Value, nil
	case models.OpGTE:
		return v >= r.Value, nil
	case models.OpLTE:
		return v <= r.Value, nil
	case models.OpEQ:
		return v == r.Value, nil
	case models.OpNEQ:
		return v != r.Value, nil
	}
	return false, nil
}
```

- [ ] **Step 4: Run tests**

```bash
cd D:/Work/Observer && go test ./pkg/rules/...
```

- [ ] **Step 5: Commit**

```bash
cd D:/Work/Observer && git add pkg/rules/ && git commit -m "feat(rules): threshold Evaluate"
```

---

## Task 6: `pkg/rules/cache` — in-memory rule cache

**Files:**
- Create: `pkg/rules/cache.go`, `pkg/rules/cache_test.go`

- [ ] **Step 1: Write the failing test**

Create `D:/Work/Observer/pkg/rules/cache_test.go`:

```go
package rules

import (
	"testing"

	"github.com/google/uuid"

	"github.com/observer-io/observer/pkg/models"
)

func TestCache_GetByDevice(t *testing.T) {
	c := NewCache()
	d1 := uuid.New()
	d2 := uuid.New()
	r1 := models.Rule{ID: uuid.New(), DeviceID: d1, Enabled: true}
	r2 := models.Rule{ID: uuid.New(), DeviceID: d1, Enabled: true}
	r3 := models.Rule{ID: uuid.New(), DeviceID: d2, Enabled: true}
	r4 := models.Rule{ID: uuid.New(), DeviceID: d1, Enabled: false} // skipped

	c.Replace([]models.Rule{r1, r2, r3, r4})

	got := c.GetByDevice(d1)
	if len(got) != 2 {
		t.Fatalf("d1 rules: got %d want 2", len(got))
	}
	if len(c.GetByDevice(d2)) != 1 {
		t.Error("d2 rules")
	}
	if len(c.GetByDevice(uuid.New())) != 0 {
		t.Error("unknown device should return 0")
	}
}
```

- [ ] **Step 2: Run to verify failure**

```bash
cd D:/Work/Observer && go test ./pkg/rules/...
```

- [ ] **Step 3: Implement**

Create `D:/Work/Observer/pkg/rules/cache.go`:

```go
package rules

import (
	"sync"

	"github.com/google/uuid"

	"github.com/observer-io/observer/pkg/models"
)

type Cache struct {
	mu    sync.RWMutex
	index map[uuid.UUID][]models.Rule
}

func NewCache() *Cache {
	return &Cache{index: map[uuid.UUID][]models.Rule{}}
}

// Replace atomically swaps the cache contents. Disabled rules are excluded.
func (c *Cache) Replace(rules []models.Rule) {
	idx := make(map[uuid.UUID][]models.Rule, len(rules))
	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		idx[r.DeviceID] = append(idx[r.DeviceID], r)
	}
	c.mu.Lock()
	c.index = idx
	c.mu.Unlock()
}

// GetByDevice returns a snapshot slice (safe to iterate).
func (c *Cache) GetByDevice(deviceID uuid.UUID) []models.Rule {
	c.mu.RLock()
	defer c.mu.RUnlock()
	src := c.index[deviceID]
	if len(src) == 0 {
		return nil
	}
	out := make([]models.Rule, len(src))
	copy(out, src)
	return out
}
```

- [ ] **Step 4: Tests pass**

```bash
cd D:/Work/Observer && go test ./pkg/rules/...
```

- [ ] **Step 5: Commit**

```bash
cd D:/Work/Observer && git add pkg/rules/cache.go pkg/rules/cache_test.go && git commit -m "feat(rules): in-memory cache keyed by device"
```

---

## Task 7: `pkg/rules/loader` — load from DB + LISTEN/NOTIFY

**Files:**
- Create: `pkg/rules/loader.go`

- [ ] **Step 1: Implement the loader** (no dedicated test — covered by integration in Task 11)

Create `D:/Work/Observer/pkg/rules/loader.go`:

```go
package rules

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/observer-io/observer/pkg/models"
)

// LoadAll reads all enabled rules from the database.
func LoadAll(ctx context.Context, pool *pgxpool.Pool) ([]models.Rule, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, tenant_id, device_id, field, op, value, action_id, enabled, debug_until, created_at
		 FROM rules WHERE enabled = TRUE`)
	if err != nil {
		return nil, fmt.Errorf("query rules: %w", err)
	}
	defer rows.Close()

	var out []models.Rule
	for rows.Next() {
		var r models.Rule
		var op string
		if err := rows.Scan(&r.ID, &r.TenantID, &r.DeviceID, &r.Field, &op, &r.Value, &r.ActionID, &r.Enabled, &r.DebugUntil, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan rule: %w", err)
		}
		r.Op = models.RuleOp(op)
		out = append(out, r)
	}
	return out, rows.Err()
}

// WatchAndRefresh runs until ctx is done. On each NOTIFY or on connection failure,
// it re-runs LoadAll and atomically replaces the cache contents. Logs errors.
func WatchAndRefresh(ctx context.Context, pool *pgxpool.Pool, cache *Cache, logger *slog.Logger) error {
	// Initial load
	if err := refresh(ctx, pool, cache); err != nil {
		logger.Error("rules initial load", "err", err)
	}

	for {
		if err := listenLoop(ctx, pool, cache, logger); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			logger.Warn("rules listener lost, reconnecting", "err", err)
		}
	}
}

func listenLoop(ctx context.Context, pool *pgxpool.Pool, cache *Cache, logger *slog.Logger) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "LISTEN rules_changed"); err != nil {
		return err
	}
	for {
		_, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			return err
		}
		if err := refresh(ctx, pool, cache); err != nil {
			logger.Error("rules refresh", "err", err)
		}
	}
}

func refresh(ctx context.Context, pool *pgxpool.Pool, cache *Cache) error {
	rs, err := LoadAll(ctx, pool)
	if err != nil {
		return err
	}
	cache.Replace(rs)
	return nil
}

// ensure import used
var _ = pgx.ErrNoRows
```

- [ ] **Step 2: Build**

```bash
cd D:/Work/Observer && go build ./pkg/rules/...
```

- [ ] **Step 3: Commit**

```bash
cd D:/Work/Observer && git add pkg/rules/loader.go && git commit -m "feat(rules): DB loader with LISTEN/NOTIFY refresh"
```

---

## Task 8: `pkg/actions` — Action interface + log + webhook implementations

**Files:**
- Create: `pkg/actions/action.go`, `pkg/actions/log.go`, `pkg/actions/webhook.go`, `pkg/actions/action_test.go`

- [ ] **Step 1: Write the failing test**

Create `D:/Work/Observer/pkg/actions/action_test.go`:

```go
package actions

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/observer-io/observer/pkg/models"
)

func TestLogAction_Run(t *testing.T) {
	a := LogAction{Logger: slog.New(slog.NewJSONHandler(io.Discard, nil))}
	err := a.Run(context.Background(), Input{
		Action:    models.Action{ID: uuid.New(), Kind: models.ActionLog},
		DeviceID:  uuid.New(),
		MessageID: uuid.New(),
		Payload:   []byte(`{"temperature":90}`),
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
}

func TestWebhookAction_Run_Success(t *testing.T) {
	got := make(chan []byte, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got <- body
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg, _ := json.Marshal(map[string]string{"url": srv.URL})
	a := WebhookAction{}
	err := a.Run(context.Background(), Input{
		Action:    models.Action{ID: uuid.New(), Kind: models.ActionWebhook, Config: cfg},
		DeviceID:  uuid.New(),
		MessageID: uuid.New(),
		Payload:   []byte(`{"temperature":90}`),
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	select {
	case body := <-got:
		if len(body) == 0 {
			t.Error("empty body")
		}
	default:
		t.Error("webhook not called")
	}
}

func TestWebhookAction_Run_Non2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	cfg, _ := json.Marshal(map[string]string{"url": srv.URL})
	a := WebhookAction{}
	err := a.Run(context.Background(), Input{
		Action:    models.Action{ID: uuid.New(), Kind: models.ActionWebhook, Config: cfg},
		DeviceID:  uuid.New(),
		MessageID: uuid.New(),
		Payload:   []byte(`{}`),
	})
	if err == nil {
		t.Error("expected error on 500")
	}
}
```

- [ ] **Step 2: Run to verify failure**

```bash
cd D:/Work/Observer && go test ./pkg/actions/...
```

- [ ] **Step 3: Implement `pkg/actions/action.go`**

Create `D:/Work/Observer/pkg/actions/action.go`:

```go
// Package actions defines the Action interface and concrete implementations.
package actions

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/observer-io/observer/pkg/models"
)

type Input struct {
	Action    models.Action
	RuleID    uuid.UUID
	DeviceID  uuid.UUID
	TenantID  uuid.UUID
	MessageID uuid.UUID
	Payload   []byte // raw telemetry JSON
}

type Runner interface {
	Run(ctx context.Context, in Input) error
}

// Registry picks an implementation per action kind.
type Registry struct {
	Log     Runner
	Webhook Runner
}

func (r Registry) Run(ctx context.Context, in Input) error {
	switch in.Action.Kind {
	case models.ActionLog:
		if r.Log == nil {
			return fmt.Errorf("log action not configured")
		}
		return r.Log.Run(ctx, in)
	case models.ActionWebhook:
		if r.Webhook == nil {
			return fmt.Errorf("webhook action not configured")
		}
		return r.Webhook.Run(ctx, in)
	default:
		return fmt.Errorf("unknown action kind: %s", in.Action.Kind)
	}
}
```

- [ ] **Step 4: Implement `pkg/actions/log.go`**

Create `D:/Work/Observer/pkg/actions/log.go`:

```go
package actions

import (
	"context"
	"log/slog"
)

type LogAction struct {
	Logger *slog.Logger
}

func (a LogAction) Run(_ context.Context, in Input) error {
	a.Logger.Info("ALERT",
		"rule_id", in.RuleID,
		"device_id", in.DeviceID,
		"message_id", in.MessageID,
		"payload", string(in.Payload),
	)
	return nil
}
```

- [ ] **Step 5: Implement `pkg/actions/webhook.go`**

Create `D:/Work/Observer/pkg/actions/webhook.go`:

```go
package actions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type WebhookAction struct {
	Client *http.Client
}

type webhookConfig struct {
	URL string `json:"url"`
}

func (a WebhookAction) Run(ctx context.Context, in Input) error {
	var cfg webhookConfig
	if err := json.Unmarshal(in.Action.Config, &cfg); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	if cfg.URL == "" {
		return fmt.Errorf("webhook url is empty")
	}

	body, _ := json.Marshal(map[string]any{
		"rule_id":    in.RuleID,
		"device_id":  in.DeviceID,
		"tenant_id":  in.TenantID,
		"message_id": in.MessageID,
		"payload":    json.RawMessage(in.Payload),
	})

	client := a.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook status %d", resp.StatusCode)
	}
	return nil
}
```

- [ ] **Step 6: Tests pass**

```bash
cd D:/Work/Observer && go test ./pkg/actions/...
```

- [ ] **Step 7: Commit**

```bash
cd D:/Work/Observer && git add pkg/actions/ && git commit -m "feat(actions): log and webhook runners with registry"
```

---

## Task 9: `pkg/mqtt` — shared-subscription consumer

**Files:**
- Create: `pkg/mqtt/mqtt.go`

- [ ] **Step 1: Add paho client dep**

```bash
cd D:/Work/Observer && go get github.com/eclipse/paho.mqtt.golang@v1.5.0
```

- [ ] **Step 2: Implement (no unit test — covered by end-to-end smoke in Task 11)**

Create `D:/Work/Observer/pkg/mqtt/mqtt.go`:

```go
// Package mqtt wraps paho to subscribe to an MQTT shared subscription and
// deliver messages onto a Go channel.
package mqtt

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
)

type Message struct {
	Topic   string
	Payload []byte
}

// Consumer manages a single MQTT client subscribed to a shared topic.
type Consumer struct {
	client paho.Client
	logger *slog.Logger
}

type Options struct {
	BrokerURL  string
	ClientID   string   // must be unique per connecting node
	ShareGroup string   // shared-subscription group name
	Topic      string   // topic filter (wildcards OK), WITHOUT the $share prefix
	Logger     *slog.Logger
}

// Start connects, subscribes to $share/<ShareGroup>/<Topic>, and returns a channel
// that receives messages until Stop is called. The returned error only covers
// connection and initial subscription failures.
func (o Options) Start(ctx context.Context) (*Consumer, <-chan Message, error) {
	ch := make(chan Message, 1024)

	opts := paho.NewClientOptions().
		AddBroker(o.BrokerURL).
		SetClientID(o.ClientID).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(2 * time.Second).
		SetOrderMatters(false)

	opts.OnConnect = func(c paho.Client) {
		shareTopic := fmt.Sprintf("$share/%s/%s", o.ShareGroup, o.Topic)
		tok := c.Subscribe(shareTopic, 0, func(_ paho.Client, m paho.Message) {
			select {
			case ch <- Message{Topic: m.Topic(), Payload: m.Payload()}:
			default:
				o.Logger.Warn("mqtt drop (channel full)", "topic", m.Topic())
			}
		})
		tok.Wait()
		if err := tok.Error(); err != nil {
			o.Logger.Error("mqtt subscribe", "err", err)
		} else {
			o.Logger.Info("mqtt subscribed", "share_topic", shareTopic)
		}
	}
	opts.OnConnectionLost = func(_ paho.Client, err error) {
		o.Logger.Warn("mqtt connection lost", "err", err)
	}

	client := paho.NewClient(opts)
	tok := client.Connect()
	tok.WaitTimeout(10 * time.Second)
	if err := tok.Error(); err != nil {
		close(ch)
		return nil, nil, fmt.Errorf("mqtt connect: %w", err)
	}

	c := &Consumer{client: client, logger: o.Logger}
	// Close channel when context done
	go func() {
		<-ctx.Done()
		c.client.Disconnect(500)
		close(ch)
	}()
	return c, ch, nil
}

func (c *Consumer) Stop() {
	c.client.Disconnect(500)
}
```

- [ ] **Step 3: Build**

```bash
cd D:/Work/Observer && go build ./pkg/mqtt/...
```

- [ ] **Step 4: Commit**

```bash
cd D:/Work/Observer && git add pkg/mqtt/ go.mod go.sum && git commit -m "feat(mqtt): paho-based shared-subscription consumer"
```

---

## Task 10: Wire `cmd/transport` — full ingest pipeline

**Files:**
- Modify: `cmd/transport/main.go`

- [ ] **Step 1: Replace `cmd/transport/main.go` with the wired version**

Overwrite `D:/Work/Observer/cmd/transport/main.go`:

```go
// Command transport consumes MQTT telemetry, writes raw rows, evaluates rules,
// and enqueues action jobs.
package main

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/observer-io/observer/pkg/config"
	"github.com/observer-io/observer/pkg/db"
	"github.com/observer-io/observer/pkg/log"
	"github.com/observer-io/observer/pkg/mqtt"
	"github.com/observer-io/observer/pkg/queue"
	"github.com/observer-io/observer/pkg/queue/inmem"
	"github.com/observer-io/observer/pkg/rules"
	"github.com/observer-io/observer/pkg/store"
	"github.com/observer-io/observer/pkg/topic"
)

// Run is the library entry point so cmd/all can embed it.
func Run(ctx context.Context, cfg *config.Config, q queue.Queue) error {
	logger := log.New(cfg.Log.Level).With("svc", "transport")
	logger.Info("transport starting")

	pool, err := db.NewPool(ctx, cfg.DB.DSN)
	if err != nil {
		return err
	}
	defer pool.Close()

	cache := rules.NewCache()
	go func() {
		if err := rules.WatchAndRefresh(ctx, pool, cache, logger); err != nil {
			logger.Error("rules watcher exited", "err", err)
		}
	}()

	mc, ch, err := mqtt.Options{
		BrokerURL:  cfg.MQTT.URL,
		ClientID:   "transport-" + uuid.NewString(),
		ShareGroup: "transport",
		Topic:      "tenants/+/devices/+/telemetry",
		Logger:     logger,
	}.Start(ctx)
	if err != nil {
		return err
	}
	defer mc.Stop()

	logger.Info("transport ready")

	for {
		select {
		case <-ctx.Done():
			logger.Info("transport shutting down")
			return nil
		case msg, ok := <-ch:
			if !ok {
				return nil
			}
			handle(ctx, logger, pool, cache, q, msg)
		}
	}
}

func handle(ctx context.Context, logger interface {
	Error(string, ...any)
	Warn(string, ...any)
}, pool any, cache *rules.Cache, q queue.Queue, msg mqtt.Message) {
	// Parse topic
	parsed, err := topic.ParseTelemetry(msg.Topic)
	if err != nil {
		logger.Warn("bad topic", "topic", msg.Topic)
		return
	}
	// Parse payload (must be a JSON object)
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(msg.Payload, &obj); err != nil {
		logger.Warn("bad payload", "err", err)
		return
	}
	messageID, _ := uuid.NewV7()

	// Look up tenant from device (cheap: could cache later; for PoC query rules only)
	// For PoC we infer tenantID from the first rule's TenantID — if no rules, we still write raw
	// with a zero tenant placeholder. API plan will normalize this via a device->tenant cache.
	// Simpler: run a short query. But for PoC, carry tenant via topic and resolve lazily.
	// Shortcut: we don't *persist* tenant_id yet into telemetry_raw here; we pick it from rules if any fire.
	// Because telemetry_raw.tenant_id is NOT NULL, resolve it from devices table.
	var tenantID uuid.UUID
	if p, ok := pool.(interface {
		QueryRow(context.Context, string, ...any) interface{ Scan(...any) error }
	}); ok {
		_ = p.QueryRow(ctx, `SELECT tenant_id FROM devices WHERE id=$1`, parsed.DeviceID).Scan(&tenantID)
	}
	if tenantID == uuid.Nil {
		logger.Warn("unknown device", "device_id", parsed.DeviceID)
		return
	}

	// Persist raw
	if p, ok := pool.(*pgxTypedPool); ok { //nolint:staticcheck // type assertion placeholder; actual call below
		_ = p
	}
	// The pool is *pgxpool.Pool; use a concrete helper:
	if err := store.InsertRaw(ctx, getPool(pool), store.RawRow{
		Time: time.Now().UTC(), TenantID: tenantID, DeviceID: parsed.DeviceID,
		MessageID: messageID, Payload: msg.Payload,
	}); err != nil {
		logger.Error("insert raw", "err", err)
	}

	// Evaluate rules
	for _, r := range cache.GetByDevice(parsed.DeviceID) {
		match, err := rules.Evaluate(r, msg.Payload)
		if err != nil {
			logger.Warn("rule eval", "err", err)
			continue
		}
		if !match {
			continue
		}
		job := queue.Job{
			ID:            uuid.New(),
			TenantID:      r.TenantID,
			DeviceID:      r.DeviceID,
			RuleID:        r.ID,
			ActionID:      r.ActionID,
			MessageID:     messageID,
			Payload:       msg.Payload,
			CorrelationID: messageID,
		}
		if err := q.Enqueue(ctx, job); err != nil {
			logger.Error("enqueue", "err", err)
		}
	}
}

// pgxTypedPool / getPool exist so this file compiles without importing pgxpool directly
// into the handle signature (keeps the handle function testable later with fakes).
type pgxTypedPool struct{}

func getPool(p any) *pgxTypedPoolReal { return p.(*pgxTypedPoolReal) }

// pgxTypedPoolReal is an alias for *pgxpool.Pool — resolved via real import below.
// This indirection is ugly; see NOTE in Task 10 step 2: we will replace with a direct
// *pgxpool.Pool parameter.

// main remains for standalone runs
func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	q := inmem.New(4096)
	defer q.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := Run(ctx, cfg, q); err != nil {
		_ = err
		os.Exit(1)
	}
}
```

**STOP — the above file has placeholder indirection I need to remove. Replace the entire file with the clean version in Step 2.**

- [ ] **Step 2: Replace with the clean version**

Overwrite `D:/Work/Observer/cmd/transport/main.go` with this clean version:

```go
// Command transport consumes MQTT telemetry, writes raw rows, evaluates rules,
// and enqueues action jobs.
package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/observer-io/observer/pkg/config"
	"github.com/observer-io/observer/pkg/db"
	observerlog "github.com/observer-io/observer/pkg/log"
	"github.com/observer-io/observer/pkg/mqtt"
	"github.com/observer-io/observer/pkg/queue"
	"github.com/observer-io/observer/pkg/queue/inmem"
	"github.com/observer-io/observer/pkg/rules"
	"github.com/observer-io/observer/pkg/store"
	"github.com/observer-io/observer/pkg/topic"
)

// Run is the library entry point so cmd/all can embed it.
func Run(ctx context.Context, cfg *config.Config, q queue.Queue) error {
	logger := observerlog.New(cfg.Log.Level).With("svc", "transport")
	logger.Info("transport starting")

	pool, err := db.NewPool(ctx, cfg.DB.DSN)
	if err != nil {
		return err
	}
	defer pool.Close()

	cache := rules.NewCache()
	go func() {
		if err := rules.WatchAndRefresh(ctx, pool, cache, logger); err != nil {
			logger.Error("rules watcher exited", "err", err)
		}
	}()

	mc, ch, err := mqtt.Options{
		BrokerURL:  cfg.MQTT.URL,
		ClientID:   "transport-" + uuid.NewString(),
		ShareGroup: "transport",
		Topic:      "tenants/+/devices/+/telemetry",
		Logger:     logger,
	}.Start(ctx)
	if err != nil {
		return err
	}
	defer mc.Stop()

	logger.Info("transport ready")

	for {
		select {
		case <-ctx.Done():
			logger.Info("transport shutting down")
			return nil
		case msg, ok := <-ch:
			if !ok {
				return nil
			}
			handle(ctx, logger, pool, cache, q, msg)
		}
	}
}

func handle(ctx context.Context, logger *slog.Logger, pool *pgxpool.Pool, cache *rules.Cache, q queue.Queue, msg mqtt.Message) {
	parsed, err := topic.ParseTelemetry(msg.Topic)
	if err != nil {
		logger.Warn("bad topic", "topic", msg.Topic)
		return
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(msg.Payload, &obj); err != nil {
		logger.Warn("bad payload", "err", err)
		return
	}

	messageID, _ := uuid.NewV7()

	var tenantID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT tenant_id FROM devices WHERE id=$1`, parsed.DeviceID).Scan(&tenantID); err != nil {
		logger.Warn("unknown device", "device_id", parsed.DeviceID, "err", err)
		return
	}

	if err := store.InsertRaw(ctx, pool, store.RawRow{
		Time: time.Now().UTC(), TenantID: tenantID, DeviceID: parsed.DeviceID,
		MessageID: messageID, Payload: msg.Payload,
	}); err != nil {
		logger.Error("insert raw", "err", err)
	}

	for _, r := range cache.GetByDevice(parsed.DeviceID) {
		match, err := rules.Evaluate(r, msg.Payload)
		if err != nil {
			logger.Warn("rule eval", "err", err)
			continue
		}
		if !match {
			continue
		}
		job := queue.Job{
			ID:            uuid.New(),
			TenantID:      r.TenantID,
			DeviceID:      r.DeviceID,
			RuleID:        r.ID,
			ActionID:      r.ActionID,
			MessageID:     messageID,
			Payload:       msg.Payload,
			CorrelationID: messageID,
		}
		if err := q.Enqueue(ctx, job); err != nil {
			logger.Error("enqueue", "err", err)
		}
	}
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	q := inmem.New(4096)
	defer q.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := Run(ctx, cfg, q); err != nil {
		os.Exit(1)
	}
}
```

- [ ] **Step 3: Build**

```bash
cd D:/Work/Observer && go build ./...
```

- [ ] **Step 4: Commit**

```bash
cd D:/Work/Observer && git add cmd/transport/main.go && git commit -m "feat(transport): wire mqtt→store→eval→enqueue pipeline"
```

---

## Task 11: Wire `cmd/runner` — consume queue, run actions, record fired_actions

**Files:**
- Modify: `cmd/runner/main.go`

- [ ] **Step 1: Overwrite `cmd/runner/main.go`**

```go
// Command runner consumes action jobs from the queue and executes them.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/observer-io/observer/pkg/actions"
	"github.com/observer-io/observer/pkg/config"
	"github.com/observer-io/observer/pkg/db"
	observerlog "github.com/observer-io/observer/pkg/log"
	"github.com/observer-io/observer/pkg/models"
	"github.com/observer-io/observer/pkg/queue"
	"github.com/observer-io/observer/pkg/queue/inmem"
	"github.com/observer-io/observer/pkg/store"
)

func Run(ctx context.Context, cfg *config.Config, q queue.Queue) error {
	logger := observerlog.New(cfg.Log.Level).With("svc", "runner")
	logger.Info("runner starting")

	pool, err := db.NewPool(ctx, cfg.DB.DSN)
	if err != nil {
		return err
	}
	defer pool.Close()

	reg := actions.Registry{
		Log:     actions.LogAction{Logger: logger},
		Webhook: actions.WebhookAction{Client: &http.Client{Timeout: 5 * time.Second}},
	}

	logger.Info("runner ready")
	return q.Consume(ctx, func(jctx context.Context, j queue.Job) error {
		return execute(jctx, logger, pool, reg, j)
	})
}

func execute(ctx context.Context, logger *slog.Logger, pool *pgxpool.Pool, reg actions.Registry, j queue.Job) error {
	var action models.Action
	var kind string
	if err := pool.QueryRow(ctx,
		`SELECT id, tenant_id, kind, config, created_at FROM actions WHERE id=$1`,
		j.ActionID,
	).Scan(&action.ID, &action.TenantID, &kind, &action.Config, &action.CreatedAt); err != nil {
		logger.Error("load action", "err", err, "action_id", j.ActionID)
		return nil // drop; don't crash the loop
	}
	action.Kind = models.ActionKind(kind)

	runErr := reg.Run(ctx, actions.Input{
		Action: action, RuleID: j.RuleID, DeviceID: j.DeviceID, TenantID: j.TenantID,
		MessageID: j.MessageID, Payload: j.Payload,
	})

	status := "ok"
	errText := ""
	if runErr != nil {
		status = "error"
		errText = runErr.Error()
		logger.Warn("action failed", "err", runErr)
	}
	if err := store.InsertFired(ctx, pool, store.FiredRow{
		TenantID: j.TenantID, DeviceID: j.DeviceID, RuleID: j.RuleID, ActionID: j.ActionID,
		MessageID: j.MessageID, Status: status, Error: errText, Payload: j.Payload,
	}); err != nil {
		logger.Error("insert fired", "err", err)
	}
	return nil
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	q := inmem.New(4096)
	defer q.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := Run(ctx, cfg, q); err != nil {
		os.Exit(1)
	}
}
```

- [ ] **Step 2: Build**

```bash
cd D:/Work/Observer && go build ./...
```

- [ ] **Step 3: Wire `cmd/all` to run both transport and runner sharing the same in-mem queue**

Overwrite `D:/Work/Observer/cmd/all/main.go`:

```go
// Command all runs transport, runner, and api in a single process (Mode A monolith).
package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/observer-io/observer/pkg/config"
	observerlog "github.com/observer-io/observer/pkg/log"
	"github.com/observer-io/observer/pkg/queue/inmem"

	runnerpkg "github.com/observer-io/observer/cmd/runner"
	transportpkg "github.com/observer-io/observer/cmd/transport"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	logger := observerlog.New(cfg.Log.Level).With("svc", "all")
	logger.Info("monolith starting")

	q := inmem.New(4096)
	defer q.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if err := transportpkg.Run(ctx, cfg, q); err != nil {
			logger.Error("transport exited", "err", err)
		}
	}()
	go func() {
		defer wg.Done()
		if err := runnerpkg.Run(ctx, cfg, q); err != nil {
			logger.Error("runner exited", "err", err)
		}
	}()

	<-ctx.Done()
	logger.Info("monolith shutting down")
	wg.Wait()
	_ = os.Stdout.Sync()
}
```

Note: `package main` from cmd/runner and cmd/transport is imported here — this requires them to be consumable as library packages. In Go, `package main` files can't be imported. **Resolve this by renaming the packages:** change `package main` to `package transport` and `package runner` respectively, and move the `main()` functions to tiny `cmd/transport/cmd/main.go` / `cmd/runner/cmd/main.go` launchers. **Simpler approach:** keep `cmd/transport` and `cmd/runner` as `package main` (for standalone builds), and duplicate the `Run` function into internal packages `internal/runservice/transport` and `internal/runservice/runner`.

**Chosen approach (simpler):** create `internal/runservice/transport` and `internal/runservice/runner` libraries, then have `cmd/transport/main.go` and `cmd/runner/main.go` become thin wrappers.

- [ ] **Step 4: Restructure to `internal/runservice/{transport,runner}`**

1. Move the `Run` function body from `cmd/transport/main.go` to a new file `D:/Work/Observer/internal/runservice/transport/transport.go` with `package transport`.
2. Move the `Run` function body from `cmd/runner/main.go` to `D:/Work/Observer/internal/runservice/runner/runner.go` with `package runner`.
3. Replace `cmd/transport/main.go` with a tiny launcher:

```go
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/observer-io/observer/internal/runservice/transport"
	"github.com/observer-io/observer/pkg/config"
	"github.com/observer-io/observer/pkg/queue/inmem"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	q := inmem.New(4096)
	defer q.Close()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := transport.Run(ctx, cfg, q); err != nil {
		os.Exit(1)
	}
}
```

4. Same for `cmd/runner/main.go`.
5. Update `cmd/all/main.go` imports to `internal/runservice/transport` and `internal/runservice/runner`.

- [ ] **Step 5: Build everything**

```bash
cd D:/Work/Observer && go build ./...
```

- [ ] **Step 6: Commit**

```bash
cd D:/Work/Observer && git add cmd/ internal/runservice/ && git commit -m "feat(runner): action dispatch + fired_actions logging; wire cmd/all"
```

---

## Task 12: End-to-end smoke test

**Files:** none (verification only; scripts may be ad-hoc)

- [ ] **Step 1: Start the stack**

```bash
cd D:/Work/Observer
docker compose -f deploy/docker-compose.yml up -d
OBSERVER_DB_DSN="postgres://observer:observer@localhost:5432/observer?sslmode=disable" \
OBSERVER_MQTT_URL="tcp://localhost:1883" \
go run ./cmd/migrate up
```

- [ ] **Step 2: Seed a tenant, device, action, and rule**

```bash
docker exec -i observer-postgres psql -U observer -d observer <<'SQL'
INSERT INTO tenants (slug, name) VALUES ('acme','Acme Inc') RETURNING id;
SQL
```

Capture the tenant UUID, then:

```bash
# export TENANT_ID=<uuid from above>
docker exec -i observer-postgres psql -U observer -d observer <<SQL
INSERT INTO devices (tenant_id, name) VALUES ('$TENANT_ID', 'boiler-01') RETURNING id;
SQL
# export DEVICE_ID=<uuid>
docker exec -i observer-postgres psql -U observer -d observer <<SQL
INSERT INTO actions (tenant_id, kind, config) VALUES ('$TENANT_ID','log','{}') RETURNING id;
SQL
# export ACTION_ID=<uuid>
docker exec -i observer-postgres psql -U observer -d observer <<SQL
INSERT INTO rules (tenant_id, device_id, field, op, value, action_id)
VALUES ('$TENANT_ID','$DEVICE_ID','temperature','>',80,'$ACTION_ID');
NOTIFY rules_changed, '$DEVICE_ID';
SQL
```

- [ ] **Step 3: Start the monolith in a separate terminal**

```bash
cd D:/Work/Observer
OBSERVER_DB_DSN="postgres://observer:observer@localhost:5432/observer?sslmode=disable" \
OBSERVER_MQTT_URL="tcp://localhost:1883" \
go run ./cmd/all
```

Expected logs (JSON): `transport starting`, `transport ready`, `runner starting`, `runner ready`, `mqtt subscribed`.

- [ ] **Step 4: Publish two messages**

From another shell:

```bash
docker run --rm --network host eclipse-mosquitto:2 mosquitto_pub \
  -h localhost -p 1883 \
  -t "tenants/acme/devices/$DEVICE_ID/telemetry" \
  -m '{"temperature": 72}'

docker run --rm --network host eclipse-mosquitto:2 mosquitto_pub \
  -h localhost -p 1883 \
  -t "tenants/acme/devices/$DEVICE_ID/telemetry" \
  -m '{"temperature": 90}'
```

Expected in the monolith logs: one `ALERT` entry for the `90` message (not for `72`).

- [ ] **Step 5: Verify DB state**

```bash
docker exec -i observer-postgres psql -U observer -d observer -c "SELECT count(*) FROM telemetry_raw;"
docker exec -i observer-postgres psql -U observer -d observer -c "SELECT status, count(*) FROM fired_actions GROUP BY status;"
```

Expected: `telemetry_raw` has 2 rows; `fired_actions` has 1 row with `status=ok`.

- [ ] **Step 6: Commit any cleanup** (none expected if everything works; if fixes were needed, commit them with `fix(...)` messages).

---

## Self-Review

**Spec coverage** (against plan §6 data flow and §12 action execution):
- MQTT shared subscription ✅ (Task 9)
- Parse topic + payload ✅ (Tasks 2, 10)
- Insert raw telemetry ✅ (Task 4 + wired in 10)
- In-memory rule cache + LISTEN/NOTIFY ✅ (Tasks 6, 7)
- Threshold eval ✅ (Task 5)
- Enqueue job on match ✅ (Task 10)
- Action registry (log + webhook) ✅ (Task 8)
- Runner consumes + records fired_actions ✅ (Task 11)
- End-to-end verification ✅ (Task 12)

**Deferred to later plans:**
- Batching + pre-aggregation (Plan 04)
- API + UI (Plans 03, 04)
- Auth (Plan 03)
- river queue backend (post-PoC)

**Placeholder scan:** none remain in the final task code. (Task 10 Step 1 had a deliberately-messy draft; Step 2 replaces it with the clean version — implementer should skip directly to Step 2 and ignore Step 1 code.)

**Type consistency:** `queue.Job` fields used in transport and runner match `pkg/queue/queue.go`. `models.Rule.Op` is a `RuleOp` string; `rules.Evaluate` uses `models.OpGT` etc. consistently. `store.RawRow` / `FiredRow` fields match the SQL schemas.

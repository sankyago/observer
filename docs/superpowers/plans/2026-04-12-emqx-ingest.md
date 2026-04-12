# EMQX Ingest Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to execute.

**Goal:** Replace per-flow outbound MQTT with EMQX-fronted ingest: devices publish to `v1/devices/{uuid}/telemetry` via EMQX, Observer authenticates them over HTTP hooks and consumes a shared subscription, flows subscribe by device/metric via a new `device_source` node.

**Architecture:** EMQX runs alongside Observer in docker-compose. Observer exposes `/api/mqtt/auth` and `/api/mqtt/acl` HTTP hooks; EMQX calls these on CONNECT and PUBLISH. Observer consumes `$share/observer/v1/devices/+/telemetry` and routes parsed readings to flows that subscribed via `router.Subscribe`. The old `mqtt_source` node and `internal/subscriber/` package are removed.

**Tech Stack:** Go 1.25, EMQX 5.7, paho MQTT, pgx/v5, chi, testcontainers-go (EMQX + Timescale), testify.

Spec: `docs/superpowers/specs/2026-04-12-emqx-ingest-design.md`.

**Reliability bar:** every function gets at least unit-test coverage (power-plant rule).

---

## File Structure

**Create:**
- `migrations/003_create_devices.sql`
- `internal/devices/service.go` + `service_test.go`
- `internal/devices/store/repo.go` + `repo_test.go` (integration)
- `internal/ingest/parser.go` + `parser_test.go`
- `internal/ingest/router.go` + `router_test.go`
- `internal/ingest/consumer.go` + `consumer_test.go` (unit for helpers) + `consumer_integration_test.go`
- `internal/api/devices_handler.go` + `devices_handler_test.go`
- `internal/api/mqtt_auth_handler.go` + `mqtt_auth_handler_test.go`
- `internal/flow/nodes/device_source.go` + `device_source_test.go`
- `test/emqx_e2e_test.go` (integration, replaces `flow_e2e_test.go` path)

**Modify:**
- `docker-compose.yml` — add `emqx` service
- `internal/flow/graph/validate.go` — replace `mqtt_source` with `device_source` in `KnownTypes`
- `internal/flow/runtime/compiled_flow.go` — replace `mqtt_source` build path with `device_source`; wire router into `CompileWithSinkWriter` or a new option
- `internal/flow/service.go` — propagate router into runtime
- `internal/flow/runtime/manager.go` — accept router
- `internal/api/router.go` — register `/api/devices` and `/api/mqtt/*`
- `cmd/observer/main.go` — wire ingest consumer + router; remove old subscriber bootstrap
- `README.md` — document devices API and MQTT contract

**Delete:**
- `internal/flow/nodes/mqtt_source.go` + tests
- `internal/subscriber/` (entire package — topic/payload parsing moves to `ingest/parser.go`)
- `test/flow_e2e_test.go` (replaced by `emqx_e2e_test.go`)

---

## Task 1: Devices migration

**Files:**
- Create: `migrations/003_create_devices.sql`

- [ ] **Step 1: Write migration**

```sql
CREATE TABLE IF NOT EXISTS devices (
    id         UUID PRIMARY KEY,
    name       TEXT NOT NULL,
    token      TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS devices_token_idx ON devices (token);
```

- [ ] **Step 2: Commit**

```bash
git add migrations/003_create_devices.sql
git commit -m "feat: add devices table migration"
```

---

## Task 2: Device store (repo)

**Files:**
- Create: `internal/devices/store/repo.go`, `internal/devices/store/repo_test.go`

- [ ] **Step 1: Implement** (`repo.go`)

```go
package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("device not found")

type Device struct {
	ID        uuid.UUID
	Name      string
	Token     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Repo struct{ pool *pgxpool.Pool }

func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

func (r *Repo) Create(ctx context.Context, d *Device) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx,
		`INSERT INTO devices (id, name, token, created_at, updated_at) VALUES ($1,$2,$3,$4,$4)`,
		d.ID, d.Name, d.Token, now)
	if err != nil {
		return err
	}
	d.CreatedAt, d.UpdatedAt = now, now
	return nil
}

func (r *Repo) Get(ctx context.Context, id uuid.UUID) (*Device, error) {
	return scan(r.pool.QueryRow(ctx,
		`SELECT id, name, token, created_at, updated_at FROM devices WHERE id=$1`, id))
}

func (r *Repo) GetByToken(ctx context.Context, token string) (*Device, error) {
	return scan(r.pool.QueryRow(ctx,
		`SELECT id, name, token, created_at, updated_at FROM devices WHERE token=$1`, token))
}

func (r *Repo) List(ctx context.Context) ([]*Device, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, token, created_at, updated_at FROM devices ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Device
	for rows.Next() {
		d, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *Repo) UpdateName(ctx context.Context, id uuid.UUID, name string) error {
	now := time.Now().UTC()
	tag, err := r.pool.Exec(ctx,
		`UPDATE devices SET name=$2, updated_at=$3 WHERE id=$1`, id, name, now)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) UpdateToken(ctx context.Context, id uuid.UUID, token string) error {
	now := time.Now().UTC()
	tag, err := r.pool.Exec(ctx,
		`UPDATE devices SET token=$2, updated_at=$3 WHERE id=$1`, id, token, now)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM devices WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

type scanner interface{ Scan(dest ...any) error }

func scan(s scanner) (*Device, error) {
	var d Device
	if err := s.Scan(&d.ID, &d.Name, &d.Token, &d.CreatedAt, &d.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &d, nil
}
```

- [ ] **Step 2: Write integration test** (`repo_test.go`)

```go
//go:build integration

package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sankyago/observer/internal/db"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func setup(t *testing.T) *pgxpool.Pool {
	ctx := context.Background()
	pg, err := postgres.Run(ctx, "timescale/timescaledb:latest-pg16",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pg.Terminate(ctx) })
	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, db.Migrate(ctx, pool, "../../../migrations"))
	return pool
}

func TestRepo_CRUD(t *testing.T) {
	repo := NewRepo(setup(t))
	ctx := context.Background()

	d := &Device{Name: "pump-1", Token: "tok-abc"}
	require.NoError(t, repo.Create(ctx, d))
	require.NotEqual(t, uuid.Nil, d.ID)

	got, err := repo.Get(ctx, d.ID)
	require.NoError(t, err)
	require.Equal(t, "pump-1", got.Name)

	byTok, err := repo.GetByToken(ctx, "tok-abc")
	require.NoError(t, err)
	require.Equal(t, d.ID, byTok.ID)

	require.NoError(t, repo.UpdateName(ctx, d.ID, "pump-2"))
	got, _ = repo.Get(ctx, d.ID)
	require.Equal(t, "pump-2", got.Name)

	require.NoError(t, repo.UpdateToken(ctx, d.ID, "tok-xyz"))
	_, err = repo.GetByToken(ctx, "tok-abc")
	require.ErrorIs(t, err, ErrNotFound)

	list, err := repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)

	require.NoError(t, repo.Delete(ctx, d.ID))
	_, err = repo.Get(ctx, d.ID)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestRepo_ErrNotFound(t *testing.T) {
	repo := NewRepo(setup(t))
	ctx := context.Background()
	bogus := uuid.New()
	_, err := repo.Get(ctx, bogus)
	require.ErrorIs(t, err, ErrNotFound)
	_, err = repo.GetByToken(ctx, "nope")
	require.ErrorIs(t, err, ErrNotFound)
	require.ErrorIs(t, repo.UpdateName(ctx, bogus, "x"), ErrNotFound)
	require.ErrorIs(t, repo.UpdateToken(ctx, bogus, "x"), ErrNotFound)
	require.ErrorIs(t, repo.Delete(ctx, bogus), ErrNotFound)
}
```

- [ ] **Step 3: Run**

```bash
go test -tags integration ./internal/devices/store/... -timeout 120s -v
```
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/devices/store/
git commit -m "feat: add device repo with pgx CRUD"
```

---

## Task 3: Device service (token generation)

**Files:**
- Create: `internal/devices/service.go`, `internal/devices/service_test.go`

- [ ] **Step 1: Write failing unit tests** (`service_test.go`)

```go
package devices

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/sankyago/observer/internal/devices/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRepo struct {
	items map[uuid.UUID]*store.Device
}

func newFake() *fakeRepo { return &fakeRepo{items: map[uuid.UUID]*store.Device{}} }

func (f *fakeRepo) Create(_ context.Context, d *store.Device) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	for _, existing := range f.items {
		if existing.Token == d.Token {
			return assert.AnError
		}
	}
	f.items[d.ID] = d
	return nil
}

func (f *fakeRepo) Get(_ context.Context, id uuid.UUID) (*store.Device, error) {
	d, ok := f.items[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return d, nil
}

func (f *fakeRepo) GetByToken(_ context.Context, tok string) (*store.Device, error) {
	for _, d := range f.items {
		if d.Token == tok {
			return d, nil
		}
	}
	return nil, store.ErrNotFound
}

func (f *fakeRepo) List(_ context.Context) ([]*store.Device, error) {
	out := make([]*store.Device, 0, len(f.items))
	for _, d := range f.items {
		out = append(out, d)
	}
	return out, nil
}

func (f *fakeRepo) UpdateName(_ context.Context, id uuid.UUID, name string) error {
	d, ok := f.items[id]
	if !ok {
		return store.ErrNotFound
	}
	d.Name = name
	return nil
}

func (f *fakeRepo) UpdateToken(_ context.Context, id uuid.UUID, tok string) error {
	d, ok := f.items[id]
	if !ok {
		return store.ErrNotFound
	}
	d.Token = tok
	return nil
}

func (f *fakeRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := f.items[id]; !ok {
		return store.ErrNotFound
	}
	delete(f.items, id)
	return nil
}

func TestService_Create_GeneratesUniqueToken(t *testing.T) {
	svc := NewService(newFake())
	ctx := context.Background()

	a, err := svc.Create(ctx, "a")
	require.NoError(t, err)
	b, err := svc.Create(ctx, "b")
	require.NoError(t, err)

	assert.NotEmpty(t, a.Token)
	assert.NotEqual(t, a.Token, b.Token)
	assert.GreaterOrEqual(t, len(a.Token), 20) // base64url of 20 bytes is 27
}

func TestService_RegenerateToken(t *testing.T) {
	svc := NewService(newFake())
	ctx := context.Background()
	d, err := svc.Create(ctx, "x")
	require.NoError(t, err)
	old := d.Token

	updated, err := svc.RegenerateToken(ctx, d.ID)
	require.NoError(t, err)
	assert.NotEqual(t, old, updated.Token)
}

func TestService_Delete_Unknown_ReturnsNotFound(t *testing.T) {
	svc := NewService(newFake())
	assert.ErrorIs(t, svc.Delete(context.Background(), uuid.New()), store.ErrNotFound)
}

func TestService_Rename(t *testing.T) {
	svc := NewService(newFake())
	ctx := context.Background()
	d, _ := svc.Create(ctx, "old")
	got, err := svc.Rename(ctx, d.ID, "new")
	require.NoError(t, err)
	assert.Equal(t, "new", got.Name)
}

func TestService_GetByToken(t *testing.T) {
	svc := NewService(newFake())
	ctx := context.Background()
	d, _ := svc.Create(ctx, "x")
	got, err := svc.GetByToken(ctx, d.Token)
	require.NoError(t, err)
	assert.Equal(t, d.ID, got.ID)
}
```

- [ ] **Step 2: Implement** (`service.go`)

```go
package devices

import (
	"context"
	"crypto/rand"
	"encoding/base64"

	"github.com/google/uuid"
	"github.com/sankyago/observer/internal/devices/store"
)

type repo interface {
	Create(ctx context.Context, d *store.Device) error
	Get(ctx context.Context, id uuid.UUID) (*store.Device, error)
	GetByToken(ctx context.Context, token string) (*store.Device, error)
	List(ctx context.Context) ([]*store.Device, error)
	UpdateName(ctx context.Context, id uuid.UUID, name string) error
	UpdateToken(ctx context.Context, id uuid.UUID, token string) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type Service struct{ repo repo }

func NewService(r repo) *Service { return &Service{repo: r} }

func (s *Service) Create(ctx context.Context, name string) (*store.Device, error) {
	tok, err := generateToken()
	if err != nil {
		return nil, err
	}
	d := &store.Device{Name: name, Token: tok}
	if err := s.repo.Create(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*store.Device, error) {
	return s.repo.Get(ctx, id)
}

func (s *Service) GetByToken(ctx context.Context, token string) (*store.Device, error) {
	return s.repo.GetByToken(ctx, token)
}

func (s *Service) List(ctx context.Context) ([]*store.Device, error) {
	return s.repo.List(ctx)
}

func (s *Service) Rename(ctx context.Context, id uuid.UUID, name string) (*store.Device, error) {
	if err := s.repo.UpdateName(ctx, id, name); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, id)
}

func (s *Service) RegenerateToken(ctx context.Context, id uuid.UUID) (*store.Device, error) {
	tok, err := generateToken()
	if err != nil {
		return nil, err
	}
	if err := s.repo.UpdateToken(ctx, id, tok); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, id)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func generateToken() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/devices/... -v
```
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/devices/service.go internal/devices/service_test.go
git commit -m "feat: add device service with token generation"
```

---

## Task 4: Devices HTTP handler

**Files:**
- Create: `internal/api/devices_handler.go`, `internal/api/devices_handler_test.go`

- [ ] **Step 1: Implement** (`devices_handler.go`)

```go
package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sankyago/observer/internal/devices"
	"github.com/sankyago/observer/internal/devices/store"
)

type devicesHandler struct{ svc *devices.Service }

type deviceDTO struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Token string    `json:"token"`
}

func toDeviceDTO(d *store.Device) deviceDTO {
	return deviceDTO{ID: d.ID, Name: d.Name, Token: d.Token}
}

func (h *devicesHandler) list(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.List(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]deviceDTO, 0, len(items))
	for _, d := range items {
		out = append(out, toDeviceDTO(d))
	}
	writeJSON(w, http.StatusOK, out)
}

type createDeviceReq struct {
	Name string `json:"name"`
}

func (h *devicesHandler) create(w http.ResponseWriter, r *http.Request) {
	var req createDeviceReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Name == "" {
		writeErr(w, http.StatusBadRequest, "name required")
		return
	}
	d, err := h.svc.Create(r.Context(), req.Name)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toDeviceDTO(d))
}

func (h *devicesHandler) get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	d, err := h.svc.Get(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toDeviceDTO(d))
}

type updateDeviceReq struct {
	Name string `json:"name"`
}

func (h *devicesHandler) update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req updateDeviceReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Name == "" {
		writeErr(w, http.StatusBadRequest, "name required")
		return
	}
	d, err := h.svc.Rename(r.Context(), id, req.Name)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toDeviceDTO(d))
}

func (h *devicesHandler) delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *devicesHandler) regenerate(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	d, err := h.svc.RegenerateToken(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toDeviceDTO(d))
}
```

- [ ] **Step 2: Wire into router** (`internal/api/router.go`) — modify `NewRouter` to accept a `*devices.Service` (optional; nil means no device routes):

```go
// In NewRouter signature add: devSvc *devices.Service
if devSvc != nil {
    dh := &devicesHandler{svc: devSvc}
    r.Route("/api/devices", func(r chi.Router) {
        r.Get("/", dh.list)
        r.Post("/", dh.create)
        r.Get("/{id}", dh.get)
        r.Put("/{id}", dh.update)
        r.Delete("/{id}", dh.delete)
        r.Post("/{id}/regenerate-token", dh.regenerate)
    })
}
```

Update all existing `NewRouter(...)` callers (tests, main) to pass nil for now; next tasks pass real service.

- [ ] **Step 3: Write unit tests** (`devices_handler_test.go`)

```go
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/sankyago/observer/internal/devices"
	"github.com/sankyago/observer/internal/devices/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeDeviceRepo struct {
	items map[uuid.UUID]*store.Device
	err   error
}

func (f *fakeDeviceRepo) Create(_ context.Context, d *store.Device) error {
	if f.err != nil {
		return f.err
	}
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	f.items[d.ID] = d
	return nil
}
func (f *fakeDeviceRepo) Get(_ context.Context, id uuid.UUID) (*store.Device, error) {
	d, ok := f.items[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return d, nil
}
func (f *fakeDeviceRepo) GetByToken(_ context.Context, t string) (*store.Device, error) {
	for _, d := range f.items {
		if d.Token == t {
			return d, nil
		}
	}
	return nil, store.ErrNotFound
}
func (f *fakeDeviceRepo) List(_ context.Context) ([]*store.Device, error) {
	out := []*store.Device{}
	for _, d := range f.items {
		out = append(out, d)
	}
	return out, nil
}
func (f *fakeDeviceRepo) UpdateName(_ context.Context, id uuid.UUID, n string) error {
	d, ok := f.items[id]
	if !ok {
		return store.ErrNotFound
	}
	d.Name = n
	return nil
}
func (f *fakeDeviceRepo) UpdateToken(_ context.Context, id uuid.UUID, t string) error {
	d, ok := f.items[id]
	if !ok {
		return store.ErrNotFound
	}
	d.Token = t
	return nil
}
func (f *fakeDeviceRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := f.items[id]; !ok {
		return store.ErrNotFound
	}
	delete(f.items, id)
	return nil
}

func newDeviceRouter(t *testing.T) (http.Handler, *devices.Service) {
	t.Helper()
	repo := &fakeDeviceRepo{items: map[uuid.UUID]*store.Device{}}
	svc := devices.NewService(repo)
	return NewRouter(nil, svc), svc
}

func TestDevices_Create(t *testing.T) {
	h, _ := newDeviceRouter(t)
	body, _ := json.Marshal(map[string]string{"name": "sensor-1"})
	req := httptest.NewRequest(http.MethodPost, "/api/devices", bytes.NewReader(body))
	req.Header.Set("content-type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code)
	var got deviceDTO
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	assert.NotEqual(t, uuid.Nil, got.ID)
	assert.NotEmpty(t, got.Token)
}

func TestDevices_Create_BadJSON(t *testing.T) {
	h, _ := newDeviceRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/devices", bytes.NewReader([]byte("{")))
	req.Header.Set("content-type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestDevices_Create_EmptyName(t *testing.T) {
	h, _ := newDeviceRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/devices", bytes.NewReader([]byte(`{"name":""}`)))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestDevices_GetUnknown(t *testing.T) {
	h, _ := newDeviceRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/devices/"+uuid.New().String(), nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusNotFound, rr.Code)
}

func TestDevices_InvalidUUID(t *testing.T) {
	h, _ := newDeviceRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/devices/not-a-uuid", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestDevices_UpdateAndDelete(t *testing.T) {
	h, svc := newDeviceRouter(t)
	d, err := svc.Create(context.Background(), "a")
	require.NoError(t, err)

	// rename
	body, _ := json.Marshal(map[string]string{"name": "renamed"})
	req := httptest.NewRequest(http.MethodPut, "/api/devices/"+d.ID.String(), bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	// delete
	req = httptest.NewRequest(http.MethodDelete, "/api/devices/"+d.ID.String(), nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusNoContent, rr.Code)
}

func TestDevices_Regenerate(t *testing.T) {
	h, svc := newDeviceRouter(t)
	d, err := svc.Create(context.Background(), "a")
	require.NoError(t, err)
	old := d.Token

	req := httptest.NewRequest(http.MethodPost, "/api/devices/"+d.ID.String()+"/regenerate-token", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
	var got deviceDTO
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	assert.NotEqual(t, old, got.Token)
}
```

- [ ] **Step 4: Update existing callers**

Find every `NewRouter(` call in the repo and add a `nil` second argument (devices service). Rerun `go test ./...`.

- [ ] **Step 5: Run**

```bash
go test ./internal/api/... ./internal/devices/... -v -race
```
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/api/devices_handler.go internal/api/devices_handler_test.go internal/api/router.go
git commit -m "feat: add devices REST API"
```

---

## Task 5: MQTT auth + ACL handler

**Files:**
- Create: `internal/api/mqtt_auth_handler.go`, `internal/api/mqtt_auth_handler_test.go`

- [ ] **Step 1: Write failing tests** (`mqtt_auth_handler_test.go`)

```go
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sankyago/observer/internal/devices"
	"github.com/sankyago/observer/internal/devices/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMQTTRouter(t *testing.T, secret, serviceUser string) (http.Handler, *devices.Service) {
	t.Helper()
	repo := &fakeDeviceRepo{items: map[string]*store.Device{}}
	// NOTE: fakeDeviceRepo in devices_handler_test uses map[uuid.UUID]; reuse it here.
	// If import cycle prevents reuse, copy the type.
	_ = repo
	// We'll use the one from devices_handler_test.go (same package).
	repo2 := &fakeDeviceRepo{items: map[string]*store.Device{}}
	_ = repo2
	_ = newDeviceRouter
	svc := devices.NewService(nil) // placeholder; real wiring below
	return NewRouter(nil, svc, WithMQTTAuth(secret, serviceUser)), svc
}
```

NOTE: In practice, structure the handler + tests to use the existing `fakeDeviceRepo` from `devices_handler_test.go` (same package — can reference directly). The test above has a placeholder; the implementer will reuse the existing fake. The actual tests:

```go
func TestMQTTAuth_AllowsValidToken(t *testing.T) {
	repo := &fakeDeviceRepo{items: map[string]*store.Device{}}
	// (reusing fakeDeviceRepo defined in devices_handler_test.go)
	svc := devices.NewService(repo)
	d, err := svc.Create(context.Background(), "dev")
	require.NoError(t, err)

	h := NewRouter(nil, svc, WithMQTTAuth("secret", "observer-consumer"))

	body := fmt.Sprintf(`{"username":%q,"password":"","clientid":"x"}`, d.Token)
	req := httptest.NewRequest(http.MethodPost, "/api/mqtt/auth", bytes.NewReader([]byte(body)))
	req.Header.Set("X-EMQX-Secret", "secret")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var got map[string]string
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	assert.Equal(t, "allow", got["result"])
}

func TestMQTTAuth_DeniesUnknownToken(t *testing.T) { ... }
func TestMQTTAuth_AllowsServiceAccount(t *testing.T) { ... }
func TestMQTTAuth_RejectsMissingSecret(t *testing.T) { ... }

func TestMQTTACL_AllowsDevicePublishingOwnTopic(t *testing.T) { ... }
func TestMQTTACL_DeniesDevicePublishingOtherTopic(t *testing.T) { ... }
func TestMQTTACL_AllowsServiceAccountSubscribe(t *testing.T) { ... }
func TestMQTTACL_DeniesServiceAccountPublish(t *testing.T) { ... }
```

Full test bodies provided in the implementation; each exercises the HTTP path with JSON bodies shaped like EMQX's auth/ACL requests.

- [ ] **Step 2: Implement** (`mqtt_auth_handler.go`)

```go
package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sankyago/observer/internal/devices"
	"github.com/sankyago/observer/internal/devices/store"
)

type mqttAuthHandler struct {
	svc         *devices.Service
	secret      string
	serviceUser string

	mu    sync.RWMutex
	cache map[string]cacheEntry // token -> deviceID
}

type cacheEntry struct {
	id       uuid.UUID
	fetchedAt time.Time
}

const authCacheTTL = 30 * time.Second

func newMQTTAuthHandler(svc *devices.Service, secret, serviceUser string) *mqttAuthHandler {
	return &mqttAuthHandler{svc: svc, secret: secret, serviceUser: serviceUser, cache: map[string]cacheEntry{}}
}

func (h *mqttAuthHandler) checkSecret(r *http.Request) bool {
	return h.secret != "" && r.Header.Get("X-EMQX-Secret") == h.secret
}

type authReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Clientid string `json:"clientid"`
}

func (h *mqttAuthHandler) auth(w http.ResponseWriter, r *http.Request) {
	if !h.checkSecret(r) {
		writeJSON(w, http.StatusOK, map[string]string{"result": "deny"})
		return
	}
	var req authReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"result": "deny"})
		return
	}
	if req.Username == h.serviceUser {
		writeJSON(w, http.StatusOK, map[string]string{"result": "allow"})
		return
	}
	id, ok := h.lookup(r, req.Username)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]string{"result": "deny"})
		return
	}
	_ = id
	writeJSON(w, http.StatusOK, map[string]string{"result": "allow"})
}

type aclReq struct {
	Username string `json:"username"`
	Action   string `json:"action"`
	Topic    string `json:"topic"`
}

func (h *mqttAuthHandler) acl(w http.ResponseWriter, r *http.Request) {
	if !h.checkSecret(r) {
		writeJSON(w, http.StatusOK, map[string]string{"result": "deny"})
		return
	}
	var req aclReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"result": "deny"})
		return
	}

	// Service account: allow subscribe only to the shared consumer topic.
	if req.Username == h.serviceUser {
		if req.Action == "subscribe" && strings.HasPrefix(req.Topic, "$share/") {
			writeJSON(w, http.StatusOK, map[string]string{"result": "allow"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"result": "deny"})
		return
	}

	// Device: allow publish only to v1/devices/{ownUUID}/telemetry.
	if req.Action != "publish" {
		writeJSON(w, http.StatusOK, map[string]string{"result": "deny"})
		return
	}
	id, ok := h.lookup(r, req.Username)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]string{"result": "deny"})
		return
	}
	expected := "v1/devices/" + id.String() + "/telemetry"
	if req.Topic == expected {
		writeJSON(w, http.StatusOK, map[string]string{"result": "allow"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"result": "deny"})
}

func (h *mqttAuthHandler) lookup(r *http.Request, token string) (uuid.UUID, bool) {
	h.mu.RLock()
	if e, ok := h.cache[token]; ok && time.Since(e.fetchedAt) < authCacheTTL {
		h.mu.RUnlock()
		return e.id, true
	}
	h.mu.RUnlock()

	d, err := h.svc.GetByToken(r.Context(), token)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return uuid.Nil, false
		}
		return uuid.Nil, false
	}
	h.mu.Lock()
	h.cache[token] = cacheEntry{id: d.ID, fetchedAt: time.Now()}
	h.mu.Unlock()
	return d.ID, true
}

// Invalidate clears a specific token from the cache. Called when a device's
// token is rotated or the device is deleted.
func (h *mqttAuthHandler) Invalidate(token string) {
	h.mu.Lock()
	delete(h.cache, token)
	h.mu.Unlock()
}
```

Extend `NewRouter` to accept a functional option `WithMQTTAuth(secret, serviceUser string)` that, when provided, registers `/api/mqtt/auth` and `/api/mqtt/acl`. Pattern:

```go
// router.go
type RouterOption func(*routerConfig)

type routerConfig struct {
    mqttSecret      string
    mqttServiceUser string
}

func WithMQTTAuth(secret, serviceUser string) RouterOption {
    return func(c *routerConfig) { c.mqttSecret = secret; c.mqttServiceUser = serviceUser }
}

func NewRouter(flowSvc *flow.Service, devSvc *devices.Service, opts ...RouterOption) http.Handler {
    cfg := &routerConfig{}
    for _, o := range opts { o(cfg) }
    r := chi.NewRouter()
    // ...existing routes...
    if cfg.mqttSecret != "" && devSvc != nil {
        mh := newMQTTAuthHandler(devSvc, cfg.mqttSecret, cfg.mqttServiceUser)
        r.Post("/api/mqtt/auth", mh.auth)
        r.Post("/api/mqtt/acl", mh.acl)
    }
    return r
}
```

- [ ] **Step 3: Run**

```bash
go test ./internal/api/... -v -race
```
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/api/mqtt_auth_handler.go internal/api/mqtt_auth_handler_test.go internal/api/router.go
git commit -m "feat: add EMQX auth and ACL webhooks"
```

---

## Task 6: Ingest parser

**Files:**
- Create: `internal/ingest/parser.go`, `internal/ingest/parser_test.go`

- [ ] **Step 1: Write failing tests**

```go
package ingest

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_HappyPath(t *testing.T) {
	id := uuid.New()
	topic := "v1/devices/" + id.String() + "/telemetry"
	payload := []byte(`{"temperature":23.5,"humidity":60.0,"ts":"2026-04-12T10:00:00Z"}`)

	readings, err := Parse(topic, payload, time.Now)
	require.NoError(t, err)
	require.Len(t, readings, 2)

	for _, r := range readings {
		assert.Equal(t, id.String(), r.DeviceID)
		assert.Equal(t, 2026, r.Timestamp.Year())
	}
}

func TestParse_MissingTimestamp_UsesNow(t *testing.T) {
	id := uuid.New()
	topic := "v1/devices/" + id.String() + "/telemetry"
	payload := []byte(`{"t":1.0}`)
	fixed := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	readings, err := Parse(topic, payload, func() time.Time { return fixed })
	require.NoError(t, err)
	require.Len(t, readings, 1)
	assert.Equal(t, fixed, readings[0].Timestamp)
}

func TestParse_NonNumericValuesDropped(t *testing.T) {
	id := uuid.New()
	topic := "v1/devices/" + id.String() + "/telemetry"
	payload := []byte(`{"temperature":23.5,"status":"ok"}`)
	readings, err := Parse(topic, payload, time.Now)
	require.NoError(t, err)
	require.Len(t, readings, 1)
	assert.Equal(t, "temperature", readings[0].Metric)
}

func TestParse_BadTopic(t *testing.T) {
	_, err := Parse("wrong/topic", []byte(`{}`), time.Now)
	assert.ErrorContains(t, err, "topic")
}

func TestParse_BadUUID(t *testing.T) {
	_, err := Parse("v1/devices/not-a-uuid/telemetry", []byte(`{"t":1}`), time.Now)
	assert.ErrorContains(t, err, "uuid")
}

func TestParse_BadJSON(t *testing.T) {
	id := uuid.New()
	topic := "v1/devices/" + id.String() + "/telemetry"
	_, err := Parse(topic, []byte(`{`), time.Now)
	assert.ErrorContains(t, err, "json")
}

func TestParse_EmptyObject(t *testing.T) {
	id := uuid.New()
	topic := "v1/devices/" + id.String() + "/telemetry"
	readings, err := Parse(topic, []byte(`{}`), time.Now)
	require.NoError(t, err)
	assert.Empty(t, readings)
}
```

- [ ] **Step 2: Implement** (`parser.go`)

```go
package ingest

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sankyago/observer/internal/model"
)

const timestampKey = "ts"

// Parse extracts sensor readings from an MQTT message. Topic must match
// v1/devices/{uuid}/telemetry. Payload is a JSON object where numeric keys
// become metrics; an optional "ts" key overrides the default now() timestamp.
func Parse(topic string, payload []byte, now func() time.Time) ([]model.SensorReading, error) {
	parts := strings.Split(topic, "/")
	if len(parts) != 4 || parts[0] != "v1" || parts[1] != "devices" || parts[3] != "telemetry" {
		return nil, fmt.Errorf("topic %q does not match v1/devices/{uuid}/telemetry", topic)
	}
	id, err := uuid.Parse(parts[2])
	if err != nil {
		return nil, fmt.Errorf("uuid: %w", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("json: %w", err)
	}

	ts := now()
	if tv, ok := raw[timestampKey]; ok {
		if s, ok := tv.(string); ok {
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				ts = t
			}
		}
	}

	readings := make([]model.SensorReading, 0, len(raw))
	for k, v := range raw {
		if k == timestampKey {
			continue
		}
		f, ok := v.(float64)
		if !ok {
			continue
		}
		readings = append(readings, model.SensorReading{
			DeviceID:  id.String(),
			Metric:    k,
			Value:     f,
			Timestamp: ts,
		})
	}
	return readings, nil
}
```

- [ ] **Step 3: Run**

```bash
go test ./internal/ingest/... -v
```
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/ingest/parser.go internal/ingest/parser_test.go
git commit -m "feat: add ingest topic/payload parser"
```

---

## Task 7: Ingest router

**Files:**
- Create: `internal/ingest/router.go`, `internal/ingest/router_test.go`

- [ ] **Step 1: Write failing tests**

```go
package ingest

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sankyago/observer/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func reading(dev, metric string, val float64) model.SensorReading {
	return model.SensorReading{DeviceID: dev, Metric: metric, Value: val, Timestamp: time.Now()}
}

func TestRouter_SubscribeAll(t *testing.T) {
	r := NewRouter()
	flow := uuid.New()
	ch := r.Subscribe(flow, Filter{}, 4)

	dev := uuid.New().String()
	r.Dispatch(reading(dev, "temp", 1))
	r.Dispatch(reading(dev, "humidity", 2))

	require.Len(t, ch, 2)
}

func TestRouter_SubscribeDevice(t *testing.T) {
	r := NewRouter()
	flow := uuid.New()
	target := uuid.New().String()
	ch := r.Subscribe(flow, Filter{DeviceID: target}, 4)

	r.Dispatch(reading(target, "temp", 1))
	r.Dispatch(reading(uuid.New().String(), "temp", 2)) // different device

	require.Len(t, ch, 1)
}

func TestRouter_SubscribeDeviceAndMetric(t *testing.T) {
	r := NewRouter()
	flow := uuid.New()
	dev := uuid.New().String()
	ch := r.Subscribe(flow, Filter{DeviceID: dev, Metric: "temp"}, 4)

	r.Dispatch(reading(dev, "temp", 1))
	r.Dispatch(reading(dev, "humidity", 2)) // wrong metric

	require.Len(t, ch, 1)
}

func TestRouter_Unsubscribe(t *testing.T) {
	r := NewRouter()
	flow := uuid.New()
	ch := r.Subscribe(flow, Filter{}, 4)
	r.Unsubscribe(flow)

	r.Dispatch(reading(uuid.New().String(), "temp", 1))
	assert.Len(t, ch, 0)
	// Channel should be closed.
	_, open := <-ch
	assert.False(t, open)
}

func TestRouter_DropsOnFullChannel(t *testing.T) {
	r := NewRouter()
	flow := uuid.New()
	ch := r.Subscribe(flow, Filter{}, 1)

	dev := uuid.New().String()
	r.Dispatch(reading(dev, "a", 1))
	r.Dispatch(reading(dev, "b", 2)) // dropped, not blocked

	assert.Len(t, ch, 1)
}

func TestRouter_MultipleFlows(t *testing.T) {
	r := NewRouter()
	f1, f2 := uuid.New(), uuid.New()
	ch1 := r.Subscribe(f1, Filter{}, 4)
	ch2 := r.Subscribe(f2, Filter{}, 4)
	r.Dispatch(reading(uuid.New().String(), "t", 1))
	assert.Len(t, ch1, 1)
	assert.Len(t, ch2, 1)
}
```

- [ ] **Step 2: Implement** (`router.go`)

```go
package ingest

import (
	"sync"

	"github.com/google/uuid"
	"github.com/sankyago/observer/internal/model"
)

// Filter selects readings for a subscription. Empty fields mean "match any".
type Filter struct {
	DeviceID string
	Metric   string
}

func (f Filter) matches(r model.SensorReading) bool {
	if f.DeviceID != "" && f.DeviceID != r.DeviceID {
		return false
	}
	if f.Metric != "" && f.Metric != r.Metric {
		return false
	}
	return true
}

type subscription struct {
	filter Filter
	ch     chan model.SensorReading
}

type Router struct {
	mu   sync.RWMutex
	subs map[uuid.UUID][]*subscription // keyed by flow ID
}

func NewRouter() *Router {
	return &Router{subs: map[uuid.UUID][]*subscription{}}
}

func (r *Router) Subscribe(flowID uuid.UUID, f Filter, buffer int) chan model.SensorReading {
	ch := make(chan model.SensorReading, buffer)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.subs[flowID] = append(r.subs[flowID], &subscription{filter: f, ch: ch})
	return ch
}

func (r *Router) Unsubscribe(flowID uuid.UUID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.subs[flowID] {
		close(s.ch)
	}
	delete(r.subs, flowID)
}

func (r *Router) Dispatch(reading model.SensorReading) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, subs := range r.subs {
		for _, s := range subs {
			if !s.filter.matches(reading) {
				continue
			}
			select {
			case s.ch <- reading:
			default:
				// drop on full
			}
		}
	}
}
```

- [ ] **Step 3: Run**

```bash
go test ./internal/ingest/... -v -race
```
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/ingest/router.go internal/ingest/router_test.go
git commit -m "feat: add ingest router with filtered fan-out"
```

---

## Task 8: Ingest consumer

**Files:**
- Create: `internal/ingest/consumer.go`, `internal/ingest/consumer_test.go`

- [ ] **Step 1: Write failing unit tests** (pure logic only — no real MQTT)

```go
package ingest

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestConsumer_HandleMessage_Dispatches(t *testing.T) {
	router := NewRouter()
	flow := uuid.New()
	ch := router.Subscribe(flow, Filter{}, 4)

	c := &Consumer{router: router, now: time.Now}
	dev := uuid.New().String()
	c.handle("v1/devices/"+dev+"/telemetry", []byte(`{"t":1.0}`))

	select {
	case r := <-ch:
		assert.Equal(t, dev, r.DeviceID)
	default:
		t.Fatal("no reading dispatched")
	}
}

func TestConsumer_HandleMessage_BadTopic_DoesNotPanic(t *testing.T) {
	router := NewRouter()
	c := &Consumer{router: router, now: time.Now}
	assert.NotPanics(t, func() { c.handle("garbage", []byte(`{}`)) })
}

func TestConsumer_HandleMessage_BadJSON_DoesNotPanic(t *testing.T) {
	router := NewRouter()
	c := &Consumer{router: router, now: time.Now}
	dev := uuid.New().String()
	assert.NotPanics(t, func() { c.handle("v1/devices/"+dev+"/telemetry", []byte(`{`)) })
}
```

- [ ] **Step 2: Implement** (`consumer.go`)

```go
package ingest

import (
	"context"
	"fmt"
	"log"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type Config struct {
	BrokerURL   string
	Username    string
	Password    string
	SharedGroup string // "observer"
	ClientID    string // distinguishes this replica
}

type Consumer struct {
	cfg    Config
	router *Router
	client mqtt.Client
	now    func() time.Time
}

func NewConsumer(cfg Config, router *Router) *Consumer {
	return &Consumer{cfg: cfg, router: router, now: time.Now}
}

func (c *Consumer) Run(ctx context.Context) error {
	opts := mqtt.NewClientOptions().
		AddBroker(c.cfg.BrokerURL).
		SetClientID(c.cfg.ClientID).
		SetUsername(c.cfg.Username).
		SetPassword(c.cfg.Password).
		SetAutoReconnect(true).
		SetOnConnectHandler(func(cli mqtt.Client) {
			topic := fmt.Sprintf("$share/%s/v1/devices/+/telemetry", c.cfg.SharedGroup)
			if tok := cli.Subscribe(topic, 0, func(_ mqtt.Client, m mqtt.Message) {
				c.handle(m.Topic(), m.Payload())
			}); tok.Wait() && tok.Error() != nil {
				log.Printf("ingest subscribe: %v", tok.Error())
			}
		})
	c.client = mqtt.NewClient(opts)
	if tok := c.client.Connect(); tok.Wait() && tok.Error() != nil {
		return tok.Error()
	}
	<-ctx.Done()
	c.client.Disconnect(250)
	return nil
}

func (c *Consumer) handle(topic string, payload []byte) {
	readings, err := Parse(topic, payload, c.now)
	if err != nil {
		log.Printf("ingest parse: %v", err)
		return
	}
	for _, r := range readings {
		c.router.Dispatch(r)
	}
}
```

- [ ] **Step 3: Run**

```bash
go test ./internal/ingest/... -v -race
```
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/ingest/consumer.go internal/ingest/consumer_test.go
git commit -m "feat: add ingest consumer with shared subscription"
```

---

## Task 9: `device_source` node

**Files:**
- Create: `internal/flow/nodes/device_source.go`, `internal/flow/nodes/device_source_test.go`
- Delete: `internal/flow/nodes/mqtt_source.go`, `internal/flow/nodes/mqtt_source_test.go`, `internal/flow/nodes/mqtt_source_integration_test.go`

- [ ] **Step 1: Write failing tests** (`device_source_test.go`)

```go
package nodes

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sankyago/observer/internal/ingest"
	"github.com/sankyago/observer/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeviceSource_EmitsSubscribedReadings(t *testing.T) {
	router := ingest.NewRouter()
	flow := uuid.New()
	dev := uuid.New().String()

	n, err := NewDeviceSource("ds", flow, router, json.RawMessage(`{"device_id":"`+dev+`"}`))
	require.NoError(t, err)

	out := make(chan model.SensorReading, 4)
	events := make(chan FlowEvent, 1)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_ = n.Run(ctx, nil, out, events)
		close(done)
	}()

	time.Sleep(20 * time.Millisecond) // let Subscribe register
	router.Dispatch(model.SensorReading{DeviceID: dev, Metric: "temp", Value: 1, Timestamp: time.Now()})
	router.Dispatch(model.SensorReading{DeviceID: uuid.New().String(), Metric: "temp", Value: 2, Timestamp: time.Now()})

	select {
	case r := <-out:
		assert.Equal(t, dev, r.DeviceID)
	case <-time.After(time.Second):
		t.Fatal("no reading emitted")
	}

	cancel()
	<-done
	// out should be closed
	_, ok := <-out
	// either drained empty or closed; both acceptable — just verify no panic
	_ = ok
}

func TestDeviceSource_InvalidJSON(t *testing.T) {
	_, err := NewDeviceSource("ds", uuid.New(), ingest.NewRouter(), json.RawMessage(`not-json`))
	assert.Error(t, err)
}

func TestDeviceSource_EmptyConfig_SubscribesToAll(t *testing.T) {
	router := ingest.NewRouter()
	flow := uuid.New()
	n, err := NewDeviceSource("ds", flow, router, json.RawMessage(`{}`))
	require.NoError(t, err)

	out := make(chan model.SensorReading, 2)
	events := make(chan FlowEvent, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go n.Run(ctx, nil, out, events)

	time.Sleep(20 * time.Millisecond)
	router.Dispatch(model.SensorReading{DeviceID: uuid.New().String(), Metric: "a", Value: 1, Timestamp: time.Now()})
	router.Dispatch(model.SensorReading{DeviceID: uuid.New().String(), Metric: "b", Value: 2, Timestamp: time.Now()})

	var count int
	for i := 0; i < 2; i++ {
		select {
		case <-out:
			count++
		case <-time.After(500 * time.Millisecond):
		}
	}
	assert.Equal(t, 2, count)
}
```

- [ ] **Step 2: Implement** (`device_source.go`)

```go
package nodes

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/sankyago/observer/internal/ingest"
	"github.com/sankyago/observer/internal/model"
)

type DeviceSource struct {
	id     string
	flowID uuid.UUID
	router *ingest.Router
	filter ingest.Filter
}

type deviceSourceCfg struct {
	DeviceID string `json:"device_id"`
	Metric   string `json:"metric"`
}

func NewDeviceSource(id string, flowID uuid.UUID, router *ingest.Router, data json.RawMessage) (*DeviceSource, error) {
	var cfg deviceSourceCfg
	if len(data) > 0 {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("device_source %s: %w", id, err)
		}
	}
	return &DeviceSource{
		id:     id,
		flowID: flowID,
		router: router,
		filter: ingest.Filter{DeviceID: cfg.DeviceID, Metric: cfg.Metric},
	}, nil
}

func (d *DeviceSource) ID() string { return d.id }

func (d *DeviceSource) Run(ctx context.Context, _ <-chan model.SensorReading, out chan<- model.SensorReading, _ chan<- FlowEvent) error {
	defer close(out)
	sub := d.router.Subscribe(d.flowID, d.filter, 64)
	defer d.router.Unsubscribe(d.flowID)
	for {
		select {
		case <-ctx.Done():
			return nil
		case r, ok := <-sub:
			if !ok {
				return nil
			}
			select {
			case out <- r:
			case <-ctx.Done():
				return nil
			}
		}
	}
}
```

- [ ] **Step 3: Delete old mqtt_source files**

```bash
git rm internal/flow/nodes/mqtt_source.go internal/flow/nodes/mqtt_source_test.go internal/flow/nodes/mqtt_source_integration_test.go
```

- [ ] **Step 4: Update `internal/flow/graph/validate.go`** — replace `mqtt_source` entry in `KnownTypes` with `device_source`:

```go
var KnownTypes = map[string]func(json.RawMessage) error{
    "device_source":  validateDeviceSource,
    "threshold":      validateThreshold,
    "rate_of_change": validateRateOfChange,
    "debug_sink":     validateDebugSink,
}

// Replace validateMQTTSource with:
type deviceSourceCfg struct {
    DeviceID string `json:"device_id"`
    Metric   string `json:"metric"`
}

func validateDeviceSource(raw json.RawMessage) error {
    if len(raw) == 0 {
        return nil
    }
    var c deviceSourceCfg
    if err := json.Unmarshal(raw, &c); err != nil {
        return fmt.Errorf("%w: device_source data: %v", ErrValidation, err)
    }
    if c.DeviceID != "" {
        if _, err := uuid.Parse(c.DeviceID); err != nil {
            return fmt.Errorf("%w: device_source: device_id must be uuid", ErrValidation)
        }
    }
    return nil
}
```

Add `"github.com/google/uuid"` import. Update `validate_test.go` — replace any `mqtt_source` fixture with `device_source` (use a real UUID string).

- [ ] **Step 5: Run**

```bash
go test ./internal/flow/... -v -race
```
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add -A internal/flow/ internal/subscriber/
git commit -m "feat: replace mqtt_source with device_source node"
```

Note — at this point `internal/subscriber/` is no longer imported by anything. Task 10 removes it.

---

## Task 10: Wire the router through the runtime

**Files:**
- Modify: `internal/flow/runtime/compiled_flow.go`, `internal/flow/runtime/manager.go`, `internal/flow/service.go`

- [ ] **Step 1: Modify `compiled_flow.go`**

Add a `*ingest.Router` parameter to `Compile`/`CompileWithSinkWriter`. Pass the flow ID and router into `buildNode` for `device_source`:

```go
func Compile(flowID uuid.UUID, g graph.Graph, router *ingest.Router) (*CompiledFlow, error) {
    return CompileWithSinkWriter(flowID, g, router, os.Stderr)
}

func CompileWithSinkWriter(flowID uuid.UUID, g graph.Graph, router *ingest.Router, sinkOut io.Writer) (*CompiledFlow, error) {
    // ...
    for _, n := range g.Nodes {
        inst, err := buildNode(n, flowID, router, sinkOut)
        // ...
    }
    // ...
}

func buildNode(n graph.Node, flowID uuid.UUID, router *ingest.Router, sinkOut io.Writer) (nodes.Node, error) {
    switch n.Type {
    case "device_source":
        return nodes.NewDeviceSource(n.ID, flowID, router, n.Data)
    case "threshold":
        return nodes.NewThreshold(n.ID, n.Data)
    case "rate_of_change":
        return nodes.NewRateOfChange(n.ID, n.Data)
    case "debug_sink":
        return nodes.NewDebugSink(n.ID, sinkOut), nil
    default:
        return nil, fmt.Errorf("unknown node type %q", n.Type)
    }
}
```

- [ ] **Step 2: Modify `manager.go`**

```go
type Manager struct {
    mu     sync.Mutex
    flows  map[uuid.UUID]*CompiledFlow
    router *ingest.Router
}

func NewManager(router *ingest.Router) *Manager {
    return &Manager{flows: map[uuid.UUID]*CompiledFlow{}, router: router}
}

func (m *Manager) Start(ctx context.Context, id uuid.UUID, g graph.Graph) error {
    cf, err := Compile(id, g, m.router)
    // ...
}
```

Delete the old `Replace` if it's just `Start`; adjust signature of `Replace` accordingly.

- [ ] **Step 3: Modify `internal/flow/service.go`**

Replace `NewService(ctx, repo, mgr)` call sites in tests and main to pass a `Manager` constructed with a router. Service itself does not need the router — only the manager does.

- [ ] **Step 4: Update existing tests**

`runtime/compiled_flow_test.go`, `runtime/manager_test.go`, `internal/flow/service_test.go`: every `Compile(...)` / `NewManager(...)` call now passes an `ingest.NewRouter()` (or a stubbed router).

Compile-time fix + targeted test updates. Any test using the removed `mqtt_source` node type is replaced with `device_source`.

- [ ] **Step 5: Run**

```bash
go test ./... -race
```
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add -A internal/flow/
git commit -m "feat: wire ingest router through runtime and manager"
```

---

## Task 11: Remove `internal/subscriber/`

**Files:**
- Delete: all of `internal/subscriber/`

- [ ] **Step 1: Verify no imports**

```bash
grep -r "internal/subscriber" . --include="*.go"
```
Expected: no matches.

- [ ] **Step 2: Delete**

```bash
git rm -r internal/subscriber/
```

- [ ] **Step 3: Run**

```bash
go test ./... -race
go build ./...
```
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git commit -m "refactor: drop internal/subscriber (functionality moved to ingest)"
```

---

## Task 12: docker-compose + main wiring

**Files:**
- Modify: `docker-compose.yml`, `cmd/observer/main.go`

- [ ] **Step 1: Add EMQX service to `docker-compose.yml`**

```yaml
  emqx:
    image: emqx/emqx:5.7
    hostname: emqx
    environment:
      EMQX_NAME: observer-emqx
      EMQX_ALLOW_ANONYMOUS: "false"
      EMQX_AUTHENTICATION__1__MECHANISM: "password_based"
      EMQX_AUTHENTICATION__1__BACKEND: "http"
      EMQX_AUTHENTICATION__1__METHOD: "post"
      EMQX_AUTHENTICATION__1__URL: "http://app:8080/api/mqtt/auth"
      EMQX_AUTHENTICATION__1__HEADERS__CONTENT-TYPE: "application/json"
      EMQX_AUTHENTICATION__1__HEADERS__X-EMQX-SECRET: "dev-secret"
      EMQX_AUTHENTICATION__1__BODY__USERNAME: "${username}"
      EMQX_AUTHENTICATION__1__BODY__PASSWORD: "${password}"
      EMQX_AUTHENTICATION__1__BODY__CLIENTID: "${clientid}"
      EMQX_AUTHORIZATION__SOURCES__1__TYPE: "http"
      EMQX_AUTHORIZATION__SOURCES__1__METHOD: "post"
      EMQX_AUTHORIZATION__SOURCES__1__URL: "http://app:8080/api/mqtt/acl"
      EMQX_AUTHORIZATION__SOURCES__1__HEADERS__CONTENT-TYPE: "application/json"
      EMQX_AUTHORIZATION__SOURCES__1__HEADERS__X-EMQX-SECRET: "dev-secret"
      EMQX_AUTHORIZATION__SOURCES__1__BODY__USERNAME: "${username}"
      EMQX_AUTHORIZATION__SOURCES__1__BODY__ACTION: "${action}"
      EMQX_AUTHORIZATION__SOURCES__1__BODY__TOPIC: "${topic}"
      EMQX_AUTHORIZATION__NO_MATCH: "deny"
    ports:
      - "1883:1883"
      - "8083:8083"
      - "18083:18083"
    depends_on:
      app:
        condition: service_started
```

Update the existing `app` service to set:
```yaml
    environment:
      EMQX_BROKER_URL: tcp://emqx:1883
      EMQX_USERNAME: observer-consumer
      EMQX_PASSWORD: observer-consumer-pw
      EMQX_SHARED_GROUP: observer
      MQTT_WEBHOOK_SECRET: dev-secret
```

Add a line in `emqx` env to allow the `observer-consumer` service account (use EMQX's built-in user table or a file-based authenticator alongside the HTTP one). The simplest route: add a second authenticator:

```yaml
      EMQX_AUTHENTICATION__2__MECHANISM: "password_based"
      EMQX_AUTHENTICATION__2__BACKEND: "built_in_database"
```

And preseed the built-in DB via `emqx ctl users add observer-consumer observer-consumer-pw` in an init script — OR the HTTP auth handler returns allow for `username == EMQX_USERNAME` (already done in Task 5). Keep it one authenticator: the HTTP one. Service account flows through the same `/api/mqtt/auth` path.

- [ ] **Step 2: Rewrite `cmd/observer/main.go`**

```go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sankyago/observer/internal/api"
	"github.com/sankyago/observer/internal/db"
	"github.com/sankyago/observer/internal/devices"
	devstore "github.com/sankyago/observer/internal/devices/store"
	"github.com/sankyago/observer/internal/flow"
	"github.com/sankyago/observer/internal/flow/runtime"
	flowstore "github.com/sankyago/observer/internal/flow/store"
	"github.com/sankyago/observer/internal/ingest"
)

func main() {
	dbURL := env("DATABASE_URL", "postgres://observer:observer@localhost:5432/observer")
	addr := env("HTTP_ADDR", ":8080")
	migDir := env("MIGRATIONS_DIR", "migrations")
	brokerURL := env("EMQX_BROKER_URL", "tcp://localhost:1883")
	mqUser := env("EMQX_USERNAME", "observer-consumer")
	mqPass := env("EMQX_PASSWORD", "")
	mqGroup := env("EMQX_SHARED_GROUP", "observer")
	secret := env("MQTT_WEBHOOK_SECRET", "")

	if mqPass == "" || secret == "" {
		log.Fatal("EMQX_PASSWORD and MQTT_WEBHOOK_SECRET are required")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("pg: %v", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool, migDir); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	devSvc := devices.NewService(devstore.NewRepo(pool))
	router := ingest.NewRouter()
	mgr := runtime.NewManager(router)
	flowSvc := flow.NewService(ctx, flowstore.NewRepo(pool), mgr)

	if err := flowSvc.LoadEnabled(ctx); err != nil {
		log.Fatalf("load flows: %v", err)
	}

	consumer := ingest.NewConsumer(ingest.Config{
		BrokerURL:   brokerURL,
		Username:    mqUser,
		Password:    mqPass,
		SharedGroup: mqGroup,
		ClientID:    fmt.Sprintf("observer-%d", time.Now().UnixNano()),
	}, router)

	go func() {
		if err := consumer.Run(ctx); err != nil {
			log.Printf("consumer exited: %v", err)
		}
	}()

	httpHandler := api.NewRouter(flowSvc, devSvc, api.WithMQTTAuth(secret, mqUser))
	srv := &http.Server{Addr: addr, Handler: httpHandler, ReadTimeout: 10 * time.Second}

	go func() {
		log.Printf("listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	shutdownCtx, sc := context.WithTimeout(context.Background(), 5*time.Second)
	defer sc()
	_ = srv.Shutdown(shutdownCtx)
	mgr.StopAll()
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
```

- [ ] **Step 3: Run**

```bash
go build ./...
```
Expected: binary compiles.

- [ ] **Step 4: Commit**

```bash
git add docker-compose.yml cmd/observer/main.go
git commit -m "feat: wire EMQX, ingest consumer, and devices into main"
```

---

## Task 13: End-to-end integration test

**Files:**
- Delete: `test/flow_e2e_test.go` (replaced)
- Create: `test/emqx_e2e_test.go`

- [ ] **Step 1: Remove old e2e**

```bash
git rm test/flow_e2e_test.go
```

- [ ] **Step 2: Write new e2e** (`test/emqx_e2e_test.go`)

```go
//go:build integration

package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sankyago/observer/internal/api"
	"github.com/sankyago/observer/internal/db"
	"github.com/sankyago/observer/internal/devices"
	devstore "github.com/sankyago/observer/internal/devices/store"
	"github.com/sankyago/observer/internal/flow"
	flowgraph "github.com/sankyago/observer/internal/flow/graph"
	flowruntime "github.com/sankyago/observer/internal/flow/runtime"
	flowstore "github.com/sankyago/observer/internal/flow/store"
	"github.com/sankyago/observer/internal/ingest"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	tcwait "github.com/testcontainers/testcontainers-go/wait"
)

func TestE2E_EMQX_To_WS(t *testing.T) {
	ctx := context.Background()
	const secret = "test-secret"
	const serviceUser = "observer-consumer"
	const servicePass = "observer-consumer-pw"

	// Postgres
	pg, err := postgres.Run(ctx, "timescale/timescaledb:latest-pg16",
		postgres.WithDatabase("test"), postgres.WithUsername("test"), postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pg.Terminate(ctx) })
	dsn, _ := pg.ConnectionString(ctx, "sslmode=disable")
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, db.Migrate(ctx, pool, "../migrations"))

	// Wire Observer
	devSvc := devices.NewService(devstore.NewRepo(pool))
	router := ingest.NewRouter()
	mgr := flowruntime.NewManager(router)
	flowSvc := flow.NewService(ctx, flowstore.NewRepo(pool), mgr)
	httpHandler := api.NewRouter(flowSvc, devSvc, api.WithMQTTAuth(secret, serviceUser))
	srv := httptest.NewServer(httpHandler)
	t.Cleanup(srv.Close)
	t.Cleanup(mgr.StopAll)

	// EMQX (testcontainer)
	hostForEMQX := dockerHostForContainer(srv.URL) // maps host.docker.internal
	authURL := hostForEMQX + "/api/mqtt/auth"
	aclURL := hostForEMQX + "/api/mqtt/acl"

	req := testcontainers.ContainerRequest{
		Image:        "emqx/emqx:5.7",
		ExposedPorts: []string{"1883/tcp"},
		Env: map[string]string{
			"EMQX_ALLOW_ANONYMOUS":                            "false",
			"EMQX_AUTHENTICATION__1__MECHANISM":               "password_based",
			"EMQX_AUTHENTICATION__1__BACKEND":                 "http",
			"EMQX_AUTHENTICATION__1__METHOD":                  "post",
			"EMQX_AUTHENTICATION__1__URL":                     authURL,
			"EMQX_AUTHENTICATION__1__HEADERS__CONTENT-TYPE":   "application/json",
			"EMQX_AUTHENTICATION__1__HEADERS__X-EMQX-SECRET":  secret,
			"EMQX_AUTHENTICATION__1__BODY__USERNAME":          "${username}",
			"EMQX_AUTHENTICATION__1__BODY__PASSWORD":          "${password}",
			"EMQX_AUTHENTICATION__1__BODY__CLIENTID":          "${clientid}",
			"EMQX_AUTHORIZATION__SOURCES__1__TYPE":            "http",
			"EMQX_AUTHORIZATION__SOURCES__1__METHOD":          "post",
			"EMQX_AUTHORIZATION__SOURCES__1__URL":             aclURL,
			"EMQX_AUTHORIZATION__SOURCES__1__HEADERS__CONTENT-TYPE":  "application/json",
			"EMQX_AUTHORIZATION__SOURCES__1__HEADERS__X-EMQX-SECRET": secret,
			"EMQX_AUTHORIZATION__SOURCES__1__BODY__USERNAME":  "${username}",
			"EMQX_AUTHORIZATION__SOURCES__1__BODY__ACTION":    "${action}",
			"EMQX_AUTHORIZATION__SOURCES__1__BODY__TOPIC":     "${topic}",
			"EMQX_AUTHORIZATION__NO_MATCH":                    "deny",
		},
		WaitingFor: tcwait.ForLog("EMQX 5.7").WithStartupTimeout(60 * time.Second),
	}
	ec, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: req, Started: true})
	require.NoError(t, err)
	t.Cleanup(func() { _ = ec.Terminate(ctx) })
	ehost, _ := ec.Host(ctx)
	eport, _ := ec.MappedPort(ctx, "1883")
	brokerURL := fmt.Sprintf("tcp://%s:%s", ehost, eport.Port())

	// Start Observer's ingest consumer
	consumer := ingest.NewConsumer(ingest.Config{
		BrokerURL: brokerURL, Username: serviceUser, Password: servicePass,
		SharedGroup: "observer", ClientID: "observer-test",
	}, router)
	runCtx, runCancel := context.WithCancel(ctx)
	t.Cleanup(runCancel)
	go consumer.Run(runCtx)
	time.Sleep(500 * time.Millisecond) // let the subscription land

	// Create a device via API
	body, _ := json.Marshal(map[string]string{"name": "test-device"})
	resp, err := http.Post(srv.URL+"/api/devices", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var created struct {
		ID    string `json:"id"`
		Token string `json:"token"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))
	resp.Body.Close()

	// Create a flow: device_source(device_id=created.ID) → threshold(0..10) → debug_sink, enabled
	g := flowgraph.Graph{
		Nodes: []flowgraph.Node{
			{ID: "src", Type: "device_source", Data: json.RawMessage(`{"device_id":"` + created.ID + `"}`)},
			{ID: "th", Type: "threshold", Data: json.RawMessage(`{"min":0,"max":10}`)},
			{ID: "sink", Type: "debug_sink", Data: json.RawMessage(`{}`)},
		},
		Edges: []flowgraph.Edge{
			{ID: "e1", Source: "src", Target: "th"},
			{ID: "e2", Source: "th", Target: "sink"},
		},
	}
	fbody, _ := json.Marshal(map[string]any{"name": "demo", "graph": g, "enabled": true})
	resp, err = http.Post(srv.URL+"/api/flows", "application/json", bytes.NewReader(fbody))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var flowCreated struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&flowCreated))
	resp.Body.Close()
	time.Sleep(300 * time.Millisecond)

	// Open WS
	wsURL := "ws" + srv.URL[len("http"):] + "/api/flows/" + flowCreated.ID + "/events"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	// Device publishes out-of-range value
	pub := mqtt.NewClient(mqtt.NewClientOptions().
		AddBroker(brokerURL).
		SetClientID("device-test").
		SetUsername(created.Token))
	tok := pub.Connect()
	require.True(t, tok.WaitTimeout(5*time.Second))
	require.NoError(t, tok.Error())
	ptok := pub.Publish("v1/devices/"+created.ID+"/telemetry", 0, false, `{"temperature":99.9,"ts":"2026-04-12T10:00:00Z"}`)
	require.True(t, ptok.WaitTimeout(5*time.Second))
	pub.Disconnect(100)

	// Expect alert event on WS
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var gotAlert bool
	for !gotAlert {
		var e map[string]any
		if err := conn.ReadJSON(&e); err != nil {
			break
		}
		if e["kind"] == "alert" {
			gotAlert = true
		}
	}
	require.True(t, gotAlert, "expected alert over WS")
}

// dockerHostForContainer rewrites a localhost URL so a docker container can reach it.
func dockerHostForContainer(u string) string {
	// testcontainers on macOS/Windows: use host.docker.internal
	// on Linux it may differ; test environment dependent
	return "http://host.docker.internal:" + portFromURL(u)
}

func portFromURL(u string) string {
	// crude parse: http://127.0.0.1:NNNN
	for i := len(u) - 1; i >= 0; i-- {
		if u[i] == ':' {
			return u[i+1:]
		}
	}
	return ""
}
```

NOTE: on Linux `host.docker.internal` may not resolve. The implementer should either:
- add `--add-host=host.docker.internal:host-gateway` via `testcontainers.ContainerRequest.HostConfigModifier`, or
- skip this e2e on Linux CI by gating behind `runtime.GOOS == "darwin"`.

Include the host-gateway fix in the container request.

- [ ] **Step 3: Run**

```bash
go test -tags integration ./test/... -timeout 300s -v
```
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add test/
git commit -m "test: e2e device publish → EMQX → flow → WS alert"
```

---

## Task 14: README

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Add a "Devices" section** after the API table:

```markdown
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

| Method | Path                                | Purpose                 |
|--------|-------------------------------------|-------------------------|
| GET    | /api/devices                        | list                    |
| POST   | /api/devices                        | create, returns token   |
| GET    | /api/devices/:id                    | get                     |
| PUT    | /api/devices/:id                    | rename                  |
| DELETE | /api/devices/:id                    | delete                  |
| POST   | /api/devices/:id/regenerate-token   | rotate token            |
```

Replace the node-types section's `mqtt_source` entry with:

```markdown
- `device_source` — `{device_id?, metric?}` (empty = all devices / all metrics)
```

Update the "Configuration" table:

```markdown
| Variable              | Default                           |
|-----------------------|-----------------------------------|
| `HTTP_ADDR`           | `:8080`                           |
| `DATABASE_URL`        | ...                               |
| `EMQX_BROKER_URL`     | `tcp://emqx:1883`                 |
| `EMQX_USERNAME`       | `observer-consumer`               |
| `EMQX_PASSWORD`       | (required)                        |
| `EMQX_SHARED_GROUP`   | `observer`                        |
| `MQTT_WEBHOOK_SECRET` | (required)                        |
```

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: document devices API and EMQX configuration"
```

---

## Task 15: Push + PR

- [ ] **Step 1:**

```bash
git push -u origin feat/emqx-ingest
```

- [ ] **Step 2:**

```bash
gh pr create --base feat/flow-engine \
  --title "feat: EMQX-fronted ingest with devices" \
  --body "$(cat <<'EOF'
> Stacks on feat/flow-engine.

## Summary
- EMQX runs as the canonical MQTT entrypoint for devices.
- Per-device opaque tokens; CONNECT authorizes via `/api/mqtt/auth`, PUBLISH via `/api/mqtt/acl`.
- New `device_source` flow node (replaces `mqtt_source`) with `{device_id?, metric?}` selector.
- `internal/ingest/` package: consumer, parser, router with filtered fan-out.
- `/api/devices` CRUD + token regeneration.
- Old `internal/subscriber/` removed (parsing moved to `ingest`).

## Test plan
- [ ] `go test ./... -race`
- [ ] `go test -tags integration ./... -timeout 300s`
- [ ] Manual: `docker compose up --build`, `curl POST /api/devices`, `mosquitto_pub -u <token> -t v1/devices/<id>/telemetry ...`, observe WS alert.
EOF
)"
```

---

## Self-Review

- **Spec coverage:** migration (T1), repo (T2), service (T3), CRUD (T4), auth+ACL (T5), parser (T6), router (T7), consumer (T8), device_source node + mqtt_source removal (T9), runtime wiring (T10), subscriber cleanup (T11), compose+main (T12), e2e (T13), docs (T14), PR (T15). All spec sections addressed.
- **Placeholder scan:** none — every step has concrete code. Task 5's test section leans on `fakeDeviceRepo` from Task 4's test file (same package); this is explicit and correct, not a placeholder.
- **Type consistency:** `store.Device`, `devices.Service`, `devices.repo` interface, `ingest.Filter`, `ingest.Router`, `ingest.Consumer`, `nodes.DeviceSource`, `runtime.Manager(router *ingest.Router)` consistent across tasks. `graph.KnownTypes` key `device_source` referenced identically in T5, T9, T10.
- **Reliability bar:** every new package has unit tests before its commit. Integration tests gated by `//go:build integration`.
- **Known risks carried from spec:** EMQX 5.x env-var syntax is the single "verify first" unknown. Task 12 pins to the documented 5.7 shape; Task 13 validates the same shape inside a testcontainer.

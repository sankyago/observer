package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/observer-io/observer/internal/testutil"
	"github.com/observer-io/observer/pkg/events"
)

func startAPI(t *testing.T) (http.Handler, *pgxpool.Pool, *events.Bus) {
	t.Helper()
	dsn := testutil.StartTimescale(t)
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)

	ctx := context.Background()
	stmts := []string{
		`CREATE EXTENSION IF NOT EXISTS pgcrypto`,
		`CREATE EXTENSION IF NOT EXISTS timescaledb`,
		`CREATE TABLE tenants (id UUID PRIMARY KEY, slug TEXT UNIQUE, name TEXT, created_at TIMESTAMPTZ DEFAULT now())`,
		`CREATE TABLE devices (id UUID PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE, name TEXT NOT NULL, type TEXT NOT NULL DEFAULT '', created_at TIMESTAMPTZ DEFAULT now())`,
		`CREATE TABLE actions (id UUID PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE, kind TEXT NOT NULL, config JSONB NOT NULL DEFAULT '{}', created_at TIMESTAMPTZ DEFAULT now())`,
		`CREATE TABLE rules (id UUID PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE, device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE, field TEXT NOT NULL, op TEXT NOT NULL, value DOUBLE PRECISION NOT NULL, action_id UUID NOT NULL REFERENCES actions(id) ON DELETE RESTRICT, enabled BOOLEAN NOT NULL DEFAULT TRUE, debug_until TIMESTAMPTZ, created_at TIMESTAMPTZ DEFAULT now())`,
		`CREATE TABLE telemetry_raw (time TIMESTAMPTZ NOT NULL, tenant_id UUID NOT NULL, device_id UUID NOT NULL, message_id UUID NOT NULL, payload JSONB NOT NULL)`,
		`SELECT create_hypertable('telemetry_raw','time')`,
		`CREATE TABLE fired_actions (id UUID PRIMARY KEY DEFAULT gen_random_uuid(), fired_at TIMESTAMPTZ DEFAULT now(), tenant_id UUID NOT NULL, device_id UUID NOT NULL, rule_id UUID NOT NULL, action_id UUID NOT NULL, message_id UUID NOT NULL, status TEXT NOT NULL, error TEXT, payload JSONB NOT NULL)`,
	}
	for _, s := range stmts {
		if _, err := pool.Exec(ctx, s); err != nil {
			t.Fatalf("schema: %v: %s", err, s)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tenants (id,slug,name) VALUES ($1,'dev','Dev')`, DevTenantID); err != nil {
		t.Fatalf("seed: %v", err)
	}

	bus := events.NewBus(16)
	return BuildRouter(Deps{Pool: pool, Bus: bus, Logger: slog.New(slog.NewJSONHandler(io.Discard, nil))}), pool, bus
}

func TestDeviceLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("container test")
	}
	h, _, _ := startAPI(t)

	body, _ := json.Marshal(map[string]string{"name": "boiler", "type": "sensor"})
	req := httptest.NewRequest("POST", "/api/v1/devices", bytes.NewReader(body))
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)
	if rw.Code != 201 {
		t.Fatalf("create: %d %s", rw.Code, rw.Body.String())
	}
	var dev struct {
		ID uuid.UUID `json:"id"`
	}
	_ = json.Unmarshal(rw.Body.Bytes(), &dev)

	rw = httptest.NewRecorder()
	h.ServeHTTP(rw, httptest.NewRequest("GET", "/api/v1/devices", nil))
	if rw.Code != 200 || !strings.Contains(rw.Body.String(), "boiler") {
		t.Fatalf("list: %d %s", rw.Code, rw.Body.String())
	}

	rw = httptest.NewRecorder()
	h.ServeHTTP(rw, httptest.NewRequest("DELETE", "/api/v1/devices/"+dev.ID.String(), nil))
	if rw.Code != 204 {
		t.Fatalf("delete: %d", rw.Code)
	}
}

func TestStreamSSE(t *testing.T) {
	if testing.Short() {
		t.Skip("container test")
	}
	h, _, bus := startAPI(t)
	srv := httptest.NewServer(h)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/api/v1/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()

	// drain the ": connected\n\n" preamble first
	preamble := make([]byte, 64)
	n, _ := resp.Body.Read(preamble)
	if !strings.Contains(string(preamble[:n]), "connected") {
		t.Fatalf("expected connected preamble, got: %q", string(preamble[:n]))
	}

	bus.Publish(events.Event{Type: "telemetry", Data: []byte(`{"x":1}`)})

	buf := make([]byte, 256)
	n, _ = resp.Body.Read(buf)
	got := string(buf[:n])
	if !strings.Contains(got, "event: telemetry") || !strings.Contains(got, `"x":1`) {
		t.Errorf("sse output: %q", got)
	}
}

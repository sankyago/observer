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
	if err != nil { t.Fatalf("pool: %v", err) }
	t.Cleanup(pool.Close)
	ctx := context.Background()
	for _, s := range []string{
		`CREATE EXTENSION IF NOT EXISTS pgcrypto`,
		`CREATE EXTENSION IF NOT EXISTS timescaledb`,
		`CREATE TABLE tenants (id UUID PRIMARY KEY, slug TEXT UNIQUE, name TEXT, created_at TIMESTAMPTZ DEFAULT now())`,
		`CREATE TABLE devices (id UUID PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id UUID REFERENCES tenants(id), name TEXT, type TEXT DEFAULT '', created_at TIMESTAMPTZ DEFAULT now())`,
		`CREATE TABLE flows (id UUID PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id UUID REFERENCES tenants(id), name TEXT, graph JSONB, enabled BOOLEAN DEFAULT TRUE, created_at TIMESTAMPTZ DEFAULT now(), updated_at TIMESTAMPTZ DEFAULT now())`,
		`CREATE TABLE telemetry_raw (time TIMESTAMPTZ NOT NULL, tenant_id UUID NOT NULL, device_id UUID NOT NULL, message_id UUID NOT NULL, payload JSONB NOT NULL)`,
		`SELECT create_hypertable('telemetry_raw','time')`,
	} {
		if _, err := pool.Exec(ctx, s); err != nil { t.Fatalf("schema: %v: %s", err, s) }
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tenants (id,slug,name) VALUES ($1,'dev','Dev')`, DevTenantID); err != nil { t.Fatalf("seed: %v", err) }
	bus := events.NewBus(16)
	return BuildRouter(Deps{Pool: pool, Bus: bus, Logger: slog.New(slog.NewJSONHandler(io.Discard, nil))}), pool, bus
}

func TestDeviceLifecycle(t *testing.T) {
	if testing.Short() { t.Skip("container") }
	h, _, _ := startAPI(t)
	body, _ := json.Marshal(map[string]string{"name":"boiler","type":"sensor"})
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, httptest.NewRequest("POST", "/api/v1/devices", bytes.NewReader(body)))
	if rw.Code != 201 { t.Fatalf("create: %d %s", rw.Code, rw.Body.String()) }
	var dev struct{ ID uuid.UUID `json:"id"` }
	_ = json.Unmarshal(rw.Body.Bytes(), &dev)
	rw = httptest.NewRecorder()
	h.ServeHTTP(rw, httptest.NewRequest("GET", "/api/v1/devices", nil))
	if rw.Code != 200 || !strings.Contains(rw.Body.String(), "boiler") { t.Fatalf("list: %d %s", rw.Code, rw.Body.String()) }
}

func TestStreamSSE(t *testing.T) {
	if testing.Short() { t.Skip("container") }
	h, _, bus := startAPI(t)
	srv := httptest.NewServer(h)
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/api/v1/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil { t.Fatal(err) }
	defer resp.Body.Close()

	buf := make([]byte, 128)
	_, _ = resp.Body.Read(buf) // preamble
	time.Sleep(50 * time.Millisecond)
	bus.Publish(events.Event{Type: "telemetry", Data: []byte(`{"x":1}`)})
	buf = make([]byte, 256)
	n, _ := resp.Body.Read(buf)
	if !strings.Contains(string(buf[:n]), "event: telemetry") { t.Errorf("sse: %q", string(buf[:n])) }
}

package repo

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/observer-io/observer/internal/testutil"
)

func newPool(t *testing.T) (*pgxpool.Pool, uuid.UUID) {
	t.Helper()
	dsn := testutil.StartTimescale(t)
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil { t.Fatalf("pool: %v", err) }
	t.Cleanup(pool.Close)
	ctx := context.Background()
	for _, s := range []string{
		`CREATE EXTENSION IF NOT EXISTS pgcrypto`,
		`CREATE TABLE tenants (id UUID PRIMARY KEY DEFAULT gen_random_uuid(), slug TEXT UNIQUE, name TEXT, created_at TIMESTAMPTZ DEFAULT now())`,
		`CREATE TABLE devices (id UUID PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id UUID REFERENCES tenants(id), name TEXT, type TEXT DEFAULT '', created_at TIMESTAMPTZ DEFAULT now())`,
		`CREATE TABLE flows (id UUID PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id UUID REFERENCES tenants(id), name TEXT, graph JSONB, enabled BOOLEAN DEFAULT TRUE, created_at TIMESTAMPTZ DEFAULT now(), updated_at TIMESTAMPTZ DEFAULT now())`,
	} {
		if _, err := pool.Exec(ctx, s); err != nil { t.Fatalf("schema: %v", err) }
	}
	var tid uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO tenants (slug,name) VALUES('t','T') RETURNING id`).Scan(&tid); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return pool, tid
}

func TestFlowsCRUD(t *testing.T) {
	if testing.Short() { t.Skip("container") }
	pool, tid := newPool(t)
	ctx := context.Background()

	f, err := CreateFlow(ctx, pool, tid, FlowInput{Name: "x", Graph: json.RawMessage(`{"nodes":[],"edges":[]}`), Enabled: true})
	if err != nil { t.Fatalf("create: %v", err) }

	got, err := GetFlow(ctx, pool, tid, f.ID)
	if err != nil || got.Name != "x" { t.Fatalf("get: %v %q", err, got.Name) }

	all, _ := ListFlows(ctx, pool, tid)
	if len(all) != 1 { t.Fatalf("list: %d", len(all)) }

	_, err = UpdateFlow(ctx, pool, tid, f.ID, FlowInput{Name: "y", Graph: json.RawMessage(`{"nodes":[],"edges":[]}`), Enabled: false})
	if err != nil { t.Fatalf("update: %v", err) }

	if err := DeleteFlow(ctx, pool, tid, f.ID); err != nil { t.Fatalf("delete: %v", err) }
	all, _ = ListFlows(ctx, pool, tid)
	if len(all) != 0 { t.Fatalf("after delete: %d", len(all)) }
}

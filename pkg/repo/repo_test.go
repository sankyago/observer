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
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)

	ctx := context.Background()
	stmts := []string{
		`CREATE EXTENSION IF NOT EXISTS timescaledb`,
		`CREATE EXTENSION IF NOT EXISTS pgcrypto`,
		`CREATE TABLE tenants (id UUID PRIMARY KEY DEFAULT gen_random_uuid(), slug TEXT UNIQUE NOT NULL, name TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now())`,
		`CREATE TABLE devices (id UUID PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE, name TEXT NOT NULL, type TEXT NOT NULL DEFAULT '', created_at TIMESTAMPTZ NOT NULL DEFAULT now())`,
		`CREATE TABLE actions (id UUID PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE, kind TEXT NOT NULL, config JSONB NOT NULL DEFAULT '{}', created_at TIMESTAMPTZ NOT NULL DEFAULT now())`,
		`CREATE TABLE rules (id UUID PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE, device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE, field TEXT NOT NULL, op TEXT NOT NULL, value DOUBLE PRECISION NOT NULL, action_id UUID NOT NULL REFERENCES actions(id) ON DELETE RESTRICT, enabled BOOLEAN NOT NULL DEFAULT TRUE, debug_until TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT now())`,
	}
	for _, s := range stmts {
		if _, err := pool.Exec(ctx, s); err != nil {
			t.Fatalf("schema: %v: %s", err, s)
		}
	}
	var tenantID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO tenants (slug, name) VALUES ('t','T') RETURNING id`).Scan(&tenantID); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return pool, tenantID
}

func TestDevicesActionsRulesRoundtrip(t *testing.T) {
	if testing.Short() {
		t.Skip("container test")
	}
	pool, tenantID := newPool(t)
	ctx := context.Background()

	dev, err := CreateDevice(ctx, pool, tenantID, "boiler", "sensor")
	if err != nil {
		t.Fatalf("CreateDevice: %v", err)
	}
	act, err := CreateAction(ctx, pool, tenantID, "log", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("CreateAction: %v", err)
	}
	rule, err := CreateRule(ctx, pool, tenantID, RuleInput{
		DeviceID: dev.ID, Field: "temperature", Op: ">", Value: 80, ActionID: act.ID, Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateRule: %v", err)
	}

	ds, _ := ListDevices(ctx, pool, tenantID)
	as, _ := ListActions(ctx, pool, tenantID)
	rs, _ := ListRules(ctx, pool, tenantID)
	if len(ds) != 1 || len(as) != 1 || len(rs) != 1 {
		t.Fatalf("lists: %d/%d/%d", len(ds), len(as), len(rs))
	}
	if rs[0].ID != rule.ID {
		t.Error("rule roundtrip")
	}

	if _, err := UpdateRule(ctx, pool, tenantID, rule.ID, RuleInput{
		DeviceID: dev.ID, Field: "temperature", Op: ">", Value: 90, ActionID: act.ID, Enabled: false,
	}); err != nil {
		t.Fatalf("UpdateRule: %v", err)
	}
	if err := DeleteRule(ctx, pool, tenantID, rule.ID); err != nil {
		t.Fatalf("DeleteRule: %v", err)
	}
	rs, _ = ListRules(ctx, pool, tenantID)
	if len(rs) != 0 {
		t.Errorf("after delete: got %d rules", len(rs))
	}
}

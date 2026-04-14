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
		`CREATE TABLE flow_executions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			executed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			tenant_id UUID NOT NULL,
			device_id UUID NOT NULL,
			flow_id UUID NOT NULL,
			node_id TEXT NOT NULL,
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

func TestInsertExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("container test")
	}
	pool := newPool(t)
	ctx := context.Background()

	tenantID := uuid.New()
	deviceID := uuid.New()
	flowID := uuid.New()
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

	if err := InsertExecution(ctx, pool, ExecutionRow{
		TenantID: tenantID, DeviceID: deviceID, FlowID: flowID, NodeID: "a1",
		MessageID: messageID, Status: "ok", Payload: payload,
	}); err != nil {
		t.Fatalf("InsertExecution: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM flow_executions WHERE message_id=$1`, messageID).Scan(&n); err != nil {
		t.Fatalf("query execution: %v", err)
	}
	if n != 1 {
		t.Fatalf("execution rows got %d want 1", n)
	}
}

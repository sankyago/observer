package repo

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Flow struct {
	ID        uuid.UUID       `json:"id"`
	TenantID  uuid.UUID       `json:"tenant_id"`
	Name      string          `json:"name"`
	Graph     json.RawMessage `json:"graph"`
	Enabled   bool            `json:"enabled"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type FlowInput struct {
	Name    string          `json:"name"`
	Graph   json.RawMessage `json:"graph"`
	Enabled bool            `json:"enabled"`
}

func ListFlows(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID) ([]Flow, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, tenant_id, name, graph, enabled, created_at, updated_at
		 FROM flows WHERE tenant_id=$1 ORDER BY created_at DESC`, tenantID)
	if err != nil { return nil, err }
	defer rows.Close()
	out := []Flow{}
	for rows.Next() {
		var f Flow
		if err := rows.Scan(&f.ID, &f.TenantID, &f.Name, &f.Graph, &f.Enabled, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func GetFlow(ctx context.Context, pool *pgxpool.Pool, tenantID, id uuid.UUID) (Flow, error) {
	var f Flow
	err := pool.QueryRow(ctx,
		`SELECT id, tenant_id, name, graph, enabled, created_at, updated_at
		 FROM flows WHERE tenant_id=$1 AND id=$2`, tenantID, id,
	).Scan(&f.ID, &f.TenantID, &f.Name, &f.Graph, &f.Enabled, &f.CreatedAt, &f.UpdatedAt)
	return f, err
}

func CreateFlow(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID, in FlowInput) (Flow, error) {
	if len(in.Graph) == 0 {
		in.Graph = json.RawMessage(`{"nodes":[],"edges":[]}`)
	}
	var f Flow
	err := pool.QueryRow(ctx,
		`INSERT INTO flows (tenant_id, name, graph, enabled) VALUES ($1,$2,$3,$4)
		 RETURNING id, tenant_id, name, graph, enabled, created_at, updated_at`,
		tenantID, in.Name, in.Graph, in.Enabled,
	).Scan(&f.ID, &f.TenantID, &f.Name, &f.Graph, &f.Enabled, &f.CreatedAt, &f.UpdatedAt)
	return f, err
}

func UpdateFlow(ctx context.Context, pool *pgxpool.Pool, tenantID, id uuid.UUID, in FlowInput) (Flow, error) {
	if len(in.Graph) == 0 {
		in.Graph = json.RawMessage(`{"nodes":[],"edges":[]}`)
	}
	var f Flow
	err := pool.QueryRow(ctx,
		`UPDATE flows SET name=$1, graph=$2, enabled=$3, updated_at=now()
		 WHERE tenant_id=$4 AND id=$5
		 RETURNING id, tenant_id, name, graph, enabled, created_at, updated_at`,
		in.Name, in.Graph, in.Enabled, tenantID, id,
	).Scan(&f.ID, &f.TenantID, &f.Name, &f.Graph, &f.Enabled, &f.CreatedAt, &f.UpdatedAt)
	return f, err
}

func DeleteFlow(ctx context.Context, pool *pgxpool.Pool, tenantID, id uuid.UUID) error {
	_, err := pool.Exec(ctx, `DELETE FROM flows WHERE tenant_id=$1 AND id=$2`, tenantID, id)
	return err
}

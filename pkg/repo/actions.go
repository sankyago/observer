package repo

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Action struct {
	ID        uuid.UUID       `json:"id"`
	TenantID  uuid.UUID       `json:"tenant_id"`
	Kind      string          `json:"kind"`
	Config    json.RawMessage `json:"config"`
	CreatedAt time.Time       `json:"created_at"`
}

func ListActions(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID) ([]Action, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, tenant_id, kind, config, created_at FROM actions
		 WHERE tenant_id=$1 ORDER BY created_at DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Action{}
	for rows.Next() {
		var a Action
		if err := rows.Scan(&a.ID, &a.TenantID, &a.Kind, &a.Config, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func CreateAction(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID, kind string, cfg json.RawMessage) (Action, error) {
	if len(cfg) == 0 {
		cfg = json.RawMessage(`{}`)
	}
	var a Action
	err := pool.QueryRow(ctx,
		`INSERT INTO actions (tenant_id, kind, config) VALUES ($1,$2,$3)
		 RETURNING id, tenant_id, kind, config, created_at`,
		tenantID, kind, cfg,
	).Scan(&a.ID, &a.TenantID, &a.Kind, &a.Config, &a.CreatedAt)
	return a, err
}

func UpdateAction(ctx context.Context, pool *pgxpool.Pool, tenantID, id uuid.UUID, kind string, cfg json.RawMessage) (Action, error) {
	if len(cfg) == 0 {
		cfg = json.RawMessage(`{}`)
	}
	var a Action
	err := pool.QueryRow(ctx,
		`UPDATE actions SET kind=$1, config=$2 WHERE tenant_id=$3 AND id=$4
		 RETURNING id, tenant_id, kind, config, created_at`,
		kind, cfg, tenantID, id,
	).Scan(&a.ID, &a.TenantID, &a.Kind, &a.Config, &a.CreatedAt)
	return a, err
}

func DeleteAction(ctx context.Context, pool *pgxpool.Pool, tenantID, id uuid.UUID) error {
	_, err := pool.Exec(ctx, `DELETE FROM actions WHERE tenant_id=$1 AND id=$2`, tenantID, id)
	return err
}

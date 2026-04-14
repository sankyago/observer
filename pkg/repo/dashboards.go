package repo

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Dashboard struct {
	ID        uuid.UUID       `json:"id"`
	TenantID  uuid.UUID       `json:"tenant_id"`
	Name      string          `json:"name"`
	Layout    json.RawMessage `json:"layout"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type DashboardInput struct {
	Name   string          `json:"name"`
	Layout json.RawMessage `json:"layout"`
}

func ListDashboards(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID) ([]Dashboard, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, tenant_id, name, layout, created_at, updated_at
		 FROM dashboards WHERE tenant_id=$1 ORDER BY created_at DESC`, tenantID)
	if err != nil { return nil, err }
	defer rows.Close()
	out := []Dashboard{}
	for rows.Next() {
		var d Dashboard
		if err := rows.Scan(&d.ID, &d.TenantID, &d.Name, &d.Layout, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func GetDashboard(ctx context.Context, pool *pgxpool.Pool, tenantID, id uuid.UUID) (Dashboard, error) {
	var d Dashboard
	err := pool.QueryRow(ctx,
		`SELECT id, tenant_id, name, layout, created_at, updated_at
		 FROM dashboards WHERE tenant_id=$1 AND id=$2`, tenantID, id,
	).Scan(&d.ID, &d.TenantID, &d.Name, &d.Layout, &d.CreatedAt, &d.UpdatedAt)
	return d, err
}

func CreateDashboard(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID, in DashboardInput) (Dashboard, error) {
	if len(in.Layout) == 0 {
		in.Layout = json.RawMessage(`{"widgets":[]}`)
	}
	var d Dashboard
	err := pool.QueryRow(ctx,
		`INSERT INTO dashboards (tenant_id, name, layout) VALUES ($1,$2,$3)
		 RETURNING id, tenant_id, name, layout, created_at, updated_at`,
		tenantID, in.Name, in.Layout,
	).Scan(&d.ID, &d.TenantID, &d.Name, &d.Layout, &d.CreatedAt, &d.UpdatedAt)
	return d, err
}

func UpdateDashboard(ctx context.Context, pool *pgxpool.Pool, tenantID, id uuid.UUID, in DashboardInput) (Dashboard, error) {
	if len(in.Layout) == 0 {
		in.Layout = json.RawMessage(`{"widgets":[]}`)
	}
	var d Dashboard
	err := pool.QueryRow(ctx,
		`UPDATE dashboards SET name=$1, layout=$2, updated_at=now()
		 WHERE tenant_id=$3 AND id=$4
		 RETURNING id, tenant_id, name, layout, created_at, updated_at`,
		in.Name, in.Layout, tenantID, id,
	).Scan(&d.ID, &d.TenantID, &d.Name, &d.Layout, &d.CreatedAt, &d.UpdatedAt)
	return d, err
}

func DeleteDashboard(ctx context.Context, pool *pgxpool.Pool, tenantID, id uuid.UUID) error {
	_, err := pool.Exec(ctx, `DELETE FROM dashboards WHERE tenant_id=$1 AND id=$2`, tenantID, id)
	return err
}

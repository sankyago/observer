package repo

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DeviceGroup struct {
	ID        uuid.UUID `json:"id"`
	TenantID  uuid.UUID `json:"tenant_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type DeviceGroupInput struct {
	Name string `json:"name"`
}

func ListGroups(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID) ([]DeviceGroup, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, tenant_id, name, created_at
		 FROM device_groups WHERE tenant_id=$1 ORDER BY created_at DESC`, tenantID)
	if err != nil { return nil, err }
	defer rows.Close()
	out := []DeviceGroup{}
	for rows.Next() {
		var g DeviceGroup
		if err := rows.Scan(&g.ID, &g.TenantID, &g.Name, &g.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func CreateGroup(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID, in DeviceGroupInput) (DeviceGroup, error) {
	var g DeviceGroup
	err := pool.QueryRow(ctx,
		`INSERT INTO device_groups (tenant_id, name) VALUES ($1,$2)
		 RETURNING id, tenant_id, name, created_at`,
		tenantID, in.Name,
	).Scan(&g.ID, &g.TenantID, &g.Name, &g.CreatedAt)
	return g, err
}

func DeleteGroup(ctx context.Context, pool *pgxpool.Pool, tenantID, id uuid.UUID) error {
	_, err := pool.Exec(ctx, `DELETE FROM device_groups WHERE tenant_id=$1 AND id=$2`, tenantID, id)
	return err
}

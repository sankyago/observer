// Package repo holds all SQL used by the HTTP API.
package repo

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Device struct {
	ID        uuid.UUID `json:"id"`
	TenantID  uuid.UUID `json:"tenant_id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
}

func ListDevices(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID) ([]Device, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, tenant_id, name, type, created_at FROM devices
		 WHERE tenant_id=$1 ORDER BY created_at DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Device{}
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.ID, &d.TenantID, &d.Name, &d.Type, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func CreateDevice(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID, name, typ string) (Device, error) {
	var d Device
	err := pool.QueryRow(ctx,
		`INSERT INTO devices (tenant_id, name, type) VALUES ($1,$2,$3)
		 RETURNING id, tenant_id, name, type, created_at`,
		tenantID, name, typ,
	).Scan(&d.ID, &d.TenantID, &d.Name, &d.Type, &d.CreatedAt)
	return d, err
}

func DeleteDevice(ctx context.Context, pool *pgxpool.Pool, tenantID, deviceID uuid.UUID) error {
	_, err := pool.Exec(ctx, `DELETE FROM devices WHERE tenant_id=$1 AND id=$2`, tenantID, deviceID)
	return err
}

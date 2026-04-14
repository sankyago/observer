// Package repo holds all SQL used by the HTTP API.
package repo

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Device struct {
	ID        uuid.UUID  `json:"id"`
	TenantID  uuid.UUID  `json:"tenant_id"`
	Name      string     `json:"name"`
	Type      string     `json:"type"`
	ProfileID *uuid.UUID `json:"profile_id"`
	GroupID   *uuid.UUID `json:"group_id"`
	CreatedAt time.Time  `json:"created_at"`
}

type DeviceInput struct {
	Name      string     `json:"name"`
	Type      string     `json:"type"`
	ProfileID *uuid.UUID `json:"profile_id"`
	GroupID   *uuid.UUID `json:"group_id"`
}

func ListDevices(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID) ([]Device, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, tenant_id, name, type, profile_id, group_id, created_at FROM devices
		 WHERE tenant_id=$1 ORDER BY created_at DESC`, tenantID)
	if err != nil { return nil, err }
	defer rows.Close()
	out := []Device{}
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.ID, &d.TenantID, &d.Name, &d.Type, &d.ProfileID, &d.GroupID, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func CreateDevice(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID, in DeviceInput) (Device, error) {
	var d Device
	err := pool.QueryRow(ctx,
		`INSERT INTO devices (tenant_id, name, type, profile_id, group_id) VALUES ($1,$2,$3,$4,$5)
		 RETURNING id, tenant_id, name, type, profile_id, group_id, created_at`,
		tenantID, in.Name, in.Type, in.ProfileID, in.GroupID,
	).Scan(&d.ID, &d.TenantID, &d.Name, &d.Type, &d.ProfileID, &d.GroupID, &d.CreatedAt)
	return d, err
}

func UpdateDevice(ctx context.Context, pool *pgxpool.Pool, tenantID, id uuid.UUID, in DeviceInput) (Device, error) {
	var d Device
	err := pool.QueryRow(ctx,
		`UPDATE devices SET name=$1, type=$2, profile_id=$3, group_id=$4
		 WHERE tenant_id=$5 AND id=$6
		 RETURNING id, tenant_id, name, type, profile_id, group_id, created_at`,
		in.Name, in.Type, in.ProfileID, in.GroupID, tenantID, id,
	).Scan(&d.ID, &d.TenantID, &d.Name, &d.Type, &d.ProfileID, &d.GroupID, &d.CreatedAt)
	return d, err
}

func DeleteDevice(ctx context.Context, pool *pgxpool.Pool, tenantID, deviceID uuid.UUID) error {
	_, err := pool.Exec(ctx, `DELETE FROM devices WHERE tenant_id=$1 AND id=$2`, tenantID, deviceID)
	return err
}

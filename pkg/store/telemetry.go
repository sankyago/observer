// Package store provides database insert helpers for telemetry and fired actions.
package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RawRow struct {
	Time      time.Time
	TenantID  uuid.UUID
	DeviceID  uuid.UUID
	MessageID uuid.UUID
	Payload   []byte // JSON bytes
}

func InsertRaw(ctx context.Context, pool *pgxpool.Pool, r RawRow) error {
	_, err := pool.Exec(ctx,
		`INSERT INTO telemetry_raw (time, tenant_id, device_id, message_id, payload)
		 VALUES ($1, $2, $3, $4, $5)`,
		r.Time, r.TenantID, r.DeviceID, r.MessageID, r.Payload,
	)
	return err
}

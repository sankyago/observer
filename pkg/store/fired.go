package store

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type FiredRow struct {
	TenantID  uuid.UUID
	DeviceID  uuid.UUID
	RuleID    uuid.UUID
	ActionID  uuid.UUID
	MessageID uuid.UUID
	Status    string // "ok" or "error"
	Error     string
	Payload   []byte
}

func InsertFired(ctx context.Context, pool *pgxpool.Pool, f FiredRow) error {
	var errText *string
	if f.Error != "" {
		errText = &f.Error
	}
	_, err := pool.Exec(ctx,
		`INSERT INTO fired_actions (tenant_id, device_id, rule_id, action_id, message_id, status, error, payload)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		f.TenantID, f.DeviceID, f.RuleID, f.ActionID, f.MessageID, f.Status, errText, f.Payload,
	)
	return err
}

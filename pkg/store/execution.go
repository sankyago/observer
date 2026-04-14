package store

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ExecutionRow struct {
	TenantID  uuid.UUID
	DeviceID  uuid.UUID
	FlowID    uuid.UUID
	NodeID    string
	MessageID uuid.UUID
	Status    string
	Error     string
	Payload   []byte
}

func InsertExecution(ctx context.Context, pool *pgxpool.Pool, e ExecutionRow) error {
	var errText *string
	if e.Error != "" {
		errText = &e.Error
	}
	_, err := pool.Exec(ctx,
		`INSERT INTO flow_executions (tenant_id, device_id, flow_id, node_id, message_id, status, error, payload)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		e.TenantID, e.DeviceID, e.FlowID, e.NodeID, e.MessageID, e.Status, errText, e.Payload,
	)
	return err
}

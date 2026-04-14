package repo

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type FiredRow struct {
	ID        uuid.UUID       `json:"id"`
	FiredAt   time.Time       `json:"fired_at"`
	DeviceID  uuid.UUID       `json:"device_id"`
	RuleID    uuid.UUID       `json:"rule_id"`
	ActionID  uuid.UUID       `json:"action_id"`
	MessageID uuid.UUID       `json:"message_id"`
	Status    string          `json:"status"`
	Error     string          `json:"error,omitempty"`
	Payload   json.RawMessage `json:"payload"`
}

func RecentFired(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID, limit int) ([]FiredRow, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := pool.Query(ctx,
		`SELECT id, fired_at, device_id, rule_id, action_id, message_id, status, coalesce(error,''), payload
		 FROM fired_actions WHERE tenant_id=$1 ORDER BY fired_at DESC LIMIT $2`,
		tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FiredRow{}
	for rows.Next() {
		var r FiredRow
		if err := rows.Scan(&r.ID, &r.FiredAt, &r.DeviceID, &r.RuleID, &r.ActionID, &r.MessageID, &r.Status, &r.Error, &r.Payload); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

package repo

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TelemetryRow struct {
	Time      time.Time       `json:"time"`
	DeviceID  uuid.UUID       `json:"device_id"`
	MessageID uuid.UUID       `json:"message_id"`
	Payload   json.RawMessage `json:"payload"`
}

func RecentTelemetry(ctx context.Context, pool *pgxpool.Pool, tenantID, deviceID uuid.UUID, limit int) ([]TelemetryRow, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := pool.Query(ctx,
		`SELECT time, device_id, message_id, payload FROM telemetry_raw
		 WHERE tenant_id=$1 AND device_id=$2 ORDER BY time DESC LIMIT $3`,
		tenantID, deviceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TelemetryRow{}
	for rows.Next() {
		var r TelemetryRow
		if err := rows.Scan(&r.Time, &r.DeviceID, &r.MessageID, &r.Payload); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

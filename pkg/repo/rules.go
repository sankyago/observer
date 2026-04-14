package repo

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Rule struct {
	ID        uuid.UUID `json:"id"`
	TenantID  uuid.UUID `json:"tenant_id"`
	DeviceID  uuid.UUID `json:"device_id"`
	Field     string    `json:"field"`
	Op        string    `json:"op"`
	Value     float64   `json:"value"`
	ActionID  uuid.UUID `json:"action_id"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}

type RuleInput struct {
	DeviceID uuid.UUID `json:"device_id"`
	Field    string    `json:"field"`
	Op       string    `json:"op"`
	Value    float64   `json:"value"`
	ActionID uuid.UUID `json:"action_id"`
	Enabled  bool      `json:"enabled"`
}

func ListRules(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID) ([]Rule, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, tenant_id, device_id, field, op, value, action_id, enabled, created_at
		 FROM rules WHERE tenant_id=$1 ORDER BY created_at DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Rule{}
	for rows.Next() {
		var r Rule
		if err := rows.Scan(&r.ID, &r.TenantID, &r.DeviceID, &r.Field, &r.Op, &r.Value, &r.ActionID, &r.Enabled, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func CreateRule(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID, in RuleInput) (Rule, error) {
	var r Rule
	err := pool.QueryRow(ctx,
		`INSERT INTO rules (tenant_id, device_id, field, op, value, action_id, enabled)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)
		 RETURNING id, tenant_id, device_id, field, op, value, action_id, enabled, created_at`,
		tenantID, in.DeviceID, in.Field, in.Op, in.Value, in.ActionID, in.Enabled,
	).Scan(&r.ID, &r.TenantID, &r.DeviceID, &r.Field, &r.Op, &r.Value, &r.ActionID, &r.Enabled, &r.CreatedAt)
	if err != nil {
		return Rule{}, err
	}
	_, _ = pool.Exec(ctx, "SELECT pg_notify('rules_changed', $1)", r.DeviceID.String())
	return r, nil
}

func UpdateRule(ctx context.Context, pool *pgxpool.Pool, tenantID, ruleID uuid.UUID, in RuleInput) (Rule, error) {
	var r Rule
	err := pool.QueryRow(ctx,
		`UPDATE rules SET device_id=$1, field=$2, op=$3, value=$4, action_id=$5, enabled=$6
		 WHERE tenant_id=$7 AND id=$8
		 RETURNING id, tenant_id, device_id, field, op, value, action_id, enabled, created_at`,
		in.DeviceID, in.Field, in.Op, in.Value, in.ActionID, in.Enabled, tenantID, ruleID,
	).Scan(&r.ID, &r.TenantID, &r.DeviceID, &r.Field, &r.Op, &r.Value, &r.ActionID, &r.Enabled, &r.CreatedAt)
	if err != nil {
		return Rule{}, err
	}
	_, _ = pool.Exec(ctx, "SELECT pg_notify('rules_changed', $1)", r.DeviceID.String())
	return r, nil
}

func DeleteRule(ctx context.Context, pool *pgxpool.Pool, tenantID, ruleID uuid.UUID) error {
	_, err := pool.Exec(ctx, `DELETE FROM rules WHERE tenant_id=$1 AND id=$2`, tenantID, ruleID)
	if err == nil {
		_, _ = pool.Exec(ctx, "SELECT pg_notify('rules_changed', '')")
	}
	return err
}

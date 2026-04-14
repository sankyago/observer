package rules

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/observer-io/observer/pkg/models"
)

// LoadAll reads all enabled rules from the database.
func LoadAll(ctx context.Context, pool *pgxpool.Pool) ([]models.Rule, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, tenant_id, device_id, field, op, value, action_id, enabled, debug_until, created_at
		 FROM rules WHERE enabled = TRUE`)
	if err != nil {
		return nil, fmt.Errorf("query rules: %w", err)
	}
	defer rows.Close()

	var out []models.Rule
	for rows.Next() {
		var r models.Rule
		var op string
		if err := rows.Scan(&r.ID, &r.TenantID, &r.DeviceID, &r.Field, &op, &r.Value, &r.ActionID, &r.Enabled, &r.DebugUntil, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan rule: %w", err)
		}
		r.Op = models.RuleOp(op)
		out = append(out, r)
	}
	return out, rows.Err()
}

// WatchAndRefresh runs until ctx is done. On each NOTIFY or on connection failure,
// it re-runs LoadAll and atomically replaces the cache contents. Logs errors.
func WatchAndRefresh(ctx context.Context, pool *pgxpool.Pool, cache *Cache, logger *slog.Logger) error {
	if err := refresh(ctx, pool, cache); err != nil {
		logger.Error("rules initial load", "err", err)
	}

	for {
		if err := listenLoop(ctx, pool, cache, logger); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			logger.Warn("rules listener lost, reconnecting", "err", err)
		}
		if ctx.Err() != nil {
			return nil
		}
	}
}

func listenLoop(ctx context.Context, pool *pgxpool.Pool, cache *Cache, logger *slog.Logger) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "LISTEN rules_changed"); err != nil {
		return err
	}
	for {
		if _, err := conn.Conn().WaitForNotification(ctx); err != nil {
			return err
		}
		if err := refresh(ctx, pool, cache); err != nil {
			logger.Error("rules refresh", "err", err)
		}
	}
}

func refresh(ctx context.Context, pool *pgxpool.Pool, cache *Cache) error {
	rs, err := LoadAll(ctx, pool)
	if err != nil {
		return err
	}
	cache.Replace(rs)
	return nil
}

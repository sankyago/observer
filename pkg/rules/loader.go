package rules

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/observer-io/observer/pkg/events"
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

// WatchBus subscribes to the events bus and reloads the rule cache whenever a
// "rules_changed" event arrives. A 30-second ticker provides eventual consistency
// if an event is missed or the bus is nil.
func WatchBus(ctx context.Context, bus *events.Bus, pool *pgxpool.Pool, cache *Cache, logger *slog.Logger) error {
	if err := refresh(ctx, pool, cache); err != nil {
		logger.Error("rules initial load", "err", err)
	}

	var sub <-chan events.Event
	if bus != nil {
		sub = bus.Subscribe(ctx)
	}
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := refresh(ctx, pool, cache); err != nil {
				logger.Error("rules periodic refresh", "err", err)
			}
		case ev, ok := <-sub:
			if !ok {
				sub = nil
				continue
			}
			if ev.Type != "rules_changed" {
				continue
			}
			if err := refresh(ctx, pool, cache); err != nil {
				logger.Error("rules refresh", "err", err)
			}
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

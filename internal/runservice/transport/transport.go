// Package transport runs the MQTT consumer / rule evaluator / Timescale writer.
package transport

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/observer-io/observer/pkg/config"
	"github.com/observer-io/observer/pkg/db"
	observerlog "github.com/observer-io/observer/pkg/log"
	"github.com/observer-io/observer/pkg/mqtt"
	"github.com/observer-io/observer/pkg/queue"
	"github.com/observer-io/observer/pkg/rules"
	"github.com/observer-io/observer/pkg/store"
	"github.com/observer-io/observer/pkg/topic"
)

// Run connects to Postgres and EMQX, subscribes to the shared telemetry topic,
// and processes messages until ctx is done.
func Run(ctx context.Context, cfg *config.Config, q queue.Queue) error {
	logger := observerlog.New(cfg.Log.Level).With("svc", "transport")
	logger.Info("transport starting")

	pool, err := db.NewPool(ctx, cfg.DB.DSN)
	if err != nil {
		return err
	}
	defer pool.Close()

	cache := rules.NewCache()
	go func() {
		if err := rules.WatchAndRefresh(ctx, pool, cache, logger); err != nil {
			logger.Error("rules watcher exited", "err", err)
		}
	}()

	mc, ch, err := mqtt.Options{
		BrokerURL:  cfg.MQTT.URL,
		ClientID:   "transport-" + uuid.NewString(),
		ShareGroup: "transport",
		Topic:      "tenants/+/devices/+/telemetry",
		Logger:     logger,
	}.Start(ctx)
	if err != nil {
		return err
	}
	defer mc.Stop()

	logger.Info("transport ready")

	for {
		select {
		case <-ctx.Done():
			logger.Info("transport shutting down")
			return nil
		case msg, ok := <-ch:
			if !ok {
				return nil
			}
			handle(ctx, logger, pool, cache, q, msg)
		}
	}
}

func handle(ctx context.Context, logger *slog.Logger, pool *pgxpool.Pool, cache *rules.Cache, q queue.Queue, msg mqtt.Message) {
	parsed, err := topic.ParseTelemetry(msg.Topic)
	if err != nil {
		logger.Warn("bad topic", "topic", msg.Topic)
		return
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(msg.Payload, &obj); err != nil {
		logger.Warn("bad payload", "err", err)
		return
	}

	messageID, _ := uuid.NewV7()

	var tenantID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT tenant_id FROM devices WHERE id=$1`, parsed.DeviceID).Scan(&tenantID); err != nil {
		logger.Warn("unknown device", "device_id", parsed.DeviceID, "err", err)
		return
	}

	if err := store.InsertRaw(ctx, pool, store.RawRow{
		Time: time.Now().UTC(), TenantID: tenantID, DeviceID: parsed.DeviceID,
		MessageID: messageID, Payload: msg.Payload,
	}); err != nil {
		logger.Error("insert raw", "err", err)
	}

	for _, r := range cache.GetByDevice(parsed.DeviceID) {
		match, err := rules.Evaluate(r, msg.Payload)
		if err != nil {
			logger.Warn("rule eval", "err", err)
			continue
		}
		if !match {
			continue
		}
		job := queue.Job{
			ID:            uuid.New(),
			TenantID:      r.TenantID,
			DeviceID:      r.DeviceID,
			RuleID:        r.ID,
			ActionID:      r.ActionID,
			MessageID:     messageID,
			Payload:       msg.Payload,
			CorrelationID: messageID,
		}
		if err := q.Enqueue(ctx, job); err != nil {
			logger.Error("enqueue", "err", err)
		}
	}
}

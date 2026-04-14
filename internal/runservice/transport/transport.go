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
	"github.com/observer-io/observer/pkg/events"
	"github.com/observer-io/observer/pkg/flowengine"
	observerlog "github.com/observer-io/observer/pkg/log"
	"github.com/observer-io/observer/pkg/mqtt"
	"github.com/observer-io/observer/pkg/queue"
	"github.com/observer-io/observer/pkg/store"
	"github.com/observer-io/observer/pkg/topic"
)

func Run(ctx context.Context, cfg *config.Config, q queue.Queue, bus *events.Bus) error {
	logger := observerlog.New(cfg.Log.Level).With("svc", "transport")
	logger.Info("transport starting")

	pool, err := db.NewPool(ctx, cfg.DB.DSN)
	if err != nil { return err }
	defer pool.Close()

	cache := flowengine.NewCache()
	go func() {
		if err := flowengine.WatchBus(ctx, bus, pool, cache, logger); err != nil {
			logger.Error("flows watcher exited", "err", err)
		}
	}()

	mc, ch, err := mqtt.Options{
		BrokerURL:  cfg.MQTT.URL,
		ClientID:   "transport-" + uuid.NewString(),
		ShareGroup: "transport",
		Topic:      "tenants/+/devices/+/telemetry",
		Logger:     logger,
	}.Start(ctx)
	if err != nil { return err }
	defer mc.Stop()

	logger.Info("transport ready")
	for {
		select {
		case <-ctx.Done():
			return nil
		case msg, ok := <-ch:
			if !ok { return nil }
			handle(ctx, logger, pool, cache, q, bus, msg)
		}
	}
}

func handle(ctx context.Context, logger *slog.Logger, pool *pgxpool.Pool, cache *flowengine.Cache, q queue.Queue, bus *events.Bus, msg mqtt.Message) {
	parsed, err := topic.ParseTelemetry(msg.Topic)
	if err != nil { logger.Warn("bad topic", "topic", msg.Topic); return }
	if !json.Valid(msg.Payload) {
		logger.Warn("bad payload"); return
	}
	messageID, _ := uuid.NewV7()
	now := time.Now().UTC()

	var tenantID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT tenant_id FROM devices WHERE id=$1`, parsed.DeviceID).Scan(&tenantID); err != nil {
		logger.Warn("unknown device", "device_id", parsed.DeviceID); return
	}

	if err := store.InsertRaw(ctx, pool, store.RawRow{
		Time: now, TenantID: tenantID, DeviceID: parsed.DeviceID,
		MessageID: messageID, Payload: msg.Payload,
	}); err != nil {
		logger.Error("insert raw", "err", err)
	}

	if bus != nil {
		body, _ := json.Marshal(map[string]any{
			"time": now, "device_id": parsed.DeviceID,
			"message_id": messageID, "payload": json.RawMessage(msg.Payload),
		})
		bus.Publish(events.Event{Type: "telemetry", Data: body})
	}

	for _, fi := range cache.GetByDevice(parsed.DeviceID) {
		hits, err := flowengine.Traverse(fi, msg.Payload)
		if err != nil {
			logger.Warn("traverse", "err", err, "flow_id", fi.FlowID)
			continue
		}
		for _, h := range hits {
			job := queue.Job{
				ID: uuid.New(), TenantID: fi.TenantID, DeviceID: parsed.DeviceID,
				FlowID: fi.FlowID, NodeID: h.NodeID, Kind: h.Kind, Config: h.Config,
				MessageID: messageID, Payload: msg.Payload, CorrelationID: messageID,
			}
			if err := q.Enqueue(ctx, job); err != nil {
				logger.Error("enqueue", "err", err)
			}
		}
	}
}

// Package runner consumes action jobs from the queue and executes them.
package runner

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/observer-io/observer/pkg/actions"
	"github.com/observer-io/observer/pkg/config"
	"github.com/observer-io/observer/pkg/db"
	observerlog "github.com/observer-io/observer/pkg/log"
	"github.com/observer-io/observer/pkg/models"
	"github.com/observer-io/observer/pkg/queue"
	"github.com/observer-io/observer/pkg/store"
)

func Run(ctx context.Context, cfg *config.Config, q queue.Queue) error {
	logger := observerlog.New(cfg.Log.Level).With("svc", "runner")
	logger.Info("runner starting")

	pool, err := db.NewPool(ctx, cfg.DB.DSN)
	if err != nil {
		return err
	}
	defer pool.Close()

	reg := actions.Registry{
		Log:     actions.LogAction{Logger: logger},
		Webhook: actions.WebhookAction{Client: &http.Client{Timeout: 5 * time.Second}},
	}

	logger.Info("runner ready")
	return q.Consume(ctx, func(jctx context.Context, j queue.Job) error {
		return execute(jctx, logger, pool, reg, j)
	})
}

func execute(ctx context.Context, logger *slog.Logger, pool *pgxpool.Pool, reg actions.Registry, j queue.Job) error {
	var action models.Action
	var kind string
	if err := pool.QueryRow(ctx,
		`SELECT id, tenant_id, kind, config, created_at FROM actions WHERE id=$1`,
		j.ActionID,
	).Scan(&action.ID, &action.TenantID, &kind, &action.Config, &action.CreatedAt); err != nil {
		logger.Error("load action", "err", err, "action_id", j.ActionID)
		return nil
	}
	action.Kind = models.ActionKind(kind)

	runErr := reg.Run(ctx, actions.Input{
		Action: action, RuleID: j.RuleID, DeviceID: j.DeviceID, TenantID: j.TenantID,
		MessageID: j.MessageID, Payload: j.Payload,
	})

	status := "ok"
	errText := ""
	if runErr != nil {
		status = "error"
		errText = runErr.Error()
		logger.Warn("action failed", "err", runErr)
	}
	if err := store.InsertFired(ctx, pool, store.FiredRow{
		TenantID: j.TenantID, DeviceID: j.DeviceID, RuleID: j.RuleID, ActionID: j.ActionID,
		MessageID: j.MessageID, Status: status, Error: errText, Payload: j.Payload,
	}); err != nil {
		logger.Error("insert fired", "err", err)
	}
	return nil
}

// Package api runs the HTTP control-plane server.
package api

import (
	"context"
	"net/http"
	"time"

	"github.com/observer-io/observer/pkg/config"
	"github.com/observer-io/observer/pkg/db"
	"github.com/observer-io/observer/pkg/events"
	"github.com/observer-io/observer/pkg/httpapi"
	observerlog "github.com/observer-io/observer/pkg/log"
)

func Run(ctx context.Context, cfg *config.Config, bus *events.Bus) error {
	logger := observerlog.New(cfg.Log.Level).With("svc", "api")
	logger.Info("api starting")

	pool, err := db.NewPool(ctx, cfg.DB.DSN)
	if err != nil {
		return err
	}
	defer pool.Close()

	handler := httpapi.BuildRouter(httpapi.Deps{Pool: pool, Bus: bus, Logger: logger})

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	logger.Info("api ready", "addr", srv.Addr)

	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		logger.Info("api shutting down")
		return nil
	case err := <-errc:
		return err
	}
}

// Command all runs transport, runner, and api in a single process (Mode A monolith).
// v1 stub: just loads config, opens DB, logs readiness, blocks.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/observer-io/observer/pkg/config"
	"github.com/observer-io/observer/pkg/db"
	"github.com/observer-io/observer/pkg/log"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	logger := log.New(cfg.Log.Level)
	logger.Info("monolith starting")

	pool, err := db.NewPool(context.Background(), cfg.DB.DSN)
	if err != nil {
		logger.Error("db", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	logger.Info("monolith ready (transport+runner+api stubs)")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	logger.Info("monolith shutting down")
}

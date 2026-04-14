// Command transport runs the MQTT consumer / rule evaluator / Timescale writer.
// v1 stub: loads config, opens DB, logs readiness, blocks until SIGINT/SIGTERM.
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
	logger.Info("transport starting")

	pool, err := db.NewPool(context.Background(), cfg.DB.DSN)
	if err != nil {
		logger.Error("db", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	logger.Info("transport ready", "mqtt_url", cfg.MQTT.URL)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	logger.Info("transport shutting down")
}

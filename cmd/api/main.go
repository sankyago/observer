package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/observer-io/observer/internal/runservice/api"
	"github.com/observer-io/observer/pkg/config"
	"github.com/observer-io/observer/pkg/events"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	bus := events.NewBus(64)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := api.Run(ctx, cfg, bus); err != nil {
		os.Exit(1)
	}
}

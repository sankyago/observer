package main

import (
	"context"
	"os/signal"
	"sync"
	"syscall"

	"github.com/observer-io/observer/internal/runservice/runner"
	"github.com/observer-io/observer/internal/runservice/transport"
	"github.com/observer-io/observer/pkg/config"
	observerlog "github.com/observer-io/observer/pkg/log"
	"github.com/observer-io/observer/pkg/queue/inmem"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	logger := observerlog.New(cfg.Log.Level).With("svc", "all")
	logger.Info("monolith starting")

	q := inmem.New(4096)
	defer q.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if err := transport.Run(ctx, cfg, q); err != nil {
			logger.Error("transport exited", "err", err)
		}
	}()
	go func() {
		defer wg.Done()
		if err := runner.Run(ctx, cfg, q); err != nil {
			logger.Error("runner exited", "err", err)
		}
	}()

	<-ctx.Done()
	logger.Info("monolith shutting down")
	wg.Wait()
}

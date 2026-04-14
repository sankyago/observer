package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/observer-io/observer/internal/runservice/transport"
	"github.com/observer-io/observer/pkg/config"
	"github.com/observer-io/observer/pkg/queue/inmem"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	q := inmem.New(4096)
	defer q.Close()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := transport.Run(ctx, cfg, q); err != nil {
		os.Exit(1)
	}
}

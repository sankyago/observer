package db

import (
	"context"
	"testing"

	"github.com/observer-io/observer/internal/testutil"
)

func TestNewPool_PingsTimescale(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping container-backed test in short mode")
	}
	dsn := testutil.StartTimescale(t)
	pool, err := NewPool(context.Background(), dsn)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

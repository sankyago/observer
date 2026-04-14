// Package testutil provides shared test helpers (Postgres/Timescale containers).
package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// StartTimescale starts a Timescale container and returns its DSN.
// The container is terminated when the test ends.
func StartTimescale(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	c, err := tcpostgres.Run(ctx,
		"timescale/timescaledb:2.17.2-pg16",
		tcpostgres.WithDatabase("observer"),
		tcpostgres.WithUsername("observer"),
		tcpostgres.WithPassword("observer"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("start timescale: %v", err)
	}
	t.Cleanup(func() { _ = c.Terminate(ctx) })
	dsn, err := c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}
	return dsn
}

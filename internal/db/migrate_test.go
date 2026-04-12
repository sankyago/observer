//go:build integration

package db

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestMigrate_RunsAllFiles(t *testing.T) {
	ctx := context.Background()
	pg, err := postgres.Run(ctx, "timescale/timescaledb:latest-pg16",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pg.Terminate(ctx) })

	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	require.NoError(t, Migrate(ctx, pool, "../../migrations"))

	var count int
	err = pool.QueryRow(ctx, "SELECT count(*) FROM flows").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count)

	// Second run must be idempotent — all DDL uses IF NOT EXISTS.
	require.NoError(t, Migrate(ctx, pool, "../../migrations"), "second Migrate must not error")
}

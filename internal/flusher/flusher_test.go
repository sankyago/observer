//go:build integration

package flusher

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sankyago/observer/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupTimescaleDB(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()

	container, err := postgres.Run(ctx,
		"timescale/timescaledb:latest-pg16",
		postgres.WithDatabase("observer_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { container.Terminate(ctx) })

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS sensor_data (
			time        TIMESTAMPTZ NOT NULL,
			device_id   TEXT NOT NULL,
			metric      TEXT NOT NULL,
			min_value   DOUBLE PRECISION NOT NULL,
			max_value   DOUBLE PRECISION NOT NULL,
			avg_value   DOUBLE PRECISION NOT NULL,
			count       INTEGER NOT NULL,
			last_value  DOUBLE PRECISION NOT NULL
		);
		SELECT create_hypertable('sensor_data', 'time', if_not_exists => TRUE);
	`)
	require.NoError(t, err)

	return pool
}

func TestFlusher_WritesToDB(t *testing.T) {
	ctx := context.Background()
	pool := setupTimescaleDB(t, ctx)

	in := make(chan model.SensorReading, 100)
	f := NewFlusher(pool, in, WithFlushInterval(100*time.Millisecond))

	fCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		f.Run(fCtx)
		close(done)
	}()

	ts := time.Date(2026, 4, 12, 10, 0, 1, 0, time.UTC)
	in <- model.SensorReading{DeviceID: "m1", Metric: "temperature", Value: 50.0, Timestamp: ts}
	in <- model.SensorReading{DeviceID: "m1", Metric: "temperature", Value: 60.0, Timestamp: ts}

	// Wait for flush
	time.Sleep(300 * time.Millisecond)
	cancel()
	<-done

	// Query with fresh context
	var rowCount int
	var minVal, maxVal, avgVal, lastVal float64
	err := pool.QueryRow(context.Background(), `
		SELECT count, min_value, max_value, avg_value, last_value
		FROM sensor_data WHERE device_id = 'm1' LIMIT 1
	`).Scan(&rowCount, &minVal, &maxVal, &avgVal, &lastVal)
	require.NoError(t, err)
	assert.Equal(t, 2, rowCount)
	assert.Equal(t, 50.0, minVal)
	assert.Equal(t, 60.0, maxVal)
	assert.Equal(t, 55.0, avgVal)
	assert.Equal(t, 60.0, lastVal)
}

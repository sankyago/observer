//go:build integration

package test

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sankyago/observer/internal/engine"
	"github.com/sankyago/observer/internal/flusher"
	"github.com/sankyago/observer/internal/model"
	"github.com/sankyago/observer/internal/subscriber"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestE2E_MQTTToAlertAndDB(t *testing.T) {
	ctx := context.Background()

	_, thisFile, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(thisFile), "..")
	confPath := filepath.Join(projectRoot, "mosquitto.conf")

	// Start Mosquitto container
	mqttReq := testcontainers.ContainerRequest{
		Image:        "eclipse-mosquitto:2",
		ExposedPorts: []string{"1883/tcp"},
		Files: []testcontainers.ContainerFile{
			{
				HostFilePath:      confPath,
				ContainerFilePath: "/mosquitto/config/mosquitto.conf",
				FileMode:          0644,
			},
		},
		WaitingFor: wait.ForListeningPort("1883/tcp"),
	}
	mqttContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: mqttReq,
		Started:          true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { testcontainers.CleanupContainer(t, mqttContainer) })

	mqttHost, err := mqttContainer.Host(ctx)
	require.NoError(t, err)
	mqttPort, err := mqttContainer.MappedPort(ctx, "1883/tcp")
	require.NoError(t, err)
	brokerURL := fmt.Sprintf("tcp://%s:%s", mqttHost, mqttPort.Port())

	// Start TimescaleDB container
	pgContainer, err := postgres.Run(ctx,
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
	t.Cleanup(func() { testcontainers.CleanupContainer(t, pgContainer) })

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	defer pool.Close()

	// Run migration
	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS sensor_data (
			time TIMESTAMPTZ NOT NULL,
			device_id TEXT NOT NULL,
			metric TEXT NOT NULL,
			min_value DOUBLE PRECISION NOT NULL,
			max_value DOUBLE PRECISION NOT NULL,
			avg_value DOUBLE PRECISION NOT NULL,
			count INTEGER NOT NULL,
			last_value DOUBLE PRECISION NOT NULL
		);
		SELECT create_hypertable('sensor_data', 'time', if_not_exists => TRUE);
	`)
	require.NoError(t, err)

	// Wire full pipeline
	mqttToEngine := make(chan model.SensorReading, 100)
	engineToFlusher := make(chan model.SensorReading, 100)

	sub := subscriber.New(brokerURL, "sensors/#", mqttToEngine)
	err = sub.Run(brokerURL)
	require.NoError(t, err)
	defer sub.Stop()

	var alertBuf bytes.Buffer
	eng := engine.NewEngine(engine.WithOutput(&alertBuf), engine.WithCooldown(0))
	go eng.Run(mqttToEngine, engineToFlusher)

	flushCtx, cancelFlush := context.WithCancel(ctx)
	defer cancelFlush()

	f := flusher.NewFlusher(pool, engineToFlusher, flusher.WithFlushInterval(200*time.Millisecond))
	go f.Run(flushCtx)

	// Give subscriber time to connect and be ready
	time.Sleep(500 * time.Millisecond)

	// Publish an abnormal temperature reading (exceeds 80.0 max threshold)
	pubOpts := mqtt.NewClientOptions().AddBroker(brokerURL).SetClientID("test-e2e-publisher")
	pubClient := mqtt.NewClient(pubOpts)
	token := pubClient.Connect()
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())
	defer pubClient.Disconnect(250)

	payload := `{"value": 96.2, "timestamp": "2026-04-12T10:00:01Z"}`
	token = pubClient.Publish("sensors/machine-42/temperature", 1, false, payload)
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())

	// Wait for the pipeline to process the message and flush to DB
	time.Sleep(1 * time.Second)

	// Verify alert output
	alertOutput := alertBuf.String()
	assert.True(t, strings.Contains(alertOutput, "[ALERT]"), "expected [ALERT] in output, got: %s", alertOutput)
	assert.True(t, strings.Contains(alertOutput, "machine-42"), "expected machine-42 in output, got: %s", alertOutput)
	assert.True(t, strings.Contains(alertOutput, "THRESHOLD"), "expected THRESHOLD in output, got: %s", alertOutput)

	// Verify DB row
	var count int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM sensor_data WHERE device_id = 'machine-42'`).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "expected 1 row in sensor_data for machine-42")
}

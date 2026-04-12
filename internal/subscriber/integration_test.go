//go:build integration

package subscriber

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/sankyago/observer/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestSubscriber_ReceivesMessage(t *testing.T) {
	ctx := context.Background()

	_, thisFile, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	confPath := filepath.Join(projectRoot, "mosquitto.conf")

	req := testcontainers.ContainerRequest{
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
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { testcontainers.CleanupContainer(t, container) })

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "1883/tcp")
	require.NoError(t, err)
	brokerURL := fmt.Sprintf("tcp://%s:%s", host, port.Port())

	out := make(chan model.SensorReading, 10)
	sub := New(brokerURL, "sensors/#", out)
	err = sub.Run(brokerURL)
	require.NoError(t, err)
	defer sub.Stop()

	// Give subscriber time to connect
	time.Sleep(500 * time.Millisecond)

	// Publish a test message
	pubOpts := mqtt.NewClientOptions().AddBroker(brokerURL).SetClientID("test-publisher")
	pubClient := mqtt.NewClient(pubOpts)
	token := pubClient.Connect()
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())
	defer pubClient.Disconnect(250)

	payload := `{"value": 42.5, "timestamp": "2026-04-12T10:00:01Z"}`
	token = pubClient.Publish("sensors/device-1/temperature", 1, false, payload)
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())

	// Wait for message
	select {
	case reading := <-out:
		assert.Equal(t, "device-1", reading.DeviceID)
		assert.Equal(t, "temperature", reading.Metric)
		assert.Equal(t, 42.5, reading.Value)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for reading")
	}
}

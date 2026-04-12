//go:build integration

package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/sankyago/observer/internal/model"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcwait "github.com/testcontainers/testcontainers-go/wait"
)

func TestMQTTSource_ReceivesMessage(t *testing.T) {
	ctx := context.Background()

	_, thisFile, _, _ := runtime.Caller(0)
	mosqConf := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "mosquitto.conf")

	req := testcontainers.ContainerRequest{
		Image:        "eclipse-mosquitto:2",
		ExposedPorts: []string{"1883/tcp"},
		Files: []testcontainers.ContainerFile{
			{HostFilePath: mosqConf, ContainerFilePath: "/mosquitto/config/mosquitto.conf", FileMode: 0644},
		},
		WaitingFor: tcwait.ForListeningPort("1883/tcp"),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: req, Started: true})
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Terminate(ctx) })

	host, err := c.Host(ctx)
	require.NoError(t, err)
	port, err := c.MappedPort(ctx, "1883")
	require.NoError(t, err)
	broker := fmt.Sprintf("tcp://%s:%s", host, port.Port())

	src, err := NewMQTTSource("src", json.RawMessage(fmt.Sprintf(`{"broker":%q,"topic":"sensors/#"}`, broker)))
	require.NoError(t, err)

	out := make(chan model.SensorReading, 1)
	events := make(chan FlowEvent, 1)
	runCtx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	go src.Run(runCtx, nil, out, events)

	time.Sleep(300 * time.Millisecond) // let subscribe settle

	pub := mqtt.NewClient(mqtt.NewClientOptions().AddBroker(broker).SetClientID("pub"))
	connTok := pub.Connect()
	require.True(t, connTok.WaitTimeout(5*time.Second), "pub connect timeout")
	require.NoError(t, connTok.Error())
	tok := pub.Publish("sensors/device-1/temperature", 0, false, `{"value":42.5,"timestamp":"2026-04-12T10:00:00Z"}`)
	require.True(t, tok.WaitTimeout(5*time.Second), "publish timeout")
	pub.Disconnect(100)

	select {
	case r := <-out:
		require.Equal(t, "device-1", r.DeviceID)
		require.Equal(t, "temperature", r.Metric)
		require.Equal(t, 42.5, r.Value)
	case <-time.After(3 * time.Second):
		t.Fatal("no reading received")
	}
}

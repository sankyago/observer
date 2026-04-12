//go:build integration

package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	dockercontainer "github.com/moby/moby/api/types/container"
	"github.com/sankyago/observer/internal/api"
	"github.com/sankyago/observer/internal/db"
	"github.com/sankyago/observer/internal/devices"
	devstore "github.com/sankyago/observer/internal/devices/store"
	"github.com/sankyago/observer/internal/flow"
	flowgraph "github.com/sankyago/observer/internal/flow/graph"
	flowruntime "github.com/sankyago/observer/internal/flow/runtime"
	flowstore "github.com/sankyago/observer/internal/flow/store"
	"github.com/sankyago/observer/internal/ingest"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	tcwait "github.com/testcontainers/testcontainers-go/wait"
)

func TestE2E_EMQX_To_WS(t *testing.T) {
	ctx := context.Background()
	const secret = "test-secret"
	const serviceUser = "observer-consumer"
	const servicePass = "observer-consumer-pw"

	// Postgres
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
	require.NoError(t, db.Migrate(ctx, pool, "../migrations"))

	// Wire Observer
	devSvc := devices.NewService(devstore.NewRepo(pool))
	router := ingest.NewRouter()
	mgr := flowruntime.NewManager(router)
	flowSvc := flow.NewService(ctx, flowstore.NewRepo(pool), mgr)
	httpHandler := api.NewRouter(flowSvc, devSvc, api.WithMQTTAuth(secret, serviceUser))
	srv := httptest.NewServer(httpHandler)
	t.Cleanup(srv.Close)
	t.Cleanup(mgr.StopAll)

	// Derive the URL that EMQX (inside Docker) can reach the httptest server on.
	hostPort := portFromURL(srv.URL)
	hostForEMQX := "http://host.docker.internal:" + hostPort
	authURL := hostForEMQX + "/api/mqtt/auth"
	aclURL := hostForEMQX + "/api/mqtt/acl"

	// Write a HOCON config file for EMQX so that HEADERS and BODY template
	// substitutions work (EMQX 5.7 does not expose these fields via env vars).
	// We include the mandatory node/cluster/dashboard sections plus auth config.
	emqxConf := fmt.Sprintf(`
node {
  name = "emqx@127.0.0.1"
  cookie = "emqxsecretcookie"
  data_dir = "data"
}
cluster {
  name = emqxcl
  discovery_strategy = manual
}
dashboard {
  listeners.http {
    bind = 18083
  }
}

authentication = [
  {
    mechanism = password_based
    backend = http
    method = post
    url = %q
    headers {
      "Content-Type" = "application/json"
      "X-EMQX-Secret" = %q
    }
    body {
      username = "${username}"
      password = "${password}"
      clientid = "${clientid}"
    }
  }
]

authorization {
  no_match = deny
  sources = [
    {
      type = http
      enable = true
      method = post
      url = %q
      headers {
        "Content-Type" = "application/json"
        "X-EMQX-Secret" = %q
      }
      body {
        username = "${username}"
        action   = "${action}"
        topic    = "${topic}"
      }
    }
  ]
}
`, authURL, secret, aclURL, secret)

	confFile, err := os.CreateTemp("", "emqx-*.conf")
	require.NoError(t, err)
	_, err = confFile.WriteString(emqxConf)
	require.NoError(t, err)
	confFile.Close()
	t.Cleanup(func() { os.Remove(confFile.Name()) })

	// EMQX testcontainer
	emqxReq := testcontainers.ContainerRequest{
		Image:        "emqx/emqx:5.7",
		ExposedPorts: []string{"1883/tcp"},
		Files: []testcontainers.ContainerFile{
			{
				HostFilePath:      confFile.Name(),
				ContainerFilePath: "/opt/emqx/etc/emqx.conf",
				FileMode:          0644,
			},
		},
		// Allow EMQX inside Docker to reach host.docker.internal on Linux.
		HostConfigModifier: func(hc *dockercontainer.HostConfig) {
			hc.ExtraHosts = append(hc.ExtraHosts, "host.docker.internal:host-gateway")
		},
		WaitingFor: tcwait.ForLog("is running now!").WithStartupTimeout(90 * time.Second),
	}
	ec, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: emqxReq,
		Started:          true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = ec.Terminate(ctx) })

	ehost, err := ec.Host(ctx)
	require.NoError(t, err)
	eport, err := ec.MappedPort(ctx, "1883/tcp")
	require.NoError(t, err)
	brokerURL := fmt.Sprintf("tcp://%s:%s", ehost, eport.Port())

	// Start Observer ingest consumer (service account).
	// Run in a goroutine; track errors so we can surface them.
	consumer := ingest.NewConsumer(ingest.Config{
		BrokerURL:   brokerURL,
		Username:    serviceUser,
		Password:    servicePass,
		SharedGroup: "observer",
		ClientID:    "observer-test",
	}, router)
	runCtx, runCancel := context.WithCancel(ctx)
	t.Cleanup(runCancel)
	consumerErrCh := make(chan error, 1)
	go func() { consumerErrCh <- consumer.Run(runCtx) }()
	// Give the consumer time to connect and subscribe.
	time.Sleep(2 * time.Second)

	// Create a device via API
	deviceBody, _ := json.Marshal(map[string]string{"name": "test-device"})
	resp, err := http.Post(srv.URL+"/api/devices", "application/json", bytes.NewReader(deviceBody))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var createdDevice struct {
		ID    string `json:"id"`
		Token string `json:"token"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&createdDevice))
	resp.Body.Close()

	// Create a flow: device_source → threshold(0..10) → debug_sink, enabled
	g := flowgraph.Graph{
		Nodes: []flowgraph.Node{
			{ID: "src", Type: "device_source", Data: json.RawMessage(`{"device_id":"` + createdDevice.ID + `"}`)},
			{ID: "th", Type: "threshold", Data: json.RawMessage(`{"min":0,"max":10}`)},
			{ID: "sink", Type: "debug_sink", Data: json.RawMessage(`{}`)},
		},
		Edges: []flowgraph.Edge{
			{ID: "e1", Source: "src", Target: "th"},
			{ID: "e2", Source: "th", Target: "sink"},
		},
	}
	flowBody, _ := json.Marshal(map[string]any{"name": "demo", "graph": g, "enabled": true})
	resp, err = http.Post(srv.URL+"/api/flows", "application/json", bytes.NewReader(flowBody))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var createdFlow struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&createdFlow))
	resp.Body.Close()
	// Allow flow compilation to settle
	time.Sleep(300 * time.Millisecond)

	// Open WebSocket to flow events
	wsURL := "ws" + srv.URL[len("http"):] + "/api/flows/" + createdFlow.ID + "/events"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	// Device publishes an out-of-range value via EMQX
	pubOpts := mqtt.NewClientOptions().
		AddBroker(brokerURL).
		SetClientID("device-test").
		SetUsername(createdDevice.Token)
	pub := mqtt.NewClient(pubOpts)
	tok := pub.Connect()
	require.True(t, tok.WaitTimeout(10*time.Second), "device MQTT connect timed out")
	require.NoError(t, tok.Error())

	mqttTopic := "v1/devices/" + createdDevice.ID + "/telemetry"
	mqttPayload := `{"temperature":99.9,"ts":"2026-04-12T10:00:00Z"}`
	ptok := pub.Publish(mqttTopic, 0, false, mqttPayload)
	require.True(t, ptok.WaitTimeout(5*time.Second), "publish timed out")
	pub.Disconnect(100)

	// Check consumer didn't fail to connect.
	select {
	case err := <-consumerErrCh:
		require.NoError(t, err, "ingest consumer failed")
	default:
		// still running — expected
	}

	// Expect an alert event on the WebSocket within 10 s
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(10*time.Second)))
	gotAlert := false
	for !gotAlert {
		var e map[string]any
		if err := conn.ReadJSON(&e); err != nil {
			break
		}
		if e["kind"] == "alert" {
			gotAlert = true
		}
	}
	require.True(t, gotAlert, "expected alert event on WebSocket")
}

// portFromURL extracts the port number from an http://host:port URL string.
func portFromURL(u string) string {
	for i := len(u) - 1; i >= 0; i-- {
		if u[i] == ':' {
			return u[i+1:]
		}
	}
	return ""
}

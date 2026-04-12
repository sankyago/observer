//go:build integration

package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sankyago/observer/internal/api"
	"github.com/sankyago/observer/internal/db"
	"github.com/sankyago/observer/internal/flow"
	flowgraph "github.com/sankyago/observer/internal/flow/graph"
	flowruntime "github.com/sankyago/observer/internal/flow/runtime"
	"github.com/sankyago/observer/internal/flow/store"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	tcwait "github.com/testcontainers/testcontainers-go/wait"
)

func TestE2E_FlowApiMQTTToWS(t *testing.T) {
	ctx := context.Background()

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

	// Mosquitto
	_, thisFile, _, _ := runtime.Caller(0)
	mosqConf := filepath.Join(filepath.Dir(thisFile), "..", "mosquitto.conf")
	mqc, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "eclipse-mosquitto:2",
			ExposedPorts: []string{"1883/tcp"},
			Files: []testcontainers.ContainerFile{
				{HostFilePath: mosqConf, ContainerFilePath: "/mosquitto/config/mosquitto.conf", FileMode: 0644},
			},
			WaitingFor: tcwait.ForListeningPort("1883/tcp"),
		},
		Started: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = mqc.Terminate(ctx) })
	host, _ := mqc.Host(ctx)
	port, _ := mqc.MappedPort(ctx, "1883")
	broker := fmt.Sprintf("tcp://%s:%s", host, port.Port())

	// Service + server
	repo := store.NewRepo(pool)
	mgr := flowruntime.NewManager()
	svc := flow.NewService(ctx, repo, mgr)
	srv := httptest.NewServer(api.NewRouter(svc))
	t.Cleanup(srv.Close)
	t.Cleanup(mgr.StopAll)

	// POST a flow: mqtt_source → threshold(0..10) → debug_sink, enabled
	g := flowgraph.Graph{
		Nodes: []flowgraph.Node{
			{ID: "src", Type: "mqtt_source", Data: json.RawMessage(fmt.Sprintf(`{"broker":%q,"topic":"sensors/#"}`, broker))},
			{ID: "th", Type: "threshold", Data: json.RawMessage(`{"min":0,"max":10}`)},
			{ID: "sink", Type: "debug_sink", Data: json.RawMessage(`{}`)},
		},
		Edges: []flowgraph.Edge{
			{ID: "e1", Source: "src", Target: "th"},
			{ID: "e2", Source: "th", Target: "sink"},
		},
	}
	body, _ := json.Marshal(map[string]any{"name": "demo", "graph": g, "enabled": true})
	resp, err := http.Post(srv.URL+"/api/flows", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var created struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))
	resp.Body.Close()

	// Open WS
	wsURL := "ws" + srv.URL[len("http"):] + "/api/flows/" + created.ID + "/events"
	time.Sleep(300 * time.Millisecond)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	// Publish out-of-range reading
	pub := mqtt.NewClient(mqtt.NewClientOptions().AddBroker(broker).SetClientID("pub"))
	require.NoError(t, pub.Connect().Error())
	time.Sleep(200 * time.Millisecond)
	pub.Publish("sensors/dev-1/temperature", 0, false, `{"value":99.9,"timestamp":"2026-04-12T10:00:00Z"}`).Wait()
	pub.Disconnect(100)

	// Expect an alert event
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var gotAlert bool
	for !gotAlert {
		var e map[string]any
		if err := conn.ReadJSON(&e); err != nil {
			break
		}
		if e["kind"] == "alert" {
			gotAlert = true
		}
	}
	require.True(t, gotAlert, "expected alert event over WS")
}

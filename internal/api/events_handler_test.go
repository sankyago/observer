package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/sankyago/observer/internal/flow"
	"github.com/sankyago/observer/internal/flow/nodes"
	"github.com/sankyago/observer/internal/flow/runtime"
	"github.com/sankyago/observer/internal/ingest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// wsDialer is a gorilla websocket dialer with no TLS verification needed.
var wsDialer = websocket.DefaultDialer

// newEnabledFlowService creates a service with one enabled debug_sink flow
// and returns the service and the flow's ID.
func newEnabledFlowService(t *testing.T) (*flow.Service, uuid.UUID) {
	t.Helper()
	repo := newFakeRepo()
	mgr := runtime.NewManager(ingest.NewRouter())
	svc := flow.NewService(context.Background(), repo, mgr)

	g := debugOnlyGraph()
	f, err := svc.Create(context.Background(), "ws-test", g, true)
	require.NoError(t, err)
	return svc, f.ID
}

// TestEventsWS_Upgrades verifies that connecting to /api/flows/<id>/events
// returns 101 Switching Protocols for a running flow.
func TestEventsWS_Upgrades(t *testing.T) {
	svc, id := newEnabledFlowService(t)
	srv := httptest.NewServer(NewRouter(svc, nil))
	t.Cleanup(srv.Close)

	wsURL := "ws" + srv.URL[len("http"):] + "/api/flows/" + id.String() + "/events"
	conn, resp, err := wsDialer.Dial(wsURL, nil)
	require.NoError(t, err, "expected 101 upgrade")
	defer conn.Close()
	assert.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
}

// TestEventsWS_ReceivesPublishedEvent verifies that after connecting,
// calling bus.Publish sends the JSON-encoded event to the WS client.
func TestEventsWS_ReceivesPublishedEvent(t *testing.T) {
	svc, id := newEnabledFlowService(t)
	srv := httptest.NewServer(NewRouter(svc, nil))
	t.Cleanup(srv.Close)

	wsURL := "ws" + srv.URL[len("http"):] + "/api/flows/" + id.String() + "/events"
	conn, _, err := wsDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	// Give the subscription goroutine a moment to register before publishing.
	time.Sleep(20 * time.Millisecond)

	bus := svc.Manager().Bus(id)
	require.NotNil(t, bus)

	want := nodes.FlowEvent{
		Kind:      "alert",
		NodeID:    "sink1",
		Detail:    "test-detail",
		Timestamp: time.Now().UTC().Truncate(time.Second),
	}
	bus.Publish(want)

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, msg, err := conn.ReadMessage()
	require.NoError(t, err)

	var got nodes.FlowEvent
	require.NoError(t, json.Unmarshal(msg, &got))
	assert.Equal(t, want.Kind, got.Kind)
	assert.Equal(t, want.NodeID, got.NodeID)
	assert.Equal(t, want.Detail, got.Detail)
}

// TestEventsWS_BogusID verifies that connecting to /api/flows/<bogus>/events
// returns 404 (flow not running).
func TestEventsWS_BogusID(t *testing.T) {
	svc, _ := newEnabledFlowService(t)
	srv := httptest.NewServer(NewRouter(svc, nil))
	t.Cleanup(srv.Close)

	bogus := uuid.New().String()
	wsURL := "ws" + srv.URL[len("http"):] + "/api/flows/" + bogus + "/events"
	_, resp, err := wsDialer.Dial(wsURL, nil)
	require.Error(t, err, "expected upgrade failure")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestEventsWS_InvalidUUID verifies that connecting with a non-UUID path segment
// returns 400.
func TestEventsWS_InvalidUUID(t *testing.T) {
	svc, _ := newEnabledFlowService(t)
	srv := httptest.NewServer(NewRouter(svc, nil))
	t.Cleanup(srv.Close)

	wsURL := "ws" + srv.URL[len("http"):] + "/api/flows/not-a-uuid/events"
	_, resp, err := wsDialer.Dial(wsURL, nil)
	require.Error(t, err, "expected upgrade failure")
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// TestEventsWS_UnsubscribesOnDisconnect verifies that closing the WS connection
// removes the subscription from the EventBus.
func TestEventsWS_UnsubscribesOnDisconnect(t *testing.T) {
	svc, id := newEnabledFlowService(t)
	srv := httptest.NewServer(NewRouter(svc, nil))
	t.Cleanup(srv.Close)

	bus := svc.Manager().Bus(id)
	require.NotNil(t, bus)

	before := bus.SubscriberCount()

	wsURL := "ws" + srv.URL[len("http"):] + "/api/flows/" + id.String() + "/events"
	conn, _, err := wsDialer.Dial(wsURL, nil)
	require.NoError(t, err)

	// Wait for the subscription to be registered.
	require.Eventually(t, func() bool {
		return bus.SubscriberCount() > before
	}, time.Second, 5*time.Millisecond, "subscription never registered")

	// Close the connection and wait for the server-side handler to unsubscribe.
	conn.Close()

	require.Eventually(t, func() bool {
		return bus.SubscriberCount() == before
	}, 2*time.Second, 10*time.Millisecond, "subscription not removed after disconnect")
}

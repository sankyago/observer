package runtime

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sankyago/observer/internal/flow/graph"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func singleSinkGraph() graph.Graph {
	return graph.Graph{
		Nodes: []graph.Node{{ID: "a", Type: "debug_sink", Data: json.RawMessage(`{}`)}},
	}
}

func TestManager_StartReplaceStop(t *testing.T) {
	mgr := NewManager()
	id := uuid.New()
	g := singleSinkGraph()

	require.NoError(t, mgr.Start(context.Background(), id, g))
	assert.True(t, mgr.Running(id))

	require.NoError(t, mgr.Replace(context.Background(), id, g))
	assert.True(t, mgr.Running(id))

	mgr.Stop(id)
	assert.False(t, mgr.Running(id))

	mgr.StopAll()
}

func TestManager_StartRegistersAndRunningReturnsTrue(t *testing.T) {
	mgr := NewManager()
	id := uuid.New()

	require.NoError(t, mgr.Start(context.Background(), id, singleSinkGraph()))
	defer mgr.StopAll()

	assert.True(t, mgr.Running(id))
}

func TestManager_StopUnregistersFlow(t *testing.T) {
	mgr := NewManager()
	id := uuid.New()

	require.NoError(t, mgr.Start(context.Background(), id, singleSinkGraph()))
	assert.True(t, mgr.Running(id))

	mgr.Stop(id)
	assert.False(t, mgr.Running(id))
}

func TestManager_StopUnknownIDIsNoOp(t *testing.T) {
	mgr := NewManager()
	assert.NotPanics(t, func() {
		mgr.Stop(uuid.New())
	})
}

func TestManager_ReplaceStopsPreviousFlow(t *testing.T) {
	mgr := NewManager()
	id := uuid.New()
	g := singleSinkGraph()

	require.NoError(t, mgr.Start(context.Background(), id, g))
	// Replace should stop previous and start a new one
	require.NoError(t, mgr.Replace(context.Background(), id, g))

	assert.True(t, mgr.Running(id))
	mgr.StopAll()
}

func TestManager_BusReturnsNilForUnknownID(t *testing.T) {
	mgr := NewManager()
	assert.Nil(t, mgr.Bus(uuid.New()))
}

func TestManager_BusReturnsEventBusForRunningFlow(t *testing.T) {
	mgr := NewManager()
	id := uuid.New()

	require.NoError(t, mgr.Start(context.Background(), id, singleSinkGraph()))
	defer mgr.StopAll()

	bus := mgr.Bus(id)
	require.NotNil(t, bus)
}

func TestManager_StopAllClearsMapAndStopsFlows(t *testing.T) {
	mgr := NewManager()
	ids := make([]uuid.UUID, 3)
	for i := range ids {
		ids[i] = uuid.New()
		require.NoError(t, mgr.Start(context.Background(), ids[i], singleSinkGraph()))
	}

	// Verify all running
	for _, id := range ids {
		assert.True(t, mgr.Running(id))
	}

	done := make(chan struct{})
	go func() {
		mgr.StopAll()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("StopAll did not return in time")
	}

	for _, id := range ids {
		assert.False(t, mgr.Running(id))
	}
}

func TestManager_StartOnAlreadyRunningIDReplacesCleanly(t *testing.T) {
	mgr := NewManager()
	id := uuid.New()

	require.NoError(t, mgr.Start(context.Background(), id, singleSinkGraph()))
	// Start again with same id — should replace the old flow
	require.NoError(t, mgr.Start(context.Background(), id, singleSinkGraph()))

	assert.True(t, mgr.Running(id))
	mgr.StopAll()
	assert.False(t, mgr.Running(id))
}

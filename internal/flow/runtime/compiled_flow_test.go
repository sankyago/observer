package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sankyago/observer/internal/flow/graph"
	"github.com/sankyago/observer/internal/ingest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeDebugSinkGraph(t *testing.T) graph.Graph {
	t.Helper()
	return graph.Graph{
		Nodes: []graph.Node{
			{ID: "a", Type: "debug_sink", Data: json.RawMessage(`{}`)},
		},
	}
}

func makeThresholdDebugGraph(t *testing.T) graph.Graph {
	t.Helper()
	return graph.Graph{
		Nodes: []graph.Node{
			{ID: "a", Type: "threshold", Data: json.RawMessage(`{"min":0,"max":10}`)},
			{ID: "b", Type: "debug_sink", Data: json.RawMessage(`{}`)},
		},
		Edges: []graph.Edge{{ID: "e1", Source: "a", Target: "b"}},
	}
}

func TestCompiledFlow_RunsThresholdPipeline(t *testing.T) {
	g := makeThresholdDebugGraph(t)
	require.NoError(t, graph.Validate(g))

	cf, err := Compile(uuid.New(), g, ingest.NewRouter())
	require.NoError(t, err)
	defer cf.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, cf.Start(ctx))
	// smoke test: starts and stops without error
}

func TestCompiledFlow_RejectsMultipleOutgoingEdges(t *testing.T) {
	// node "a" has two outgoing edges (a->b, a->c)
	g := graph.Graph{
		Nodes: []graph.Node{
			{ID: "a", Type: "threshold", Data: json.RawMessage(`{"min":0,"max":10}`)},
			{ID: "b", Type: "debug_sink", Data: json.RawMessage(`{}`)},
			{ID: "c", Type: "debug_sink", Data: json.RawMessage(`{}`)},
		},
		Edges: []graph.Edge{
			{ID: "e1", Source: "a", Target: "b"},
			{ID: "e2", Source: "a", Target: "c"},
		},
	}
	_, err := CompileWithSinkWriter(uuid.New(), g, ingest.NewRouter(), &bytes.Buffer{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple outgoing edges")
}

func TestCompiledFlow_RejectsMultipleIncomingEdges(t *testing.T) {
	// node "c" has two incoming edges (a->c, b->c)
	g := graph.Graph{
		Nodes: []graph.Node{
			{ID: "a", Type: "threshold", Data: json.RawMessage(`{"min":0,"max":10}`)},
			{ID: "b", Type: "threshold", Data: json.RawMessage(`{"min":0,"max":10}`)},
			{ID: "c", Type: "debug_sink", Data: json.RawMessage(`{}`)},
		},
		Edges: []graph.Edge{
			{ID: "e1", Source: "a", Target: "c"},
			{ID: "e2", Source: "b", Target: "c"},
		},
	}
	_, err := CompileWithSinkWriter(uuid.New(), g, ingest.NewRouter(), &bytes.Buffer{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple incoming edges")
}

func TestCompiledFlow_BuildNodeErrorsOnUnknownType(t *testing.T) {
	g := graph.Graph{
		Nodes: []graph.Node{
			{ID: "x", Type: "not_a_real_type", Data: json.RawMessage(`{}`)},
		},
	}
	_, err := CompileWithSinkWriter(uuid.New(), g, ingest.NewRouter(), &bytes.Buffer{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown node type")
}

func TestCompiledFlow_StartStopCleansUpGoroutines(t *testing.T) {
	g := makeDebugSinkGraph(t)
	cf, err := CompileWithSinkWriter(uuid.New(), g, ingest.NewRouter(), &bytes.Buffer{})
	require.NoError(t, err)

	ctx := context.Background()
	require.NoError(t, cf.Start(ctx))

	// Stop must return promptly (all goroutines terminate)
	done := make(chan struct{})
	go func() {
		cf.Stop()
		close(done)
	}()

	select {
	case <-done:
		// passed
	case <-time.After(3 * time.Second):
		t.Fatal("Stop() did not return in time — goroutine leak suspected")
	}
}

func TestCompiledFlow_BusReturnsSameInstance(t *testing.T) {
	g := makeDebugSinkGraph(t)
	cf, err := CompileWithSinkWriter(uuid.New(), g, ingest.NewRouter(), &bytes.Buffer{})
	require.NoError(t, err)
	defer cf.Stop()

	bus1 := cf.Bus()
	bus2 := cf.Bus()
	assert.Same(t, bus1, bus2)
}

func TestCompiledFlow_StopWithoutStartIsNoOp(t *testing.T) {
	g := makeDebugSinkGraph(t)
	cf, err := CompileWithSinkWriter(uuid.New(), g, ingest.NewRouter(), &bytes.Buffer{})
	require.NoError(t, err)

	// Stop before Start should not panic
	assert.NotPanics(t, func() { cf.Stop() })
}

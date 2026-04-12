package graph

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustRaw(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}

func TestValidate_OK(t *testing.T) {
	g := Graph{
		Nodes: []Node{
			{ID: "a", Type: "mqtt_source", Data: mustRaw(t, map[string]any{"broker": "tcp://x:1", "topic": "sensors/#"})},
			{ID: "b", Type: "threshold", Data: mustRaw(t, map[string]any{"min": 0.0, "max": 1.0})},
			{ID: "c", Type: "debug_sink", Data: mustRaw(t, map[string]any{})},
		},
		Edges: []Edge{{ID: "e1", Source: "a", Target: "b"}, {ID: "e2", Source: "b", Target: "c"}},
	}
	assert.NoError(t, Validate(g))
}

func TestValidate_UnknownType(t *testing.T) {
	g := Graph{Nodes: []Node{{ID: "a", Type: "nope", Data: mustRaw(t, map[string]any{})}}}
	assert.ErrorContains(t, Validate(g), "unknown node type")
}

func TestValidate_DanglingEdge(t *testing.T) {
	g := Graph{
		Nodes: []Node{{ID: "a", Type: "debug_sink", Data: mustRaw(t, map[string]any{})}},
		Edges: []Edge{{ID: "e1", Source: "a", Target: "missing"}},
	}
	assert.ErrorContains(t, Validate(g), "edge e1: target")
}

func TestValidate_Cycle(t *testing.T) {
	g := Graph{
		Nodes: []Node{
			{ID: "a", Type: "threshold", Data: mustRaw(t, map[string]any{"min": 0.0, "max": 1.0})},
			{ID: "b", Type: "threshold", Data: mustRaw(t, map[string]any{"min": 0.0, "max": 1.0})},
		},
		Edges: []Edge{{ID: "e1", Source: "a", Target: "b"}, {ID: "e2", Source: "b", Target: "a"}},
	}
	assert.ErrorContains(t, Validate(g), "cycle")
}

func TestValidate_BadConfig(t *testing.T) {
	g := Graph{
		Nodes: []Node{{ID: "a", Type: "threshold", Data: mustRaw(t, map[string]any{"min": 5.0, "max": 1.0})}},
	}
	assert.ErrorContains(t, Validate(g), "min must be < max")
}

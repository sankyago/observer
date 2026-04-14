package flowengine

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestTraverse_ConditionPrunesBranch(t *testing.T) {
	did := uuid.New()
	graphBytes, _ := json.Marshal(Graph{
		Nodes: []Node{
			{ID: "d", Type: "device", Data: mustJSON(map[string]any{"device_id": did.String()})},
			{ID: "c", Type: "condition", Data: mustJSON(map[string]any{"field": "temperature", "op": ">", "value": 80.0})},
			{ID: "a", Type: "action", Data: mustJSON(map[string]any{"kind": "log", "config": map[string]any{}})},
		},
		Edges: []Edge{{Source: "d", Target: "c"}, {Source: "c", Target: "a"}},
	})

	c := NewCache()
	_ = c.Replace([]struct {
		ID       uuid.UUID
		TenantID uuid.UUID
		Graph    []byte
	}{{ID: uuid.New(), TenantID: uuid.New(), Graph: graphBytes}})

	entries := c.GetByDevice(did)
	if len(entries) != 1 {
		t.Fatalf("entries: %d", len(entries))
	}

	// Below threshold → no hits
	hits, err := Traverse(entries[0], []byte(`{"temperature": 70}`))
	if err != nil { t.Fatal(err) }
	if len(hits) != 0 {
		t.Errorf("expected no hits below threshold, got %d", len(hits))
	}

	// Above threshold → one hit
	hits, err = Traverse(entries[0], []byte(`{"temperature": 90}`))
	if err != nil { t.Fatal(err) }
	if len(hits) != 1 || hits[0].Kind != "log" {
		t.Errorf("expected 1 log hit, got %+v", hits)
	}
}

func TestTraverse_ActionChain(t *testing.T) {
	did := uuid.New()
	graphBytes, _ := json.Marshal(Graph{
		Nodes: []Node{
			{ID: "d", Type: "device", Data: mustJSON(map[string]any{"device_id": did.String()})},
			{ID: "a1", Type: "action", Data: mustJSON(map[string]any{"kind": "log", "config": map[string]any{}})},
			{ID: "a2", Type: "action", Data: mustJSON(map[string]any{"kind": "webhook", "config": map[string]any{"url": "http://x"}})},
		},
		Edges: []Edge{{Source: "d", Target: "a1"}, {Source: "a1", Target: "a2"}},
	})
	c := NewCache()
	_ = c.Replace([]struct {
		ID       uuid.UUID
		TenantID uuid.UUID
		Graph    []byte
	}{{ID: uuid.New(), TenantID: uuid.New(), Graph: graphBytes}})
	entries := c.GetByDevice(did)
	hits, err := Traverse(entries[0], []byte(`{}`))
	if err != nil { t.Fatal(err) }
	if len(hits) != 2 {
		t.Errorf("expected 2 hits, got %d", len(hits))
	}
	if hits[0].Kind != "log" || hits[1].Kind != "webhook" {
		t.Errorf("order wrong: %+v", hits)
	}
}

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil { panic(err) }
	return b
}

// Package flowengine parses React Flow graphs and traverses them per incoming
// telemetry message to decide which action nodes to fire.
package flowengine

import (
	"encoding/json"

	"github.com/google/uuid"
)

type Graph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

type Node struct {
	ID   string          `json:"id"`
	Type string          `json:"type"` // "device" | "condition" | "action"
	Data json.RawMessage `json:"data"`
}

type Edge struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type DeviceNodeData struct {
	DeviceID uuid.UUID `json:"device_id"`
}

type ConditionNodeData struct {
	Field string  `json:"field"`
	Op    string  `json:"op"`
	Value float64 `json:"value"`
}

type ActionNodeData struct {
	Kind   string          `json:"kind"`
	Config json.RawMessage `json:"config"`
}

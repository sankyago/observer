package nodes

import (
	"context"
	"time"

	"github.com/sankyago/observer/internal/model"
)

type FlowEvent struct {
	Kind      string               `json:"kind"` // "reading" | "alert" | "error"
	NodeID    string               `json:"node_id"`
	Reading   *model.SensorReading `json:"reading,omitempty"`
	Detail    string               `json:"detail,omitempty"`
	Timestamp time.Time            `json:"ts"`
}

// Node runs until ctx is cancelled. It reads from `in` (nil for sources),
// writes downstream readings to `out` (nil for sinks), and publishes events
// (including alerts) to `events`. Closing `out` on exit signals downstream.
type Node interface {
	ID() string
	Run(ctx context.Context, in <-chan model.SensorReading, out chan<- model.SensorReading, events chan<- FlowEvent) error
}

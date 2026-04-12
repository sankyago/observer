package nodes

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/sankyago/observer/internal/ingest"
	"github.com/sankyago/observer/internal/model"
)

// DeviceSource subscribes to the ingest.Router and forwards matching readings
// to the downstream pipeline.
type DeviceSource struct {
	id     string
	flowID uuid.UUID
	router *ingest.Router
	filter ingest.Filter
}

type deviceSourceCfg struct {
	DeviceID string `json:"device_id"`
	Metric   string `json:"metric"`
}

// NewDeviceSource constructs a DeviceSource. data may be empty JSON object.
func NewDeviceSource(id string, flowID uuid.UUID, router *ingest.Router, data json.RawMessage) (*DeviceSource, error) {
	var cfg deviceSourceCfg
	if len(data) > 0 {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("device_source %s: %w", id, err)
		}
	}
	return &DeviceSource{
		id:     id,
		flowID: flowID,
		router: router,
		filter: ingest.Filter{DeviceID: cfg.DeviceID, Metric: cfg.Metric},
	}, nil
}

// ID returns the node's unique identifier within the flow graph.
func (d *DeviceSource) ID() string { return d.id }

// Run subscribes to the router, forwards matching readings to out, and returns
// when ctx is cancelled.
func (d *DeviceSource) Run(ctx context.Context, _ <-chan model.SensorReading, out chan<- model.SensorReading, _ chan<- FlowEvent) error {
	defer close(out)
	sub := d.router.Subscribe(d.flowID, d.filter, 64)
	defer d.router.Unsubscribe(d.flowID)
	for {
		select {
		case <-ctx.Done():
			return nil
		case r, ok := <-sub:
			if !ok {
				return nil
			}
			select {
			case out <- r:
			case <-ctx.Done():
				return nil
			}
		}
	}
}

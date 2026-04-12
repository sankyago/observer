package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sankyago/observer/internal/engine"
	"github.com/sankyago/observer/internal/model"
)

type Threshold struct {
	id   string
	rule engine.ThresholdRule
}

func NewThreshold(id string, data json.RawMessage) (*Threshold, error) {
	var cfg struct {
		Min float64 `json:"min"`
		Max float64 `json:"max"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("threshold %s: %w", id, err)
	}
	return &Threshold{id: id, rule: engine.ThresholdRule{Min: cfg.Min, Max: cfg.Max}}, nil
}

func (t *Threshold) ID() string { return t.id }

func (t *Threshold) Run(ctx context.Context, in <-chan model.SensorReading, out chan<- model.SensorReading, events chan<- FlowEvent) error {
	defer close(out)
	for {
		select {
		case <-ctx.Done():
			return nil
		case r, ok := <-in:
			if !ok {
				return nil
			}
			if violated, detail := t.rule.Check(r.Value); violated {
				reading := r
				select {
				case events <- FlowEvent{Kind: "alert", NodeID: t.id, Reading: &reading, Detail: detail, Timestamp: time.Now()}:
				case <-ctx.Done():
					return nil
				default:
				}
			}
			select {
			case out <- r:
			case <-ctx.Done():
				return nil
			}
		}
	}
}

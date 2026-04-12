package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/sankyago/observer/internal/engine"
	"github.com/sankyago/observer/internal/model"
)

type RateOfChange struct {
	id      string
	rule    engine.RateRule
	winSize int

	mu      sync.Mutex
	windows map[windowKey]*engine.SlidingWindow
}

type windowKey struct{ device, metric string }

func NewRateOfChange(id string, data json.RawMessage) (*RateOfChange, error) {
	var cfg struct {
		MaxPerSecond float64 `json:"max_per_second"`
		WindowSize   int     `json:"window_size"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("rate_of_change %s: %w", id, err)
	}
	return &RateOfChange{
		id:      id,
		rule:    engine.RateRule{MaxPerSecond: cfg.MaxPerSecond},
		winSize: cfg.WindowSize,
		windows: make(map[windowKey]*engine.SlidingWindow),
	}, nil
}

func (n *RateOfChange) ID() string { return n.id }

func (n *RateOfChange) Run(ctx context.Context, in <-chan model.SensorReading, out chan<- model.SensorReading, events chan<- FlowEvent) error {
	defer close(out)
	for {
		select {
		case <-ctx.Done():
			return nil
		case r, ok := <-in:
			if !ok {
				return nil
			}
			n.check(r, events, ctx)
			select {
			case out <- r:
			case <-ctx.Done():
				return nil
			}
		}
	}
}

func (n *RateOfChange) check(r model.SensorReading, events chan<- FlowEvent, ctx context.Context) {
	n.mu.Lock()
	key := windowKey{r.DeviceID, r.Metric}
	w, ok := n.windows[key]
	if !ok {
		w = engine.NewSlidingWindow(n.winSize)
		n.windows[key] = w
	}
	oldest, duration, ready := w.OldestAndDuration(r.Timestamp)
	w.Push(r.Value, r.Timestamp)
	n.mu.Unlock()

	if !ready {
		return
	}
	if violated, detail := n.rule.Check(oldest, r.Value, duration); violated {
		reading := r
		select {
		case events <- FlowEvent{Kind: "alert", NodeID: n.id, Reading: &reading, Detail: detail, Timestamp: time.Now()}:
		case <-ctx.Done():
		default:
		}
	}
}

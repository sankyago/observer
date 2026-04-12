package ingest

import (
	"sync"

	"github.com/google/uuid"
	"github.com/sankyago/observer/internal/model"
)

// Filter selects readings for a subscription. Empty fields mean "match any".
type Filter struct {
	DeviceID string
	Metric   string
}

func (f Filter) matches(r model.SensorReading) bool {
	if f.DeviceID != "" && f.DeviceID != r.DeviceID {
		return false
	}
	if f.Metric != "" && f.Metric != r.Metric {
		return false
	}
	return true
}

type subscription struct {
	filter Filter
	ch     chan model.SensorReading
}

// Router fans out dispatched readings to subscribed flows.
type Router struct {
	mu   sync.RWMutex
	subs map[uuid.UUID][]*subscription // keyed by flow ID
}

// NewRouter creates a ready-to-use Router.
func NewRouter() *Router {
	return &Router{subs: map[uuid.UUID][]*subscription{}}
}

// Subscribe registers a channel that receives readings matching f.
// The channel has the given buffer size; full channels drop readings.
func (r *Router) Subscribe(flowID uuid.UUID, f Filter, buffer int) chan model.SensorReading {
	ch := make(chan model.SensorReading, buffer)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.subs[flowID] = append(r.subs[flowID], &subscription{filter: f, ch: ch})
	return ch
}

// Unsubscribe removes all subscriptions for flowID and closes their channels.
func (r *Router) Unsubscribe(flowID uuid.UUID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.subs[flowID] {
		close(s.ch)
	}
	delete(r.subs, flowID)
}

// Dispatch sends a reading to every matching subscriber. Drops on full channels.
func (r *Router) Dispatch(reading model.SensorReading) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, subs := range r.subs {
		for _, s := range subs {
			if !s.filter.matches(reading) {
				continue
			}
			select {
			case s.ch <- reading:
			default:
				// drop on full
			}
		}
	}
}

package runtime

import (
	"sync"

	"github.com/sankyago/observer/internal/flow/nodes"
)

type EventBus struct {
	mu     sync.RWMutex
	subs   map[chan nodes.FlowEvent]struct{}
	closed bool
}

func NewEventBus() *EventBus {
	return &EventBus{subs: make(map[chan nodes.FlowEvent]struct{})}
}

func (b *EventBus) Subscribe(buffer int) <-chan nodes.FlowEvent {
	ch := make(chan nodes.FlowEvent, buffer)
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.closed {
		b.subs[ch] = struct{}{}
	} else {
		close(ch)
	}
	return ch
}

func (b *EventBus) Unsubscribe(ch <-chan nodes.FlowEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for k := range b.subs {
		if (<-chan nodes.FlowEvent)(k) == ch {
			delete(b.subs, k)
			close(k)
			return
		}
	}
}

func (b *EventBus) Publish(e nodes.FlowEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subs {
		select {
		case ch <- e:
		default:
			// drop to avoid blocking slow consumers
		}
	}
}

func (b *EventBus) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subs)
}

func (b *EventBus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for ch := range b.subs {
		close(ch)
	}
	b.subs = nil
}

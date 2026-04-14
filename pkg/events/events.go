// Package events provides an in-process publish/subscribe bus used to push
// telemetry-ingested and action-fired events to SSE subscribers.
package events

import (
	"context"
	"sync"
)

type Event struct {
	Type string
	Data []byte
}

type Bus struct {
	bufSize int
	mu      sync.Mutex
	subs    map[chan Event]struct{}
}

func NewBus(perSubscriberBuffer int) *Bus {
	if perSubscriberBuffer < 1 {
		perSubscriberBuffer = 1
	}
	return &Bus{bufSize: perSubscriberBuffer, subs: map[chan Event]struct{}{}}
}

func (b *Bus) Subscribe(ctx context.Context) <-chan Event {
	ch := make(chan Event, b.bufSize)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()

	go func() {
		<-ctx.Done()
		b.mu.Lock()
		delete(b.subs, ch)
		b.mu.Unlock()
		close(ch)
	}()
	return ch
}

func (b *Bus) Publish(e Event) {
	b.mu.Lock()
	for ch := range b.subs {
		select {
		case ch <- e:
		default:
		}
	}
	b.mu.Unlock()
}

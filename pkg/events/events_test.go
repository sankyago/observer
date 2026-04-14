package events

import (
	"context"
	"testing"
	"time"
)

func TestBus_PublishToSubscribers(t *testing.T) {
	b := NewBus(16)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sub1 := b.Subscribe(ctx)
	sub2 := b.Subscribe(ctx)

	b.Publish(Event{Type: "telemetry", Data: []byte(`{"x":1}`)})
	b.Publish(Event{Type: "fired", Data: []byte(`{"y":2}`)})

	for name, sub := range map[string]<-chan Event{"sub1": sub1, "sub2": sub2} {
		for i, want := range []string{"telemetry", "fired"} {
			select {
			case ev := <-sub:
				if ev.Type != want {
					t.Errorf("%s[%d]: got %q want %q", name, i, ev.Type, want)
				}
			case <-time.After(time.Second):
				t.Fatalf("%s: timeout waiting for event %d", name, i)
			}
		}
	}
}

func TestBus_SlowSubscriberDoesNotBlockOthers(t *testing.T) {
	b := NewBus(4)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	slow := b.Subscribe(ctx)
	fast := b.Subscribe(ctx)
	_ = slow // never read

	for i := 0; i < 100; i++ {
		b.Publish(Event{Type: "x"})
	}

	select {
	case <-fast:
	case <-time.After(time.Second):
		t.Fatal("fast subscriber starved by slow one")
	}
}

package runtime

import (
	"testing"
	"time"

	"github.com/sankyago/observer/internal/flow/nodes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventBus_FanOutAndDropOldest(t *testing.T) {
	bus := NewEventBus()
	defer bus.Close()

	sub1 := bus.Subscribe(4)
	sub2 := bus.Subscribe(4)

	for i := 0; i < 3; i++ {
		bus.Publish(nodes.FlowEvent{Kind: "reading", NodeID: "n", Timestamp: time.Now()})
	}

	for i := 0; i < 3; i++ {
		select {
		case <-sub1:
		case <-time.After(time.Second):
			t.Fatalf("sub1 missing event %d", i)
		}
		select {
		case <-sub2:
		case <-time.After(time.Second):
			t.Fatalf("sub2 missing event %d", i)
		}
	}

	// Fill one subscriber and verify publish does not block
	slow := bus.Subscribe(1)
	bus.Publish(nodes.FlowEvent{Kind: "reading"})
	bus.Publish(nodes.FlowEvent{Kind: "reading"}) // second should be dropped (not block)
	assert.Len(t, slow, 1)
}

func TestEventBus_SubscriberReceivesPublishedEvents(t *testing.T) {
	bus := NewEventBus()
	defer bus.Close()

	sub := bus.Subscribe(8)
	events := []nodes.FlowEvent{
		{Kind: "reading", NodeID: "n1"},
		{Kind: "alert", NodeID: "n2"},
		{Kind: "error", NodeID: "n3"},
	}
	for _, e := range events {
		bus.Publish(e)
	}

	for i, want := range events {
		select {
		case got := <-sub:
			assert.Equal(t, want.Kind, got.Kind, "event %d kind", i)
			assert.Equal(t, want.NodeID, got.NodeID, "event %d node_id", i)
		case <-time.After(time.Second):
			t.Fatalf("timeout waiting for event %d", i)
		}
	}
}

func TestEventBus_MultipleSubscribersEachGetEveryEvent(t *testing.T) {
	bus := NewEventBus()
	defer bus.Close()

	const numSubs = 5
	subs := make([]<-chan nodes.FlowEvent, numSubs)
	for i := range subs {
		subs[i] = bus.Subscribe(4)
	}

	evt := nodes.FlowEvent{Kind: "reading", NodeID: "src"}
	bus.Publish(evt)

	for i, sub := range subs {
		select {
		case got := <-sub:
			assert.Equal(t, evt.Kind, got.Kind, "sub %d", i)
		case <-time.After(time.Second):
			t.Fatalf("sub %d did not receive event", i)
		}
	}
}

func TestEventBus_UnsubscribeClosesChannelAndStopsDelivery(t *testing.T) {
	bus := NewEventBus()
	defer bus.Close()

	sub := bus.Subscribe(4)
	bus.Publish(nodes.FlowEvent{Kind: "reading"})

	// Drain the first event
	select {
	case <-sub:
	case <-time.After(time.Second):
		t.Fatal("expected first event")
	}

	bus.Unsubscribe(sub)

	// Channel should be closed
	select {
	case _, ok := <-sub:
		require.False(t, ok, "channel should be closed after Unsubscribe")
	case <-time.After(time.Second):
		t.Fatal("channel not closed after Unsubscribe")
	}

	// Publishing after unsubscribe should not send to the removed channel
	bus.Publish(nodes.FlowEvent{Kind: "reading"})
	// No way to receive — just verify no panic and bus still works
}

func TestEventBus_PublishAfterCloseIsNoOp(t *testing.T) {
	bus := NewEventBus()
	sub := bus.Subscribe(4)
	bus.Close()

	// Should not panic
	assert.NotPanics(t, func() {
		bus.Publish(nodes.FlowEvent{Kind: "reading"})
	})

	// Channel closed by Close()
	select {
	case _, ok := <-sub:
		require.False(t, ok)
	case <-time.After(time.Second):
		t.Fatal("channel not closed")
	}
}

func TestEventBus_SubscribeAfterCloseReturnsClosedChannel(t *testing.T) {
	bus := NewEventBus()
	bus.Close()

	sub := bus.Subscribe(4)
	select {
	case _, ok := <-sub:
		require.False(t, ok, "channel returned after Close should already be closed")
	case <-time.After(time.Second):
		t.Fatal("channel was not closed")
	}
}

func TestEventBus_SlowSubscriberDropsWithoutBlockingOthers(t *testing.T) {
	bus := NewEventBus()
	defer bus.Close()

	// slow subscriber with buffer=1
	slow := bus.Subscribe(1)
	// fast subscriber with large buffer
	fast := bus.Subscribe(16)

	// Publish more events than slow can hold
	for i := 0; i < 8; i++ {
		bus.Publish(nodes.FlowEvent{Kind: "reading"})
	}

	// fast should have all 8
	assert.Len(t, fast, 8)
	// slow should have at most 1 (rest dropped), but publish must not have blocked
	assert.LessOrEqual(t, len(slow), 1)
	_ = slow
}

func TestEventBus_CloseIsIdempotent(t *testing.T) {
	bus := NewEventBus()
	bus.Subscribe(4)

	assert.NotPanics(t, func() {
		bus.Close()
		bus.Close()
		bus.Close()
	})
}

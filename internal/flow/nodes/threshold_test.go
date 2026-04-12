package nodes

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/sankyago/observer/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestThreshold_PassthroughAndAlert(t *testing.T) {
	n, err := NewThreshold("t1", json.RawMessage(`{"min":0,"max":10}`))
	require.NoError(t, err)

	in := make(chan model.SensorReading, 2)
	out := make(chan model.SensorReading, 2)
	events := make(chan FlowEvent, 4)

	in <- model.SensorReading{DeviceID: "d", Metric: "m", Value: 5, Timestamp: time.Now()}
	in <- model.SensorReading{DeviceID: "d", Metric: "m", Value: 99, Timestamp: time.Now()}
	close(in)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, n.Run(ctx, in, out, events))

	// both values pass through
	_, ok1 := <-out
	_, ok2 := <-out
	assert.True(t, ok1)
	assert.True(t, ok2)

	// only the second value produced an alert
	var alerts int
	for len(events) > 0 {
		e := <-events
		if e.Kind == "alert" {
			alerts++
		}
	}
	assert.Equal(t, 1, alerts)
}

// TestThreshold_BoundaryAndAlert checks that boundary values do NOT alert and
// out-of-range values do.
func TestThreshold_BoundaryAndAlert(t *testing.T) {
	tests := []struct {
		name      string
		value     float64
		wantAlert bool
	}{
		{"below min emits alert", -1, true},
		{"at min boundary no alert", 0, false},
		{"at max boundary no alert", 10, false},
		{"above max emits alert", 11, true},
		{"in range no alert", 5, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			n, err := NewThreshold("t", json.RawMessage(`{"min":0,"max":10}`))
			require.NoError(t, err)

			in := make(chan model.SensorReading, 1)
			out := make(chan model.SensorReading, 1)
			events := make(chan FlowEvent, 4)

			in <- model.SensorReading{DeviceID: "d", Metric: "m", Value: tc.value, Timestamp: time.Now()}
			close(in)

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			require.NoError(t, n.Run(ctx, in, out, events))

			var alerts int
			for len(events) > 0 {
				if (<-events).Kind == "alert" {
					alerts++
				}
			}
			if tc.wantAlert {
				assert.Equal(t, 1, alerts, "expected alert for value %v", tc.value)
			} else {
				assert.Equal(t, 0, alerts, "expected no alert for value %v", tc.value)
			}
		})
	}
}

// TestThreshold_CtxCancel verifies that cancelling the context causes Run to
// return without deadlock even when the in channel has no data.
func TestThreshold_CtxCancel(t *testing.T) {
	n, err := NewThreshold("t", json.RawMessage(`{"min":0,"max":10}`))
	require.NoError(t, err)

	in := make(chan model.SensorReading) // unbuffered, will never send
	out := make(chan model.SensorReading, 1)
	events := make(chan FlowEvent, 4)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = n.Run(ctx, in, out, events)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after ctx cancel")
	}
}

// TestThreshold_InChannelClosePropagatesToOut verifies that closing in causes
// out to be closed (range-able).
func TestThreshold_InChannelClosePropagatesToOut(t *testing.T) {
	n, err := NewThreshold("t", json.RawMessage(`{"min":0,"max":10}`))
	require.NoError(t, err)

	in := make(chan model.SensorReading)
	out := make(chan model.SensorReading, 4)
	events := make(chan FlowEvent, 4)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = n.Run(ctx, in, out, events)
	}()

	close(in)
	<-done

	// out must be closed — draining with range should terminate.
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		for range out {
		}
	}()

	select {
	case <-drained:
	case <-time.After(time.Second):
		t.Fatal("out was not closed after in was closed")
	}
}

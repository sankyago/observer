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

func TestRateOfChange_EmitsAlert(t *testing.T) {
	n, err := NewRateOfChange("r1", json.RawMessage(`{"max_per_second":1.0,"window_size":5}`))
	require.NoError(t, err)

	in := make(chan model.SensorReading, 3)
	out := make(chan model.SensorReading, 3)
	events := make(chan FlowEvent, 8)

	t0 := time.Now()
	in <- model.SensorReading{DeviceID: "d", Metric: "m", Value: 10, Timestamp: t0}
	in <- model.SensorReading{DeviceID: "d", Metric: "m", Value: 11, Timestamp: t0.Add(500 * time.Millisecond)}
	in <- model.SensorReading{DeviceID: "d", Metric: "m", Value: 50, Timestamp: t0.Add(1 * time.Second)}
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
	assert.GreaterOrEqual(t, alerts, 1)
}

// TestRateOfChange_MultiDevice verifies that two different (deviceID, metric) pairs
// use independent windows; an alert for one must not affect the other.
func TestRateOfChange_MultiDevice(t *testing.T) {
	n, err := NewRateOfChange("r", json.RawMessage(`{"max_per_second":1.0,"window_size":2}`))
	require.NoError(t, err)

	in := make(chan model.SensorReading, 8)
	out := make(chan model.SensorReading, 8)
	events := make(chan FlowEvent, 16)

	t0 := time.Now()

	// device A: spike — window_size=2, so need 3 readings; 3rd reading sees 2 pushes → ready.
	in <- model.SensorReading{DeviceID: "A", Metric: "m", Value: 0, Timestamp: t0}
	in <- model.SensorReading{DeviceID: "A", Metric: "m", Value: 1, Timestamp: t0.Add(500 * time.Millisecond)}
	in <- model.SensorReading{DeviceID: "A", Metric: "m", Value: 200, Timestamp: t0.Add(time.Second)}

	// device B: stable — should NOT alert
	in <- model.SensorReading{DeviceID: "B", Metric: "m", Value: 5, Timestamp: t0}
	in <- model.SensorReading{DeviceID: "B", Metric: "m", Value: 5, Timestamp: t0.Add(500 * time.Millisecond)}
	in <- model.SensorReading{DeviceID: "B", Metric: "m", Value: 5, Timestamp: t0.Add(time.Second)}

	close(in)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, n.Run(ctx, in, out, events))

	var alertsByDevice = map[string]int{}
	for len(events) > 0 {
		e := <-events
		if e.Kind == "alert" && e.Reading != nil {
			alertsByDevice[e.Reading.DeviceID]++
		}
	}
	assert.GreaterOrEqual(t, alertsByDevice["A"], 1, "expected alert for device A")
	assert.Equal(t, 0, alertsByDevice["B"], "expected no alert for device B")
}

// TestRateOfChange_SingleReadingNoAlert verifies that a single reading with a
// window_size=5 produces no alert (!ready short-circuit).
func TestRateOfChange_SingleReadingNoAlert(t *testing.T) {
	n, err := NewRateOfChange("r", json.RawMessage(`{"max_per_second":0.001,"window_size":5}`))
	require.NoError(t, err)

	in := make(chan model.SensorReading, 1)
	out := make(chan model.SensorReading, 1)
	events := make(chan FlowEvent, 4)

	in <- model.SensorReading{DeviceID: "d", Metric: "m", Value: 999, Timestamp: time.Now()}
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
	assert.Equal(t, 0, alerts, "single reading should produce no alert")
}

// TestRateOfChange_CtxCancel verifies that cancelling the context causes Run to
// return without deadlock.
func TestRateOfChange_CtxCancel(t *testing.T) {
	n, err := NewRateOfChange("r", json.RawMessage(`{"max_per_second":1.0,"window_size":5}`))
	require.NoError(t, err)

	in := make(chan model.SensorReading) // unbuffered, never sends
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

// TestRateOfChange_InChannelClosePropagatesToOut verifies that closing in causes
// out to be closed.
func TestRateOfChange_InChannelClosePropagatesToOut(t *testing.T) {
	n, err := NewRateOfChange("r", json.RawMessage(`{"max_per_second":1.0,"window_size":5}`))
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

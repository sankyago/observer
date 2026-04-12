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

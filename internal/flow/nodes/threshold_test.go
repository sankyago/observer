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

package nodes

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/sankyago/observer/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDebugSink_LogsReading(t *testing.T) {
	var buf bytes.Buffer
	n := NewDebugSink("s1", &buf)

	in := make(chan model.SensorReading, 1)
	events := make(chan FlowEvent, 1)
	in <- model.SensorReading{DeviceID: "d", Metric: "m", Value: 3.14, Timestamp: time.Now()}
	close(in)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, n.Run(ctx, in, nil, events))

	assert.Contains(t, buf.String(), "3.14")
}

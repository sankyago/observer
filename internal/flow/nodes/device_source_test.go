package nodes

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sankyago/observer/internal/ingest"
	"github.com/sankyago/observer/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeviceSource_EmitsSubscribedReadings(t *testing.T) {
	router := ingest.NewRouter()
	flowID := uuid.New()
	dev := uuid.New().String()

	n, err := NewDeviceSource("ds", flowID, router, json.RawMessage(`{"device_id":"`+dev+`"}`))
	require.NoError(t, err)

	out := make(chan model.SensorReading, 4)
	events := make(chan FlowEvent, 1)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_ = n.Run(ctx, nil, out, events)
		close(done)
	}()

	time.Sleep(20 * time.Millisecond) // let Subscribe register

	// This reading matches: same device.
	router.Dispatch(model.SensorReading{DeviceID: dev, Metric: "temp", Value: 1, Timestamp: time.Now()})
	// This reading does NOT match: different device.
	router.Dispatch(model.SensorReading{DeviceID: uuid.New().String(), Metric: "temp", Value: 2, Timestamp: time.Now()})

	select {
	case r := <-out:
		assert.Equal(t, dev, r.DeviceID)
	case <-time.After(time.Second):
		t.Fatal("no reading emitted")
	}

	cancel()
	<-done
	// out should be closed — draining must not block
	for range out {
	}
}

func TestDeviceSource_InvalidJSON(t *testing.T) {
	_, err := NewDeviceSource("ds", uuid.New(), ingest.NewRouter(), json.RawMessage(`not-json`))
	assert.Error(t, err)
}

func TestDeviceSource_EmptyConfig_SubscribesToAll(t *testing.T) {
	router := ingest.NewRouter()
	flowID := uuid.New()
	n, err := NewDeviceSource("ds", flowID, router, json.RawMessage(`{}`))
	require.NoError(t, err)

	out := make(chan model.SensorReading, 4)
	events := make(chan FlowEvent, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = n.Run(ctx, nil, out, events) }()

	time.Sleep(20 * time.Millisecond)

	router.Dispatch(model.SensorReading{DeviceID: uuid.New().String(), Metric: "a", Value: 1, Timestamp: time.Now()})
	router.Dispatch(model.SensorReading{DeviceID: uuid.New().String(), Metric: "b", Value: 2, Timestamp: time.Now()})

	var count int
	for i := 0; i < 2; i++ {
		select {
		case <-out:
			count++
		case <-time.After(500 * time.Millisecond):
		}
	}
	assert.Equal(t, 2, count)
}

func TestDeviceSource_ID(t *testing.T) {
	n, err := NewDeviceSource("my-id", uuid.New(), ingest.NewRouter(), json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Equal(t, "my-id", n.ID())
}

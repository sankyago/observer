package ingest

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sankyago/observer/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func reading(dev, metric string, val float64) model.SensorReading {
	return model.SensorReading{DeviceID: dev, Metric: metric, Value: val, Timestamp: time.Now()}
}

func TestRouter_SubscribeAll(t *testing.T) {
	r := NewRouter()
	flow := uuid.New()
	ch := r.Subscribe(flow, Filter{}, 4)

	dev := uuid.New().String()
	r.Dispatch(reading(dev, "temp", 1))
	r.Dispatch(reading(dev, "humidity", 2))

	require.Len(t, ch, 2)
}

func TestRouter_SubscribeDevice(t *testing.T) {
	r := NewRouter()
	flow := uuid.New()
	target := uuid.New().String()
	ch := r.Subscribe(flow, Filter{DeviceID: target}, 4)

	r.Dispatch(reading(target, "temp", 1))
	r.Dispatch(reading(uuid.New().String(), "temp", 2)) // different device

	require.Len(t, ch, 1)
}

func TestRouter_SubscribeDeviceAndMetric(t *testing.T) {
	r := NewRouter()
	flow := uuid.New()
	dev := uuid.New().String()
	ch := r.Subscribe(flow, Filter{DeviceID: dev, Metric: "temp"}, 4)

	r.Dispatch(reading(dev, "temp", 1))
	r.Dispatch(reading(dev, "humidity", 2)) // wrong metric

	require.Len(t, ch, 1)
}

func TestRouter_Unsubscribe(t *testing.T) {
	r := NewRouter()
	flow := uuid.New()
	ch := r.Subscribe(flow, Filter{}, 4)
	r.Unsubscribe(flow)

	r.Dispatch(reading(uuid.New().String(), "temp", 1))
	assert.Len(t, ch, 0)
	// Channel should be closed.
	_, open := <-ch
	assert.False(t, open)
}

func TestRouter_DropsOnFullChannel(t *testing.T) {
	r := NewRouter()
	flow := uuid.New()
	ch := r.Subscribe(flow, Filter{}, 1)

	dev := uuid.New().String()
	r.Dispatch(reading(dev, "a", 1))
	r.Dispatch(reading(dev, "b", 2)) // dropped, not blocked

	assert.Len(t, ch, 1)
}

func TestRouter_MultipleFlows(t *testing.T) {
	r := NewRouter()
	f1, f2 := uuid.New(), uuid.New()
	ch1 := r.Subscribe(f1, Filter{}, 4)
	ch2 := r.Subscribe(f2, Filter{}, 4)
	r.Dispatch(reading(uuid.New().String(), "t", 1))
	assert.Len(t, ch1, 1)
	assert.Len(t, ch2, 1)
}

package ingest

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestConsumer_HandleMessage_Dispatches(t *testing.T) {
	router := NewRouter()
	flow := uuid.New()
	ch := router.Subscribe(flow, Filter{}, 4)

	c := &Consumer{router: router, now: time.Now}
	dev := uuid.New().String()
	c.handle("v1/devices/"+dev+"/telemetry", []byte(`{"t":1.0}`))

	select {
	case r := <-ch:
		assert.Equal(t, dev, r.DeviceID)
	default:
		t.Fatal("no reading dispatched")
	}
}

func TestConsumer_HandleMessage_BadTopic_DoesNotPanic(t *testing.T) {
	router := NewRouter()
	c := &Consumer{router: router, now: time.Now}
	assert.NotPanics(t, func() { c.handle("garbage", []byte(`{}`)) })
}

func TestConsumer_HandleMessage_BadJSON_DoesNotPanic(t *testing.T) {
	router := NewRouter()
	c := &Consumer{router: router, now: time.Now}
	dev := uuid.New().String()
	assert.NotPanics(t, func() { c.handle("v1/devices/"+dev+"/telemetry", []byte(`{`)) })
}

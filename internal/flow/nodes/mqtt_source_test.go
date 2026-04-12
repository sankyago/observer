package nodes

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMQTTSource_Success(t *testing.T) {
	cfg := json.RawMessage(`{"broker":"tcp://localhost:1883","topic":"sensors/#","username":"","password":""}`)
	src, err := NewMQTTSource("node1", cfg)
	require.NoError(t, err)
	require.NotNil(t, src)
	assert.Equal(t, "node1", src.id)
	assert.Equal(t, "tcp://localhost:1883", src.broker)
	assert.Equal(t, "sensors/#", src.topic)
	assert.Equal(t, "", src.username)
	assert.Equal(t, "", src.password)
}

func TestNewMQTTSource_WithCredentials(t *testing.T) {
	cfg := json.RawMessage(`{"broker":"tcp://broker:1883","topic":"data/#","username":"user1","password":"s3cr3t"}`)
	src, err := NewMQTTSource("cred-node", cfg)
	require.NoError(t, err)
	require.NotNil(t, src)
	assert.Equal(t, "user1", src.username)
	assert.Equal(t, "s3cr3t", src.password)
}

func TestNewMQTTSource_InvalidJSON(t *testing.T) {
	tests := []struct {
		name string
		id   string
		data json.RawMessage
	}{
		{"not json", "node-a", json.RawMessage(`not-json`)},
		{"truncated", "node-b", json.RawMessage(`{"broker":`)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src, err := NewMQTTSource(tc.id, tc.data)
			require.Error(t, err)
			assert.Nil(t, src)
			assert.Contains(t, err.Error(), tc.id)
		})
	}
}

func TestMQTTSource_ID(t *testing.T) {
	cfg := json.RawMessage(`{"broker":"tcp://localhost:1883","topic":"t"}`)
	src, err := NewMQTTSource("my-id", cfg)
	require.NoError(t, err)
	assert.Equal(t, "my-id", src.ID())
}

func TestMQTTSource_emitErr(t *testing.T) {
	cfg := json.RawMessage(`{"broker":"tcp://localhost:1883","topic":"t"}`)
	src, err := NewMQTTSource("err-node", cfg)
	require.NoError(t, err)

	events := make(chan FlowEvent, 1)
	src.emitErr(events, "something went wrong")

	require.Len(t, events, 1)
	ev := <-events
	assert.Equal(t, "error", ev.Kind)
	assert.Equal(t, "err-node", ev.NodeID)
	assert.Equal(t, "something went wrong", ev.Detail)
}

func TestMQTTSource_emitErr_FullChannel(t *testing.T) {
	cfg := json.RawMessage(`{"broker":"tcp://localhost:1883","topic":"t"}`)
	src, err := NewMQTTSource("drop-node", cfg)
	require.NoError(t, err)

	// channel with no capacity — emitErr must not block
	events := make(chan FlowEvent)
	src.emitErr(events, "dropped")
	// if we reach here, the default branch fired correctly
}

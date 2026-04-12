package subscriber

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTopic_Valid(t *testing.T) {
	deviceID, metric, err := ParseTopic("sensors/machine-42/temperature")
	require.NoError(t, err)
	assert.Equal(t, "machine-42", deviceID)
	assert.Equal(t, "temperature", metric)
}

func TestParseTopic_Invalid_TooFewParts(t *testing.T) {
	_, _, err := ParseTopic("sensors/machine-42")
	assert.Error(t, err)
}

func TestParseTopic_Invalid_WrongPrefix(t *testing.T) {
	_, _, err := ParseTopic("other/machine-42/temperature")
	assert.Error(t, err)
}

func TestParsePayload_Valid(t *testing.T) {
	payload := []byte(`{"value": 94.5, "timestamp": "2026-04-12T10:00:01Z"}`)
	value, ts, err := ParsePayload(payload)
	require.NoError(t, err)
	assert.Equal(t, 94.5, value)
	assert.Equal(t, time.Date(2026, 4, 12, 10, 0, 1, 0, time.UTC), ts)
}

func TestParsePayload_Invalid_JSON(t *testing.T) {
	_, _, err := ParsePayload([]byte(`not json`))
	assert.Error(t, err)
}

func TestParsePayload_Missing_Value(t *testing.T) {
	_, _, err := ParsePayload([]byte(`{"timestamp": "2026-04-12T10:00:01Z"}`))
	assert.Error(t, err)
}

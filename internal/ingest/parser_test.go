package ingest

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_HappyPath(t *testing.T) {
	id := uuid.New()
	topic := "v1/devices/" + id.String() + "/telemetry"
	payload := []byte(`{"temperature":23.5,"humidity":60.0,"ts":"2026-04-12T10:00:00Z"}`)

	readings, err := Parse(topic, payload, time.Now)
	require.NoError(t, err)
	require.Len(t, readings, 2)

	for _, r := range readings {
		assert.Equal(t, id.String(), r.DeviceID)
		assert.Equal(t, 2026, r.Timestamp.Year())
	}
}

func TestParse_MissingTimestamp_UsesNow(t *testing.T) {
	id := uuid.New()
	topic := "v1/devices/" + id.String() + "/telemetry"
	payload := []byte(`{"t":1.0}`)
	fixed := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	readings, err := Parse(topic, payload, func() time.Time { return fixed })
	require.NoError(t, err)
	require.Len(t, readings, 1)
	assert.Equal(t, fixed, readings[0].Timestamp)
}

func TestParse_NonNumericValuesDropped(t *testing.T) {
	id := uuid.New()
	topic := "v1/devices/" + id.String() + "/telemetry"
	payload := []byte(`{"temperature":23.5,"status":"ok"}`)
	readings, err := Parse(topic, payload, time.Now)
	require.NoError(t, err)
	require.Len(t, readings, 1)
	assert.Equal(t, "temperature", readings[0].Metric)
}

func TestParse_BadTopic(t *testing.T) {
	_, err := Parse("wrong/topic", []byte(`{}`), time.Now)
	assert.ErrorContains(t, err, "topic")
}

func TestParse_BadUUID(t *testing.T) {
	_, err := Parse("v1/devices/not-a-uuid/telemetry", []byte(`{"t":1}`), time.Now)
	assert.ErrorContains(t, err, "uuid")
}

func TestParse_BadJSON(t *testing.T) {
	id := uuid.New()
	topic := "v1/devices/" + id.String() + "/telemetry"
	_, err := Parse(topic, []byte(`{`), time.Now)
	assert.ErrorContains(t, err, "json")
}

func TestParse_EmptyObject(t *testing.T) {
	id := uuid.New()
	topic := "v1/devices/" + id.String() + "/telemetry"
	readings, err := Parse(topic, []byte(`{}`), time.Now)
	require.NoError(t, err)
	assert.Empty(t, readings)
}

func TestParse_BadTimestamp_ReturnsError(t *testing.T) {
	id := uuid.New()
	topic := "v1/devices/" + id.String() + "/telemetry"
	_, err := Parse(topic, []byte(`{"t":1,"ts":"not-a-date"}`), time.Now)
	assert.ErrorContains(t, err, "ts")
}

func TestParse_TimestampWrongType_ReturnsError(t *testing.T) {
	id := uuid.New()
	topic := "v1/devices/" + id.String() + "/telemetry"
	_, err := Parse(topic, []byte(`{"t":1,"ts":42}`), time.Now)
	assert.ErrorContains(t, err, "ts")
}

func TestParse_ValidTimestamp_IsUsed(t *testing.T) {
	id := uuid.New()
	topic := "v1/devices/" + id.String() + "/telemetry"
	payload := []byte(`{"pressure":101.3,"ts":"2025-06-15T08:30:00Z"}`)
	want := time.Date(2025, 6, 15, 8, 30, 0, 0, time.UTC)

	readings, err := Parse(topic, payload, time.Now)
	require.NoError(t, err)
	require.Len(t, readings, 1)
	assert.Equal(t, want, readings[0].Timestamp)
}

package flusher

import (
	"testing"
	"time"

	"github.com/sankyago/observer/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAggregator_Add_SingleReading(t *testing.T) {
	a := NewAggregator()
	ts := time.Date(2026, 4, 12, 10, 0, 1, 0, time.UTC)

	a.Add(model.SensorReading{
		DeviceID: "m1", Metric: "temperature", Value: 50.0, Timestamp: ts,
	})

	rows := a.Flush(ts)
	require.Len(t, rows, 1)
	assert.Equal(t, "m1", rows[0].DeviceID)
	assert.Equal(t, "temperature", rows[0].Metric)
	assert.Equal(t, 50.0, rows[0].MinValue)
	assert.Equal(t, 50.0, rows[0].MaxValue)
	assert.Equal(t, 50.0, rows[0].AvgValue)
	assert.Equal(t, 1, rows[0].Count)
	assert.Equal(t, 50.0, rows[0].LastValue)
}

func TestAggregator_Add_MultipleReadings(t *testing.T) {
	a := NewAggregator()
	ts := time.Date(2026, 4, 12, 10, 0, 1, 0, time.UTC)

	a.Add(model.SensorReading{DeviceID: "m1", Metric: "temperature", Value: 40.0, Timestamp: ts})
	a.Add(model.SensorReading{DeviceID: "m1", Metric: "temperature", Value: 60.0, Timestamp: ts})
	a.Add(model.SensorReading{DeviceID: "m1", Metric: "temperature", Value: 50.0, Timestamp: ts})

	rows := a.Flush(ts)
	require.Len(t, rows, 1)
	assert.Equal(t, 40.0, rows[0].MinValue)
	assert.Equal(t, 60.0, rows[0].MaxValue)
	assert.Equal(t, 50.0, rows[0].AvgValue)
	assert.Equal(t, 3, rows[0].Count)
	assert.Equal(t, 50.0, rows[0].LastValue)
}

func TestAggregator_Flush_ClearsState(t *testing.T) {
	a := NewAggregator()
	ts := time.Date(2026, 4, 12, 10, 0, 1, 0, time.UTC)

	a.Add(model.SensorReading{DeviceID: "m1", Metric: "temperature", Value: 50.0, Timestamp: ts})
	a.Flush(ts)

	rows := a.Flush(ts.Add(1 * time.Second))
	assert.Len(t, rows, 0)
}

func TestAggregator_Flush_MultipleSensors(t *testing.T) {
	a := NewAggregator()
	ts := time.Date(2026, 4, 12, 10, 0, 1, 0, time.UTC)

	a.Add(model.SensorReading{DeviceID: "m1", Metric: "temperature", Value: 50.0, Timestamp: ts})
	a.Add(model.SensorReading{DeviceID: "m2", Metric: "humidity", Value: 70.0, Timestamp: ts})

	rows := a.Flush(ts)
	assert.Len(t, rows, 2)
}

func TestAggregator_Dedup_SkipsUnchanged(t *testing.T) {
	a := NewAggregator()
	ts := time.Date(2026, 4, 12, 10, 0, 1, 0, time.UTC)

	// First flush — should produce a row
	a.Add(model.SensorReading{DeviceID: "m1", Metric: "temperature", Value: 22.1, Timestamp: ts})
	rows := a.Flush(ts)
	require.Len(t, rows, 1)

	// Second flush — same value, should be skipped
	a.Add(model.SensorReading{DeviceID: "m1", Metric: "temperature", Value: 22.1, Timestamp: ts.Add(1 * time.Second)})
	rows = a.Flush(ts.Add(1 * time.Second))
	assert.Len(t, rows, 0)
}

func TestAggregator_Dedup_WritesOnChange(t *testing.T) {
	a := NewAggregator()
	ts := time.Date(2026, 4, 12, 10, 0, 1, 0, time.UTC)

	a.Add(model.SensorReading{DeviceID: "m1", Metric: "temperature", Value: 22.1, Timestamp: ts})
	a.Flush(ts)

	// Different value — should produce a row
	a.Add(model.SensorReading{DeviceID: "m1", Metric: "temperature", Value: 23.0, Timestamp: ts.Add(1 * time.Second)})
	rows := a.Flush(ts.Add(1 * time.Second))
	assert.Len(t, rows, 1)
}

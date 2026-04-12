package flusher

import (
	"math"
	"time"

	"github.com/sankyago/observer/internal/model"
)

type AggregatedRow struct {
	Time      time.Time
	DeviceID  string
	Metric    string
	MinValue  float64
	MaxValue  float64
	AvgValue  float64
	Count     int
	LastValue float64
}

type bucket struct {
	min   float64
	max   float64
	sum   float64
	count int
	last  float64
}

type sensorKey struct {
	deviceID string
	metric   string
}

type Aggregator struct {
	buckets  map[sensorKey]*bucket
	previous map[sensorKey]float64
}

func NewAggregator() *Aggregator {
	return &Aggregator{
		buckets:  make(map[sensorKey]*bucket),
		previous: make(map[sensorKey]float64),
	}
}

func (a *Aggregator) Add(r model.SensorReading) {
	key := sensorKey{deviceID: r.DeviceID, metric: r.Metric}
	b, ok := a.buckets[key]
	if !ok {
		b = &bucket{min: math.MaxFloat64, max: -math.MaxFloat64}
		a.buckets[key] = b
	}
	if r.Value < b.min {
		b.min = r.Value
	}
	if r.Value > b.max {
		b.max = r.Value
	}
	b.sum += r.Value
	b.count++
	b.last = r.Value
}

func (a *Aggregator) Flush(ts time.Time) []AggregatedRow {
	var rows []AggregatedRow
	for key, b := range a.buckets {
		avg := b.sum / float64(b.count)

		// Dedup: skip if all values identical and same as previous flush
		if b.min == b.max {
			if prev, ok := a.previous[key]; ok && prev == b.last {
				continue
			}
		}

		rows = append(rows, AggregatedRow{
			Time:      ts.Truncate(time.Second),
			DeviceID:  key.deviceID,
			Metric:    key.metric,
			MinValue:  b.min,
			MaxValue:  b.max,
			AvgValue:  avg,
			Count:     b.count,
			LastValue: b.last,
		})
		a.previous[key] = b.last
	}

	a.buckets = make(map[sensorKey]*bucket)
	return rows
}

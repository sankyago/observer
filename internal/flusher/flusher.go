package flusher

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sankyago/observer/internal/model"
)

type FlusherOption func(*Flusher)

func WithFlushInterval(d time.Duration) FlusherOption {
	return func(f *Flusher) { f.interval = d }
}

type Flusher struct {
	pool       *pgxpool.Pool
	in         <-chan model.SensorReading
	aggregator *Aggregator
	interval   time.Duration
}

func NewFlusher(pool *pgxpool.Pool, in <-chan model.SensorReading, opts ...FlusherOption) *Flusher {
	f := &Flusher{
		pool:       pool,
		in:         in,
		aggregator: NewAggregator(),
		interval:   1 * time.Second,
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

func (f *Flusher) Run(ctx context.Context) {
	ticker := time.NewTicker(f.interval)
	defer ticker.Stop()

	for {
		select {
		case reading, ok := <-f.in:
			if !ok {
				f.flush(context.Background())
				return
			}
			f.aggregator.Add(reading)
		case <-ticker.C:
			f.flush(ctx)
		case <-ctx.Done():
			f.flush(context.Background())
			return
		}
	}
}

func (f *Flusher) flush(ctx context.Context) {
	rows := f.aggregator.Flush(time.Now())
	if len(rows) == 0 {
		return
	}

	copyRows := make([][]interface{}, len(rows))
	for i, r := range rows {
		copyRows[i] = []interface{}{r.Time, r.DeviceID, r.Metric, r.MinValue, r.MaxValue, r.AvgValue, r.Count, r.LastValue}
	}

	_, err := f.pool.CopyFrom(
		ctx,
		pgx.Identifier{"sensor_data"},
		[]string{"time", "device_id", "metric", "min_value", "max_value", "avg_value", "count", "last_value"},
		pgx.CopyFromRows(copyRows),
	)
	if err != nil {
		log.Printf("flush error: %v", err)
	}
}

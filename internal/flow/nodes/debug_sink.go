package nodes

import (
	"context"
	"fmt"
	"io"

	"github.com/sankyago/observer/internal/model"
)

type DebugSink struct {
	id  string
	out io.Writer
}

func NewDebugSink(id string, out io.Writer) *DebugSink {
	return &DebugSink{id: id, out: out}
}

func (d *DebugSink) ID() string { return d.id }

func (d *DebugSink) Run(ctx context.Context, in <-chan model.SensorReading, _ chan<- model.SensorReading, _ chan<- FlowEvent) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case r, ok := <-in:
			if !ok {
				return nil
			}
			fmt.Fprintf(d.out, "[DEBUG %s] %s | %s | %s | value=%v\n", d.id, r.Timestamp.Format("2006-01-02T15:04:05Z07:00"), r.DeviceID, r.Metric, r.Value)
		}
	}
}

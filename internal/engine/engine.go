package engine

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/sankyago/observer/internal/model"
)

type alertKey struct {
	deviceID  string
	metric    string
	alertType string
}

type Option func(*Engine)

func WithOutput(w io.Writer) Option {
	return func(e *Engine) { e.output = w }
}

func WithWindowSize(n int) Option {
	return func(e *Engine) { e.windowSize = n }
}

func WithCooldown(d time.Duration) Option {
	return func(e *Engine) { e.cooldown = d }
}

type Engine struct {
	thresholds map[string]ThresholdRule
	rates      map[string]RateRule
	windows    map[string]*SlidingWindow // key: "deviceID:metric"
	lastAlert  map[alertKey]time.Time
	output     io.Writer
	windowSize int
	cooldown   time.Duration
}

func NewEngine(opts ...Option) *Engine {
	thresholds, rates := DefaultRules()
	e := &Engine{
		thresholds: thresholds,
		rates:      rates,
		windows:    make(map[string]*SlidingWindow),
		lastAlert:  make(map[alertKey]time.Time),
		output:     os.Stderr,
		windowSize: 10,
		cooldown:   30 * time.Second,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func (e *Engine) Process(r model.SensorReading) {
	// Threshold check
	if rule, ok := e.thresholds[r.Metric]; ok {
		if violated, msg := rule.Check(r.Value); violated {
			e.emit(r, "THRESHOLD", msg)
		}
	}

	// Rate-of-change check
	windowKey := r.DeviceID + ":" + r.Metric
	w, ok := e.windows[windowKey]
	if !ok {
		w = NewSlidingWindow(e.windowSize)
		e.windows[windowKey] = w
	}

	if rateRule, ok := e.rates[r.Metric]; ok {
		if oldVal, dur, hasData := w.OldestAndDuration(r.Timestamp); hasData {
			if violated, msg := rateRule.Check(oldVal, r.Value, dur); violated {
				e.emit(r, "RATE", msg)
			}
		}
	}

	w.Push(r.Value, r.Timestamp)
}

func (e *Engine) emit(r model.SensorReading, alertType, detail string) {
	key := alertKey{deviceID: r.DeviceID, metric: r.Metric, alertType: alertType}
	if last, ok := e.lastAlert[key]; ok && e.cooldown > 0 {
		if r.Timestamp.Sub(last) < e.cooldown {
			return
		}
	}
	e.lastAlert[key] = r.Timestamp

	fmt.Fprintf(e.output, "[ALERT] %s | %s | %s | %s | %s\n",
		r.Timestamp.Format(time.RFC3339),
		r.DeviceID,
		r.Metric,
		alertType,
		detail,
	)
}

func (e *Engine) Run(in <-chan model.SensorReading, out chan<- model.SensorReading) {
	for reading := range in {
		e.Process(reading)
		out <- reading
	}
	close(out)
}

package engine

import "time"

type windowEntry struct {
	value     float64
	timestamp time.Time
}

type SlidingWindow struct {
	entries []windowEntry
	size    int
	head    int
	count   int
}

func NewSlidingWindow(size int) *SlidingWindow {
	return &SlidingWindow{
		entries: make([]windowEntry, size),
		size:    size,
	}
}

func (w *SlidingWindow) Push(value float64, ts time.Time) {
	w.entries[w.head] = windowEntry{value: value, timestamp: ts}
	w.head = (w.head + 1) % w.size
	if w.count < w.size {
		w.count++
	}
}

func (w *SlidingWindow) OldestAndDuration(now time.Time) (float64, float64, bool) {
	if w.count < 2 {
		return 0, 0, false
	}
	oldest := (w.head - w.count + w.size) % w.size
	entry := w.entries[oldest]
	duration := now.Sub(entry.timestamp).Seconds()
	return entry.value, duration, true
}

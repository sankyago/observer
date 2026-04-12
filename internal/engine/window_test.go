package engine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSlidingWindow_Push_ReturnsOldestAndDuration(t *testing.T) {
	w := NewSlidingWindow(3)
	t0 := time.Date(2026, 4, 12, 10, 0, 0, 0, time.UTC)

	w.Push(10.0, t0)
	w.Push(20.0, t0.Add(1*time.Second))
	w.Push(30.0, t0.Add(2*time.Second))

	oldVal, dur, ok := w.OldestAndDuration(t0.Add(2 * time.Second))
	assert.True(t, ok)
	assert.Equal(t, 10.0, oldVal)
	assert.Equal(t, 2.0, dur)
}

func TestSlidingWindow_Push_EvictsOldest(t *testing.T) {
	w := NewSlidingWindow(2)
	t0 := time.Date(2026, 4, 12, 10, 0, 0, 0, time.UTC)

	w.Push(10.0, t0)
	w.Push(20.0, t0.Add(1*time.Second))
	w.Push(30.0, t0.Add(2*time.Second)) // evicts 10.0

	oldVal, dur, ok := w.OldestAndDuration(t0.Add(2 * time.Second))
	assert.True(t, ok)
	assert.Equal(t, 20.0, oldVal)
	assert.Equal(t, 1.0, dur)
}

func TestSlidingWindow_OldestAndDuration_Empty(t *testing.T) {
	w := NewSlidingWindow(3)
	_, _, ok := w.OldestAndDuration(time.Now())
	assert.False(t, ok)
}

func TestSlidingWindow_OldestAndDuration_SingleElement(t *testing.T) {
	w := NewSlidingWindow(3)
	t0 := time.Date(2026, 4, 12, 10, 0, 0, 0, time.UTC)
	w.Push(10.0, t0)
	_, _, ok := w.OldestAndDuration(t0)
	assert.False(t, ok) // need at least 2 entries to compute rate
}

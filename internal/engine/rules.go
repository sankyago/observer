package engine

import (
	"fmt"
	"math"
)

type ThresholdRule struct {
	Min float64
	Max float64
}

func (r ThresholdRule) Check(value float64) (bool, string) {
	if value > r.Max {
		return true, fmt.Sprintf("value=%.1f (max=%.0f)", value, r.Max)
	}
	if value < r.Min {
		return true, fmt.Sprintf("value=%.1f (min=%.0f)", value, r.Min)
	}
	return false, ""
}

type RateRule struct {
	MaxPerSecond float64
}

func (r RateRule) Check(oldValue, newValue, durationSec float64) (bool, string) {
	if durationSec <= 0 {
		return false, ""
	}
	rate := math.Abs(newValue-oldValue) / durationSec
	if rate > r.MaxPerSecond {
		return true, fmt.Sprintf("delta=%.1f/sec (max=%.1f/sec)", rate, r.MaxPerSecond)
	}
	return false, ""
}

func DefaultRules() (map[string]ThresholdRule, map[string]RateRule) {
	thresholds := map[string]ThresholdRule{
		"temperature": {Min: -20.0, Max: 80.0},
		"humidity":    {Min: 10.0, Max: 90.0},
		"pressure":    {Min: 950.0, Max: 1050.0},
	}
	rates := map[string]RateRule{
		"temperature": {MaxPerSecond: 2.0},
		"humidity":    {MaxPerSecond: 5.0},
		"pressure":    {MaxPerSecond: 10.0},
	}
	return thresholds, rates
}

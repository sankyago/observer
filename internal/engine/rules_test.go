package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestThresholdRule_Check_AboveMax(t *testing.T) {
	rule := ThresholdRule{Min: -20.0, Max: 80.0}
	violated, msg := rule.Check(96.2)
	assert.True(t, violated)
	assert.Contains(t, msg, "96.2")
	assert.Contains(t, msg, "max=80")
}

func TestThresholdRule_Check_BelowMin(t *testing.T) {
	rule := ThresholdRule{Min: -20.0, Max: 80.0}
	violated, msg := rule.Check(-25.0)
	assert.True(t, violated)
	assert.Contains(t, msg, "-25")
	assert.Contains(t, msg, "min=-20")
}

func TestThresholdRule_Check_Normal(t *testing.T) {
	rule := ThresholdRule{Min: -20.0, Max: 80.0}
	violated, msg := rule.Check(50.0)
	assert.False(t, violated)
	assert.Empty(t, msg)
}

func TestRateRule_Check_ExceedsRate(t *testing.T) {
	rule := RateRule{MaxPerSecond: 2.0}
	violated, msg := rule.Check(10.0, 30.0, 5.0)
	assert.True(t, violated)
	assert.Contains(t, msg, "4.0")
	assert.Contains(t, msg, "max=2.0")
}

func TestRateRule_Check_NormalRate(t *testing.T) {
	rule := RateRule{MaxPerSecond: 2.0}
	violated, msg := rule.Check(10.0, 11.0, 5.0)
	assert.False(t, violated)
	assert.Empty(t, msg)
}

func TestRateRule_Check_ZeroDuration(t *testing.T) {
	rule := RateRule{MaxPerSecond: 2.0}
	violated, _ := rule.Check(10.0, 30.0, 0.0)
	assert.False(t, violated)
}

func TestDefaultRules_ContainsExpectedMetrics(t *testing.T) {
	thresholds, rates := DefaultRules()
	assert.Contains(t, thresholds, "temperature")
	assert.Contains(t, thresholds, "humidity")
	assert.Contains(t, thresholds, "pressure")
	assert.Contains(t, rates, "temperature")
	assert.Contains(t, rates, "humidity")
	assert.Contains(t, rates, "pressure")
}

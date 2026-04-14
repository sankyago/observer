package flowengine

import (
	"encoding/json"
)

// EvaluateCondition returns true if the condition matches the numeric value
// of `field` in the JSON payload. Missing or non-numeric fields yield false.
func EvaluateCondition(c ConditionNodeData, payload []byte) (bool, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(payload, &obj); err != nil {
		return false, err
	}
	raw, ok := obj[c.Field]
	if !ok {
		return false, nil
	}
	var v float64
	if err := json.Unmarshal(raw, &v); err != nil {
		return false, nil
	}
	switch c.Op {
	case ">":
		return v > c.Value, nil
	case "<":
		return v < c.Value, nil
	case ">=":
		return v >= c.Value, nil
	case "<=":
		return v <= c.Value, nil
	case "=":
		return v == c.Value, nil
	case "!=":
		return v != c.Value, nil
	}
	return false, nil
}

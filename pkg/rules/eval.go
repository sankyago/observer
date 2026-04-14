// Package rules provides rule caching and threshold evaluation.
package rules

import (
	"encoding/json"

	"github.com/observer-io/observer/pkg/models"
)

// Evaluate reports whether the rule matches the given JSON payload.
// Non-numeric or missing fields never match.
func Evaluate(r models.Rule, payload []byte) (bool, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(payload, &obj); err != nil {
		return false, err
	}
	raw, ok := obj[r.Field]
	if !ok {
		return false, nil
	}
	var v float64
	if err := json.Unmarshal(raw, &v); err != nil {
		return false, nil
	}
	switch r.Op {
	case models.OpGT:
		return v > r.Value, nil
	case models.OpLT:
		return v < r.Value, nil
	case models.OpGTE:
		return v >= r.Value, nil
	case models.OpLTE:
		return v <= r.Value, nil
	case models.OpEQ:
		return v == r.Value, nil
	case models.OpNEQ:
		return v != r.Value, nil
	}
	return false, nil
}

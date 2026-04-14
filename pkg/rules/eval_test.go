package rules

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/observer-io/observer/pkg/models"
)

func mkRule(field string, op models.RuleOp, v float64) models.Rule {
	return models.Rule{
		ID: uuid.New(), TenantID: uuid.New(), DeviceID: uuid.New(),
		Field: field, Op: op, Value: v,
		ActionID: uuid.New(), Enabled: true,
	}
}

func TestEvaluate_Matches(t *testing.T) {
	payload := json.RawMessage(`{"temperature": 85, "battery": 10}`)

	cases := []struct {
		name string
		rule models.Rule
		want bool
	}{
		{"gt true", mkRule("temperature", models.OpGT, 80), true},
		{"gt false", mkRule("temperature", models.OpGT, 100), false},
		{"lt true", mkRule("battery", models.OpLT, 20), true},
		{"eq true", mkRule("temperature", models.OpEQ, 85), true},
		{"neq true", mkRule("temperature", models.OpNEQ, 90), true},
		{"missing field", mkRule("missing", models.OpGT, 0), false},
	}
	for _, tc := range cases {
		got, err := Evaluate(tc.rule, payload)
		if err != nil {
			t.Errorf("%s: err %v", tc.name, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}

func TestEvaluate_NonNumericField_NoMatch(t *testing.T) {
	payload := json.RawMessage(`{"temperature": "hot"}`)
	match, err := Evaluate(mkRule("temperature", models.OpGT, 0), payload)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if match {
		t.Error("non-numeric field should not match")
	}
}

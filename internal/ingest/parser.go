package ingest

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sankyago/observer/internal/model"
)

const timestampKey = "ts"

// Parse extracts sensor readings from an MQTT message. Topic must match
// v1/devices/{uuid}/telemetry. Payload is a JSON object where numeric keys
// become metrics; an optional "ts" key overrides the default now() timestamp.
func Parse(topic string, payload []byte, now func() time.Time) ([]model.SensorReading, error) {
	parts := strings.Split(topic, "/")
	if len(parts) != 4 || parts[0] != "v1" || parts[1] != "devices" || parts[3] != "telemetry" {
		return nil, fmt.Errorf("topic %q does not match v1/devices/{uuid}/telemetry", topic)
	}
	id, err := uuid.Parse(parts[2])
	if err != nil {
		return nil, fmt.Errorf("uuid: %w", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("json: %w", err)
	}

	ts := now()
	if tv, ok := raw[timestampKey]; ok {
		s, isStr := tv.(string)
		if !isStr {
			return nil, fmt.Errorf("ts: expected RFC3339 string, got %T", tv)
		}
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return nil, fmt.Errorf("ts: %w", err)
		}
		ts = t
	}

	readings := make([]model.SensorReading, 0, len(raw))
	for k, v := range raw {
		if k == timestampKey {
			continue
		}
		f, ok := v.(float64)
		if !ok {
			continue
		}
		readings = append(readings, model.SensorReading{
			DeviceID:  id.String(),
			Metric:    k,
			Value:     f,
			Timestamp: ts,
		})
	}
	return readings, nil
}

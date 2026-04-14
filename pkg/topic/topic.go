// Package topic parses Observer MQTT topic names.
package topic

import (
	"errors"
	"strings"

	"github.com/google/uuid"
)

type Telemetry struct {
	TenantSlug string
	DeviceID   uuid.UUID
}

var errBadTopic = errors.New("invalid telemetry topic")

// ParseTelemetry expects: tenants/<slug>/devices/<uuid>/telemetry
func ParseTelemetry(t string) (Telemetry, error) {
	parts := strings.Split(t, "/")
	if len(parts) != 5 {
		return Telemetry{}, errBadTopic
	}
	if parts[0] != "tenants" || parts[2] != "devices" || parts[4] != "telemetry" {
		return Telemetry{}, errBadTopic
	}
	if parts[1] == "" {
		return Telemetry{}, errBadTopic
	}
	id, err := uuid.Parse(parts[3])
	if err != nil {
		return Telemetry{}, errBadTopic
	}
	return Telemetry{TenantSlug: parts[1], DeviceID: id}, nil
}

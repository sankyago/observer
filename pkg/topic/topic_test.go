package topic

import (
	"testing"

	"github.com/google/uuid"
)

func TestParseTelemetry_OK(t *testing.T) {
	did := uuid.New()
	topic := "tenants/acme/devices/" + did.String() + "/telemetry"
	parsed, err := ParseTelemetry(topic)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if parsed.TenantSlug != "acme" {
		t.Errorf("tenant slug: got %q", parsed.TenantSlug)
	}
	if parsed.DeviceID != did {
		t.Errorf("device id: got %s want %s", parsed.DeviceID, did)
	}
}

func TestParseTelemetry_BadShape(t *testing.T) {
	cases := []string{
		"tenants/acme/devices",
		"tenants/acme/devices//telemetry",
		"tenants//devices/" + uuid.New().String() + "/telemetry",
		"wrong/acme/devices/" + uuid.New().String() + "/telemetry",
		"tenants/acme/devices/not-a-uuid/telemetry",
		"tenants/acme/devices/" + uuid.New().String() + "/attributes",
	}
	for _, tc := range cases {
		if _, err := ParseTelemetry(tc); err == nil {
			t.Errorf("expected error for %q", tc)
		}
	}
}

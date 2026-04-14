package config

import (
	"testing"
)

func TestLoad_DefaultsAndOverrides(t *testing.T) {
	t.Setenv("OBSERVER_DB_DSN", "postgres://u:p@localhost:5432/db?sslmode=disable")
	t.Setenv("OBSERVER_MQTT_URL", "tcp://localhost:1883")

	c, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if c.DB.DSN != "postgres://u:p@localhost:5432/db?sslmode=disable" {
		t.Errorf("DB.DSN = %q, want override", c.DB.DSN)
	}
	if c.MQTT.URL != "tcp://localhost:1883" {
		t.Errorf("MQTT.URL = %q, want override", c.MQTT.URL)
	}
	if c.Log.Level != "info" {
		t.Errorf("Log.Level default = %q, want info", c.Log.Level)
	}
}

func TestLoad_MissingDSNReturnsError(t *testing.T) {
	t.Setenv("OBSERVER_DB_DSN", "")
	t.Setenv("OBSERVER_MQTT_URL", "tcp://localhost:1883")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing DB.DSN, got nil")
	}
}

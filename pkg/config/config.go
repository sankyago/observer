// Package config loads runtime configuration from environment variables.
package config

import (
	"fmt"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	DB   DB
	MQTT MQTT
	Log  Log
}

type DB struct {
	DSN string `envconfig:"DSN" required:"true"`
}

type MQTT struct {
	URL string `envconfig:"URL" required:"true"`
}

type Log struct {
	Level string `envconfig:"LEVEL" default:"info"`
}

func Load() (*Config, error) {
	var c Config
	if err := envconfig.Process("OBSERVER", &c); err != nil {
		return nil, err
	}
	if c.DB.DSN == "" {
		return nil, fmt.Errorf("required key DB_DSN missing value")
	}
	if c.MQTT.URL == "" {
		return nil, fmt.Errorf("required key MQTT_URL missing value")
	}
	return &c, nil
}

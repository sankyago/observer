package model

import "time"

type SensorReading struct {
	DeviceID  string
	Metric    string
	Value     float64
	Timestamp time.Time
}

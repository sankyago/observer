package subscriber

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/sankyago/observer/internal/model"
)

type payload struct {
	Value     *float64 `json:"value"`
	Timestamp string   `json:"timestamp"`
}

func ParseTopic(topic string) (string, string, error) {
	parts := strings.Split(topic, "/")
	if len(parts) != 3 || parts[0] != "sensors" {
		return "", "", fmt.Errorf("invalid topic: %s, expected sensors/{device_id}/{metric}", topic)
	}
	return parts[1], parts[2], nil
}

func ParsePayload(data []byte) (float64, time.Time, error) {
	var p payload
	if err := json.Unmarshal(data, &p); err != nil {
		return 0, time.Time{}, fmt.Errorf("invalid JSON: %w", err)
	}
	if p.Value == nil {
		return 0, time.Time{}, fmt.Errorf("missing 'value' field")
	}
	ts, err := time.Parse(time.RFC3339, p.Timestamp)
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("invalid timestamp: %w", err)
	}
	return *p.Value, ts, nil
}

type Subscriber struct {
	client mqtt.Client
	out    chan<- model.SensorReading
	topic  string
}

func New(brokerURL, topic string, out chan<- model.SensorReading) *Subscriber {
	return &Subscriber{
		out:   out,
		topic: topic,
	}
}

func (s *Subscriber) Run(brokerURL string) error {
	opts := mqtt.NewClientOptions().
		AddBroker(brokerURL).
		SetAutoReconnect(true).
		SetClientID("observer").
		SetDefaultPublishHandler(s.handleMessage)

	s.client = mqtt.NewClient(opts)
	if token := s.client.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("mqtt connect: %w", token.Error())
	}

	if token := s.client.Subscribe(s.topic, 1, nil); token.Wait() && token.Error() != nil {
		return fmt.Errorf("mqtt subscribe: %w", token.Error())
	}

	log.Printf("subscribed to %s on %s", s.topic, brokerURL)
	return nil
}

func (s *Subscriber) handleMessage(_ mqtt.Client, msg mqtt.Message) {
	deviceID, metric, err := ParseTopic(msg.Topic())
	if err != nil {
		log.Printf("skip message: %v", err)
		return
	}

	value, ts, err := ParsePayload(msg.Payload())
	if err != nil {
		log.Printf("skip message on %s: %v", msg.Topic(), err)
		return
	}

	s.out <- model.SensorReading{
		DeviceID:  deviceID,
		Metric:    metric,
		Value:     value,
		Timestamp: ts,
	}
}

func (s *Subscriber) Stop() {
	if s.client != nil {
		s.client.Disconnect(250)
	}
}

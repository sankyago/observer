package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/sankyago/observer/internal/model"
	"github.com/sankyago/observer/internal/subscriber"
)

type MQTTSource struct {
	id       string
	broker   string
	topic    string
	username string
	password string
}

func NewMQTTSource(id string, data json.RawMessage) (*MQTTSource, error) {
	var cfg struct {
		Broker   string `json:"broker"`
		Topic    string `json:"topic"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("mqtt_source %s: %w", id, err)
	}
	return &MQTTSource{id: id, broker: cfg.Broker, topic: cfg.Topic, username: cfg.Username, password: cfg.Password}, nil
}

func (m *MQTTSource) ID() string { return m.id }

func (m *MQTTSource) Run(ctx context.Context, _ <-chan model.SensorReading, out chan<- model.SensorReading, events chan<- FlowEvent) error {
	defer close(out)

	opts := mqtt.NewClientOptions().
		AddBroker(m.broker).
		SetClientID(fmt.Sprintf("observer-%s-%d", m.id, time.Now().UnixNano())).
		SetAutoReconnect(true)
	if m.username != "" {
		opts.SetUsername(m.username).SetPassword(m.password)
	}

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("mqtt connect: %w", token.Error())
	}
	defer client.Disconnect(250)

	handler := func(_ mqtt.Client, msg mqtt.Message) {
		device, metric, err := subscriber.ParseTopic(msg.Topic())
		if err != nil {
			m.emitErr(events, err.Error())
			return
		}
		value, ts, err := subscriber.ParsePayload(msg.Payload())
		if err != nil {
			m.emitErr(events, err.Error())
			return
		}
		reading := model.SensorReading{DeviceID: device, Metric: metric, Value: value, Timestamp: ts}
		select {
		case out <- reading:
		case <-ctx.Done():
		}
	}
	if token := client.Subscribe(m.topic, 0, handler); token.Wait() && token.Error() != nil {
		return fmt.Errorf("mqtt subscribe: %w", token.Error())
	}

	<-ctx.Done()
	return nil
}

func (m *MQTTSource) emitErr(events chan<- FlowEvent, msg string) {
	select {
	case events <- FlowEvent{Kind: "error", NodeID: m.id, Detail: msg, Timestamp: time.Now()}:
	default:
	}
}

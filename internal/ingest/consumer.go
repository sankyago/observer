package ingest

import (
	"context"
	"fmt"
	"log"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Config holds EMQX connection parameters for the Consumer.
type Config struct {
	BrokerURL   string
	Username    string
	Password    string
	SharedGroup string // e.g. "observer"
	ClientID    string // distinguishes this replica
}

// Consumer connects to EMQX via a shared subscription and dispatches parsed
// readings to the Router.
type Consumer struct {
	cfg    Config
	router *Router
	client mqtt.Client
	now    func() time.Time
}

// NewConsumer creates a Consumer ready to be started with Run.
func NewConsumer(cfg Config, router *Router) *Consumer {
	return &Consumer{cfg: cfg, router: router, now: time.Now}
}

// Run connects to EMQX, subscribes to the shared topic, and blocks until ctx
// is cancelled.
func (c *Consumer) Run(ctx context.Context) error {
	opts := mqtt.NewClientOptions().
		AddBroker(c.cfg.BrokerURL).
		SetClientID(c.cfg.ClientID).
		SetUsername(c.cfg.Username).
		SetPassword(c.cfg.Password).
		SetAutoReconnect(true).
		SetOnConnectHandler(func(cli mqtt.Client) {
			topic := fmt.Sprintf("$share/%s/v1/devices/+/telemetry", c.cfg.SharedGroup)
			if tok := cli.Subscribe(topic, 0, func(_ mqtt.Client, m mqtt.Message) {
				c.handle(m.Topic(), m.Payload())
			}); tok.Wait() && tok.Error() != nil {
				log.Printf("ingest subscribe: %v", tok.Error())
			}
		})
	c.client = mqtt.NewClient(opts)
	if tok := c.client.Connect(); tok.Wait() && tok.Error() != nil {
		return tok.Error()
	}
	<-ctx.Done()
	c.client.Disconnect(250)
	return nil
}

// handle parses a single MQTT message and dispatches each reading to the router.
func (c *Consumer) handle(topic string, payload []byte) {
	readings, err := Parse(topic, payload, c.now)
	if err != nil {
		log.Printf("ingest parse: %v", err)
		return
	}
	for _, r := range readings {
		c.router.Dispatch(r)
	}
}

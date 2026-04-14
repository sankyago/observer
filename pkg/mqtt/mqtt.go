// Package mqtt wraps paho to subscribe to an MQTT shared subscription and
// deliver messages onto a Go channel.
package mqtt

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
)

type Message struct {
	Topic   string
	Payload []byte
}

type Consumer struct {
	client paho.Client
	logger *slog.Logger
}

type Options struct {
	BrokerURL  string
	ClientID   string
	ShareGroup string
	Topic      string
	Logger     *slog.Logger
}

// Start connects, subscribes to $share/<ShareGroup>/<Topic>, and returns a channel
// that receives messages until the context is cancelled or Stop is called.
func (o Options) Start(ctx context.Context) (*Consumer, <-chan Message, error) {
	ch := make(chan Message, 1024)

	opts := paho.NewClientOptions().
		AddBroker(o.BrokerURL).
		SetClientID(o.ClientID).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(2 * time.Second).
		SetOrderMatters(false)

	opts.OnConnect = func(c paho.Client) {
		shareTopic := fmt.Sprintf("$share/%s/%s", o.ShareGroup, o.Topic)
		tok := c.Subscribe(shareTopic, 0, func(_ paho.Client, m paho.Message) {
			select {
			case ch <- Message{Topic: m.Topic(), Payload: m.Payload()}:
			default:
				o.Logger.Warn("mqtt drop (channel full)", "topic", m.Topic())
			}
		})
		tok.Wait()
		if err := tok.Error(); err != nil {
			o.Logger.Error("mqtt subscribe", "err", err)
		} else {
			o.Logger.Info("mqtt subscribed", "share_topic", shareTopic)
		}
	}
	opts.OnConnectionLost = func(_ paho.Client, err error) {
		o.Logger.Warn("mqtt connection lost", "err", err)
	}

	client := paho.NewClient(opts)
	tok := client.Connect()
	tok.WaitTimeout(10 * time.Second)
	if err := tok.Error(); err != nil {
		close(ch)
		return nil, nil, fmt.Errorf("mqtt connect: %w", err)
	}

	c := &Consumer{client: client, logger: o.Logger}
	go func() {
		<-ctx.Done()
		c.client.Disconnect(500)
		close(ch)
	}()
	return c, ch, nil
}

func (c *Consumer) Stop() {
	c.client.Disconnect(500)
}

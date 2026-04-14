package actions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type WebhookAction struct {
	Client *http.Client
}

type webhookConfig struct {
	URL string `json:"url"`
}

func (a WebhookAction) Run(ctx context.Context, in Input) error {
	var cfg webhookConfig
	if err := json.Unmarshal(in.Action.Config, &cfg); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	if cfg.URL == "" {
		return fmt.Errorf("webhook url is empty")
	}

	body, _ := json.Marshal(map[string]any{
		"rule_id":    in.RuleID,
		"device_id":  in.DeviceID,
		"tenant_id":  in.TenantID,
		"message_id": in.MessageID,
		"payload":    json.RawMessage(in.Payload),
	})

	client := a.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook status %d", resp.StatusCode)
	}
	return nil
}

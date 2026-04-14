package actions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// LinearAction creates an issue in Linear via the GraphQL API.
type LinearAction struct {
	Client *http.Client
}

type linearConfig struct {
	APIKey      string `json:"api_key"`
	TeamID      string `json:"team_id"`
	Title       string `json:"title"`       // template — supports {{device_id}}, {{field}}, {{value}}, {{payload}}
	Description string `json:"description"` // optional template
	Priority    int    `json:"priority"`    // 0 (none) to 4 (low)
}

type linearGraphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

type linearResponse struct {
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
	Data struct {
		IssueCreate struct {
			Success bool `json:"success"`
			Issue   struct {
				ID         string `json:"id"`
				Identifier string `json:"identifier"`
			} `json:"issue"`
		} `json:"issueCreate"`
	} `json:"data"`
}

func (a LinearAction) Run(ctx context.Context, in Input) error {
	var cfg linearConfig
	if err := json.Unmarshal(in.Config, &cfg); err != nil {
		return fmt.Errorf("parse linear config: %w", err)
	}
	if cfg.APIKey == "" {
		return fmt.Errorf("linear api_key is empty")
	}
	if cfg.TeamID == "" {
		return fmt.Errorf("linear team_id is empty")
	}
	if cfg.Title == "" {
		cfg.Title = "Observer alert on device {{device_id}}"
	}

	subs := map[string]string{
		"device_id":  in.DeviceID.String(),
		"flow_id":    in.FlowID.String(),
		"node_id":    in.NodeID,
		"message_id": in.MessageID.String(),
		"payload":    string(in.Payload),
	}
	// Also surface every top-level payload key as a substitution, for convenience.
	var payloadObj map[string]interface{}
	if err := json.Unmarshal(in.Payload, &payloadObj); err == nil {
		for k, v := range payloadObj {
			subs[k] = fmt.Sprintf("%v", v)
		}
	}

	title := applyTemplate(cfg.Title, subs)
	description := applyTemplate(cfg.Description, subs)

	vars := map[string]interface{}{
		"teamId":      cfg.TeamID,
		"title":       title,
		"description": description,
	}
	if cfg.Priority > 0 {
		vars["priority"] = cfg.Priority
	}

	reqBody := linearGraphQLRequest{
		Query: `mutation($teamId: String!, $title: String!, $description: String, $priority: Int) {
			issueCreate(input: { teamId: $teamId, title: $title, description: $description, priority: $priority }) {
				success
				issue { id identifier }
			}
		}`,
		Variables: vars,
	}
	body, _ := json.Marshal(reqBody)

	client := a.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.linear.app/graphql", bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", cfg.APIKey)

	resp, err := client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var parsed linearResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return fmt.Errorf("decode linear response: %w", err)
	}
	if len(parsed.Errors) > 0 {
		msgs := make([]string, 0, len(parsed.Errors))
		for _, e := range parsed.Errors {
			msgs = append(msgs, e.Message)
		}
		return fmt.Errorf("linear api error: %s", strings.Join(msgs, "; "))
	}
	if !parsed.Data.IssueCreate.Success {
		return fmt.Errorf("linear issueCreate returned success=false")
	}
	return nil
}

// applyTemplate replaces {{name}} placeholders with values from subs.
// Unknown placeholders are left as-is.
func applyTemplate(tmpl string, subs map[string]string) string {
	out := tmpl
	for k, v := range subs {
		out = strings.ReplaceAll(out, "{{"+k+"}}", v)
	}
	return out
}

package actions

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/observer-io/observer/pkg/models"
)

func TestWorkflow_RunsNodesInOrder(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(200)
	}))
	defer srv.Close()

	reg := &Registry{
		Log:     LogAction{Logger: slog.New(slog.NewJSONHandler(io.Discard, nil))},
		Webhook: WebhookAction{},
	}
	wf := WorkflowAction{Registry: reg}

	cfg := map[string]any{
		"nodes": []map[string]any{
			{"id": "a", "kind": "log", "config": map[string]any{}},
			{"id": "b", "kind": "webhook", "config": map[string]any{"url": srv.URL}},
		},
		"edges": []map[string]any{{"source": "a", "target": "b"}},
	}
	cfgBytes, _ := json.Marshal(cfg)

	err := wf.Run(context.Background(), Input{
		Action:    models.Action{ID: uuid.New(), Kind: models.ActionWorkflow, Config: cfgBytes},
		DeviceID:  uuid.New(),
		MessageID: uuid.New(),
		Payload:   []byte(`{"temperature":90}`),
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if hits != 1 {
		t.Errorf("webhook called %d times, want 1", hits)
	}
}

func TestWorkflow_DetectsCycle(t *testing.T) {
	wf := WorkflowAction{Registry: &Registry{Log: LogAction{Logger: slog.New(slog.NewJSONHandler(io.Discard, nil))}}}
	cfg := map[string]any{
		"nodes": []map[string]any{{"id": "a", "kind": "log"}, {"id": "b", "kind": "log"}},
		"edges": []map[string]any{{"source": "a", "target": "b"}, {"source": "b", "target": "a"}},
	}
	cfgBytes, _ := json.Marshal(cfg)
	err := wf.Run(context.Background(), Input{
		Action: models.Action{ID: uuid.New(), Kind: models.ActionWorkflow, Config: cfgBytes},
	})
	if err == nil {
		t.Fatal("expected cycle error")
	}
}

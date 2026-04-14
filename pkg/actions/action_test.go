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
)

func TestLogAction_Run(t *testing.T) {
	a := LogAction{Logger: slog.New(slog.NewJSONHandler(io.Discard, nil))}
	err := a.Run(context.Background(), Input{Kind: "log", FlowID: uuid.New(), NodeID: "n1", DeviceID: uuid.New(), MessageID: uuid.New(), Payload: []byte(`{}`)})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
}

func TestWebhookAction_Success(t *testing.T) {
	got := make(chan []byte, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got <- body
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	cfg, _ := json.Marshal(map[string]string{"url": srv.URL})
	a := WebhookAction{}
	if err := a.Run(context.Background(), Input{Kind: "webhook", Config: cfg, FlowID: uuid.New(), NodeID: "n1", DeviceID: uuid.New(), MessageID: uuid.New(), Payload: []byte(`{"x":1}`)}); err != nil {
		t.Fatal(err)
	}
	select {
	case body := <-got:
		if len(body) == 0 { t.Error("empty") }
	default:
		t.Error("not called")
	}
}

func TestWebhookAction_Non2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(500) }))
	defer srv.Close()
	cfg, _ := json.Marshal(map[string]string{"url": srv.URL})
	a := WebhookAction{}
	if err := a.Run(context.Background(), Input{Kind: "webhook", Config: cfg, FlowID: uuid.New(), NodeID: "n1", MessageID: uuid.New(), Payload: []byte(`{}`)}); err == nil {
		t.Error("expected error")
	}
}

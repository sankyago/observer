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

func TestLogAction_Run(t *testing.T) {
	a := LogAction{Logger: slog.New(slog.NewJSONHandler(io.Discard, nil))}
	err := a.Run(context.Background(), Input{
		Action:    models.Action{ID: uuid.New(), Kind: models.ActionLog},
		DeviceID:  uuid.New(),
		MessageID: uuid.New(),
		Payload:   []byte(`{"temperature":90}`),
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
}

func TestWebhookAction_Run_Success(t *testing.T) {
	got := make(chan []byte, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got <- body
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg, _ := json.Marshal(map[string]string{"url": srv.URL})
	a := WebhookAction{}
	err := a.Run(context.Background(), Input{
		Action:    models.Action{ID: uuid.New(), Kind: models.ActionWebhook, Config: cfg},
		DeviceID:  uuid.New(),
		MessageID: uuid.New(),
		Payload:   []byte(`{"temperature":90}`),
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	select {
	case body := <-got:
		if len(body) == 0 {
			t.Error("empty body")
		}
	default:
		t.Error("webhook not called")
	}
}

func TestWebhookAction_Run_Non2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	cfg, _ := json.Marshal(map[string]string{"url": srv.URL})
	a := WebhookAction{}
	err := a.Run(context.Background(), Input{
		Action:    models.Action{ID: uuid.New(), Kind: models.ActionWebhook, Config: cfg},
		DeviceID:  uuid.New(),
		MessageID: uuid.New(),
		Payload:   []byte(`{}`),
	})
	if err == nil {
		t.Error("expected error on 500")
	}
}

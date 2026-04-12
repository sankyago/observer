package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sankyago/observer/internal/flow"
	"github.com/sankyago/observer/internal/web"
)

func NewRouter(svc *flow.Service) http.Handler {
	r := chi.NewRouter()
	r.Get("/api/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	if svc != nil {
		h := &flowsHandler{svc: svc}
		r.Route("/api/flows", func(r chi.Router) {
			r.Get("/", h.list)
			r.Post("/", h.create)
			r.Get("/{id}", h.get)
			r.Put("/{id}", h.update)
			r.Delete("/{id}", h.delete)
			r.Get("/{id}/events", h.events)
		})
	}
	r.Handle("/*", web.Handler())
	return r
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

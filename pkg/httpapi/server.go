// Package httpapi serves the Observer control-plane HTTP API.
package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/observer-io/observer/pkg/events"
)

var DevTenantID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

type Deps struct {
	Pool   *pgxpool.Pool
	Bus    *events.Bus
	Logger *slog.Logger
}

func BuildRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"*"},
		ExposedHeaders: []string{"Link"},
		MaxAge:         300,
	}))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/devices", d.listDevices)
		r.Post("/devices", d.createDevice)
		r.Delete("/devices/{id}", d.deleteDevice)

		r.Get("/actions", d.listActions)
		r.Post("/actions", d.createAction)
		r.Delete("/actions/{id}", d.deleteAction)

		r.Get("/rules", d.listRules)
		r.Post("/rules", d.createRule)
		r.Put("/rules/{id}", d.updateRule)
		r.Delete("/rules/{id}", d.deleteRule)

		r.Get("/telemetry/recent", d.recentTelemetry)
		r.Get("/fired-actions", d.recentFired)

		r.Get("/stream", d.stream)
	})

	return r
}

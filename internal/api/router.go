package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sankyago/observer/internal/devices"
	"github.com/sankyago/observer/internal/flow"
)

// RouterOption configures optional behaviour of NewRouter.
type RouterOption func(*routerConfig)

type routerConfig struct {
	mqttSecret      string
	mqttServiceUser string
}

// WithMQTTAuth enables the /api/mqtt/auth and /api/mqtt/acl endpoints.
// Both secret and serviceUser must be non-empty and a devices service must be
// provided, otherwise the routes are not registered.
func WithMQTTAuth(secret, serviceUser string) RouterOption {
	return func(c *routerConfig) {
		c.mqttSecret = secret
		c.mqttServiceUser = serviceUser
	}
}

func NewRouter(flowSvc *flow.Service, devSvc *devices.Service, opts ...RouterOption) http.Handler {
	cfg := &routerConfig{}
	for _, o := range opts {
		o(cfg)
	}

	r := chi.NewRouter()
	r.Get("/api/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	if flowSvc != nil {
		h := &flowsHandler{svc: flowSvc}
		r.Route("/api/flows", func(r chi.Router) {
			r.Get("/", h.list)
			r.Post("/", h.create)
			r.Get("/{id}", h.get)
			r.Put("/{id}", h.update)
			r.Delete("/{id}", h.delete)
			r.Get("/{id}/events", h.events)
		})
	}

	if devSvc != nil {
		dh := &devicesHandler{svc: devSvc}
		r.Route("/api/devices", func(r chi.Router) {
			r.Get("/", dh.list)
			r.Post("/", dh.create)
			r.Get("/{id}", dh.get)
			r.Put("/{id}", dh.update)
			r.Delete("/{id}", dh.delete)
			r.Post("/{id}/regenerate-token", dh.regenerate)
		})
	}

	if cfg.mqttSecret != "" && devSvc != nil {
		mh := newMQTTAuthHandler(devSvc, cfg.mqttSecret, cfg.mqttServiceUser)
		r.Post("/api/mqtt/auth", mh.auth)
		r.Post("/api/mqtt/acl", mh.acl)
	}

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

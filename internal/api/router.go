package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sankyago/observer/internal/devices"
	"github.com/sankyago/observer/internal/flow"
	"github.com/sankyago/observer/internal/web"
)

// RouterOption configures optional behaviour of NewRouter.
type RouterOption func(*routerConfig)

type routerConfig struct {
	mqttSecret      string
	mqttServiceUser string
	mqttHandler     *mqttAuthHandler
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

// WithMQTTAuthHandler registers a pre-built mqttAuthHandler for the MQTT
// auth/ACL endpoints. Use this when you need a reference to the handler before
// building the router (e.g. to wire its Invalidate method into devices.Service).
func WithMQTTAuthHandler(h *MQTTAuthHandler) RouterOption {
	return func(c *routerConfig) { c.mqttHandler = h.inner }
}

// MQTTAuthHandler is an exported wrapper around the internal mqttAuthHandler
// that lets callers access the Invalidate method for cache wiring.
type MQTTAuthHandler struct{ inner *mqttAuthHandler }

// NewMQTTAuthHandler constructs an MQTTAuthHandler that can be passed to
// WithMQTTAuthHandler and whose Invalidate method can be wired into
// devices.WithTokenInvalidator. svc may be set later with SetService when
// circular construction order requires it.
func NewMQTTAuthHandler(secret, serviceUser string) *MQTTAuthHandler {
	return &MQTTAuthHandler{inner: newMQTTAuthHandler(nil, secret, serviceUser)}
}

// SetService injects the devices.Service after construction.
func (h *MQTTAuthHandler) SetService(svc *devices.Service) { h.inner.svc = svc }

// Invalidate removes token from the in-memory lookup cache.
func (h *MQTTAuthHandler) Invalidate(token string) { h.inner.Invalidate(token) }

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

	var mh *mqttAuthHandler
	if cfg.mqttHandler != nil {
		mh = cfg.mqttHandler
	} else if cfg.mqttSecret != "" && devSvc != nil {
		mh = newMQTTAuthHandler(devSvc, cfg.mqttSecret, cfg.mqttServiceUser)
	}
	if mh != nil {
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

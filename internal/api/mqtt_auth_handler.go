package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sankyago/observer/internal/devices"
	"github.com/sankyago/observer/internal/devices/store"
)

type mqttAuthHandler struct {
	svc         *devices.Service
	secret      string
	serviceUser string

	mu    sync.RWMutex
	cache map[string]cacheEntry // token -> deviceID
}

type cacheEntry struct {
	id        uuid.UUID
	fetchedAt time.Time
}

const authCacheTTL = 30 * time.Second

func newMQTTAuthHandler(svc *devices.Service, secret, serviceUser string) *mqttAuthHandler {
	return &mqttAuthHandler{
		svc:         svc,
		secret:      secret,
		serviceUser: serviceUser,
		cache:       map[string]cacheEntry{},
	}
}

func (h *mqttAuthHandler) checkSecret(r *http.Request) bool {
	return h.secret != "" && r.Header.Get("X-EMQX-Secret") == h.secret
}

type authReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Clientid string `json:"clientid"`
}

func (h *mqttAuthHandler) auth(w http.ResponseWriter, r *http.Request) {
	if !h.checkSecret(r) {
		writeJSON(w, http.StatusOK, map[string]string{"result": "deny"})
		return
	}
	var req authReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"result": "deny"})
		return
	}
	if req.Username == h.serviceUser {
		writeJSON(w, http.StatusOK, map[string]string{"result": "allow"})
		return
	}
	id, ok := h.lookup(r, req.Username)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]string{"result": "deny"})
		return
	}
	_ = id
	writeJSON(w, http.StatusOK, map[string]string{"result": "allow"})
}

type aclReq struct {
	Username string `json:"username"`
	Action   string `json:"action"`
	Topic    string `json:"topic"`
}

func (h *mqttAuthHandler) acl(w http.ResponseWriter, r *http.Request) {
	if !h.checkSecret(r) {
		writeJSON(w, http.StatusOK, map[string]string{"result": "deny"})
		return
	}
	var req aclReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"result": "deny"})
		return
	}

	// Service account: allow subscribe to the telemetry wildcard topic.
	// EMQX strips the "$share/{group}/" prefix before calling the ACL hook, so
	// we must accept the bare topic (e.g. "v1/devices/+/telemetry") as well as
	// the full shared-subscription form (e.g. "$share/observer/v1/devices/+/telemetry").
	if req.Username == h.serviceUser {
		if req.Action == "subscribe" &&
			(strings.HasPrefix(req.Topic, "$share/") || strings.HasPrefix(req.Topic, "v1/devices/")) {
			writeJSON(w, http.StatusOK, map[string]string{"result": "allow"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"result": "deny"})
		return
	}

	// Device: allow publish only to v1/devices/{ownUUID}/telemetry.
	if req.Action != "publish" {
		writeJSON(w, http.StatusOK, map[string]string{"result": "deny"})
		return
	}
	id, ok := h.lookup(r, req.Username)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]string{"result": "deny"})
		return
	}
	expected := "v1/devices/" + id.String() + "/telemetry"
	if req.Topic == expected {
		writeJSON(w, http.StatusOK, map[string]string{"result": "allow"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"result": "deny"})
}

func (h *mqttAuthHandler) lookup(r *http.Request, token string) (uuid.UUID, bool) {
	h.mu.RLock()
	if e, ok := h.cache[token]; ok && time.Since(e.fetchedAt) < authCacheTTL {
		h.mu.RUnlock()
		return e.id, true
	}
	h.mu.RUnlock()

	d, err := h.svc.GetByToken(r.Context(), token)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return uuid.Nil, false
		}
		return uuid.Nil, false
	}
	h.mu.Lock()
	h.cache[token] = cacheEntry{id: d.ID, fetchedAt: time.Now()}
	h.mu.Unlock()
	return d.ID, true
}

// Invalidate clears a specific token from the cache. Called when a device's
// token is rotated or the device is deleted.
func (h *mqttAuthHandler) Invalidate(token string) {
	h.mu.Lock()
	delete(h.cache, token)
	h.mu.Unlock()
}

package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sankyago/observer/internal/flow"
	"github.com/sankyago/observer/internal/flow/graph"
	"github.com/sankyago/observer/internal/flow/store"
)

type flowsHandler struct {
	svc *flow.Service
}

type flowDTO struct {
	ID      uuid.UUID   `json:"id"`
	Name    string      `json:"name"`
	Graph   graph.Graph `json:"graph"`
	Enabled bool        `json:"enabled"`
}

func toDTO(f *store.Flow) flowDTO {
	return flowDTO{ID: f.ID, Name: f.Name, Graph: f.Graph, Enabled: f.Enabled}
}

func (h *flowsHandler) list(w http.ResponseWriter, r *http.Request) {
	flows, err := h.svc.List(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]flowDTO, 0, len(flows))
	for _, f := range flows {
		out = append(out, toDTO(f))
	}
	writeJSON(w, http.StatusOK, out)
}

type createRequest struct {
	Name    string      `json:"name"`
	Graph   graph.Graph `json:"graph"`
	Enabled bool        `json:"enabled"`
}

func (h *flowsHandler) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Name == "" {
		writeErr(w, http.StatusBadRequest, "name required")
		return
	}
	f, err := h.svc.Create(r.Context(), req.Name, req.Graph, req.Enabled)
	if err != nil {
		if errors.Is(err, graph.ErrValidation) {
			writeErr(w, http.StatusBadRequest, err.Error())
		} else {
			writeErr(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, toDTO(f))
}

func (h *flowsHandler) get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	f, err := h.svc.Get(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toDTO(f))
}

type updateRequest struct {
	Name    *string      `json:"name,omitempty"`
	Graph   *graph.Graph `json:"graph,omitempty"`
	Enabled *bool        `json:"enabled,omitempty"`
}

func (h *flowsHandler) update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	f, err := h.svc.Update(r.Context(), id, flow.UpdateRequest{Name: req.Name, Graph: req.Graph, Enabled: req.Enabled})
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		if errors.Is(err, graph.ErrValidation) {
			writeErr(w, http.StatusBadRequest, err.Error())
		} else {
			writeErr(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, toDTO(f))
}

func (h *flowsHandler) delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// events is implemented in events_handler.go
func (h *flowsHandler) events(w http.ResponseWriter, r *http.Request) {
	serveEvents(h.svc, w, r)
}

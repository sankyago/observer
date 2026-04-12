package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sankyago/observer/internal/devices"
	"github.com/sankyago/observer/internal/devices/store"
)

type devicesHandler struct{ svc *devices.Service }

type deviceDTO struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Token string    `json:"token"`
}

func toDeviceDTO(d *store.Device) deviceDTO {
	return deviceDTO{ID: d.ID, Name: d.Name, Token: d.Token}
}

func (h *devicesHandler) list(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.List(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]deviceDTO, 0, len(items))
	for _, d := range items {
		out = append(out, toDeviceDTO(d))
	}
	writeJSON(w, http.StatusOK, out)
}

type createDeviceReq struct {
	Name string `json:"name"`
}

func (h *devicesHandler) create(w http.ResponseWriter, r *http.Request) {
	var req createDeviceReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Name == "" {
		writeErr(w, http.StatusBadRequest, "name required")
		return
	}
	d, err := h.svc.Create(r.Context(), req.Name)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toDeviceDTO(d))
}

func (h *devicesHandler) get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	d, err := h.svc.Get(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toDeviceDTO(d))
}

type updateDeviceReq struct {
	Name string `json:"name"`
}

func (h *devicesHandler) update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req updateDeviceReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Name == "" {
		writeErr(w, http.StatusBadRequest, "name required")
		return
	}
	d, err := h.svc.Rename(r.Context(), id, req.Name)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toDeviceDTO(d))
}

func (h *devicesHandler) delete(w http.ResponseWriter, r *http.Request) {
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

func (h *devicesHandler) regenerate(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	d, err := h.svc.RegenerateToken(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toDeviceDTO(d))
}

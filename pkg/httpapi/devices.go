package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/observer-io/observer/pkg/repo"
)

func (d Deps) listDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := repo.ListDevices(r.Context(), d.Pool, DevTenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, devices)
}

type createDeviceReq struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func (d Deps) createDevice(w http.ResponseWriter, r *http.Request) {
	var in createDeviceReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.Name == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}
	dev, err := repo.CreateDevice(r.Context(), d.Pool, DevTenantID, in.Name, in.Type)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, dev)
}

func (d Deps) deleteDevice(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad id")
		return
	}
	if err := repo.DeleteDevice(r.Context(), d.Pool, DevTenantID, id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

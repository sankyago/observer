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
	if err != nil { writeError(w, 500, err.Error()); return }
	writeJSON(w, 200, devices)
}

func (d Deps) createDevice(w http.ResponseWriter, r *http.Request) {
	var in repo.DeviceInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil { writeError(w, 400, err.Error()); return }
	if in.Name == "" { writeError(w, 400, "name required"); return }
	dev, err := repo.CreateDevice(r.Context(), d.Pool, DevTenantID, in)
	if err != nil { writeError(w, 500, err.Error()); return }
	writeJSON(w, 201, dev)
}

func (d Deps) updateDevice(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil { writeError(w, 400, "bad id"); return }
	var in repo.DeviceInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil { writeError(w, 400, err.Error()); return }
	if in.Name == "" { writeError(w, 400, "name required"); return }
	dev, err := repo.UpdateDevice(r.Context(), d.Pool, DevTenantID, id, in)
	if err != nil { writeError(w, 500, err.Error()); return }
	writeJSON(w, 200, dev)
}

func (d Deps) deleteDevice(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil { writeError(w, 400, "bad id"); return }
	if err := repo.DeleteDevice(r.Context(), d.Pool, DevTenantID, id); err != nil { writeError(w, 500, err.Error()); return }
	w.WriteHeader(204)
}

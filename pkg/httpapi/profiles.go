package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/observer-io/observer/pkg/repo"
)

func (d Deps) listProfiles(w http.ResponseWriter, r *http.Request) {
	list, err := repo.ListProfiles(r.Context(), d.Pool, DevTenantID)
	if err != nil { writeError(w, 500, err.Error()); return }
	writeJSON(w, 200, list)
}

func (d Deps) createProfile(w http.ResponseWriter, r *http.Request) {
	var in repo.DeviceProfileInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil { writeError(w, 400, err.Error()); return }
	if in.Name == "" { writeError(w, 400, "name required"); return }
	p, err := repo.CreateProfile(r.Context(), d.Pool, DevTenantID, in)
	if err != nil { writeError(w, 500, err.Error()); return }
	writeJSON(w, 201, p)
}

func (d Deps) deleteProfile(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil { writeError(w, 400, "bad id"); return }
	if err := repo.DeleteProfile(r.Context(), d.Pool, DevTenantID, id); err != nil { writeError(w, 500, err.Error()); return }
	w.WriteHeader(204)
}

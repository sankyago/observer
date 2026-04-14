package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/observer-io/observer/pkg/repo"
)

func (d Deps) listGroups(w http.ResponseWriter, r *http.Request) {
	list, err := repo.ListGroups(r.Context(), d.Pool, DevTenantID)
	if err != nil { writeError(w, 500, err.Error()); return }
	writeJSON(w, 200, list)
}

func (d Deps) createGroup(w http.ResponseWriter, r *http.Request) {
	var in repo.DeviceGroupInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil { writeError(w, 400, err.Error()); return }
	if in.Name == "" { writeError(w, 400, "name required"); return }
	g, err := repo.CreateGroup(r.Context(), d.Pool, DevTenantID, in)
	if err != nil { writeError(w, 500, err.Error()); return }
	writeJSON(w, 201, g)
}

func (d Deps) deleteGroup(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil { writeError(w, 400, "bad id"); return }
	if err := repo.DeleteGroup(r.Context(), d.Pool, DevTenantID, id); err != nil { writeError(w, 500, err.Error()); return }
	w.WriteHeader(204)
}

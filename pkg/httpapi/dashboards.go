package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/observer-io/observer/pkg/repo"
)

func (d Deps) listDashboards(w http.ResponseWriter, r *http.Request) {
	list, err := repo.ListDashboards(r.Context(), d.Pool, DevTenantID)
	if err != nil { writeError(w, 500, err.Error()); return }
	writeJSON(w, 200, list)
}

func (d Deps) getDashboard(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil { writeError(w, 400, "bad id"); return }
	db, err := repo.GetDashboard(r.Context(), d.Pool, DevTenantID, id)
	if err != nil { writeError(w, 404, err.Error()); return }
	writeJSON(w, 200, db)
}

func (d Deps) createDashboard(w http.ResponseWriter, r *http.Request) {
	var in repo.DashboardInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil { writeError(w, 400, err.Error()); return }
	if in.Name == "" { writeError(w, 400, "name required"); return }
	db, err := repo.CreateDashboard(r.Context(), d.Pool, DevTenantID, in)
	if err != nil { writeError(w, 500, err.Error()); return }
	writeJSON(w, 201, db)
}

func (d Deps) updateDashboard(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil { writeError(w, 400, "bad id"); return }
	var in repo.DashboardInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil { writeError(w, 400, err.Error()); return }
	if in.Name == "" { writeError(w, 400, "name required"); return }
	db, err := repo.UpdateDashboard(r.Context(), d.Pool, DevTenantID, id, in)
	if err != nil { writeError(w, 500, err.Error()); return }
	writeJSON(w, 200, db)
}

func (d Deps) deleteDashboard(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil { writeError(w, 400, "bad id"); return }
	if err := repo.DeleteDashboard(r.Context(), d.Pool, DevTenantID, id); err != nil { writeError(w, 500, err.Error()); return }
	w.WriteHeader(204)
}

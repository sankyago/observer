package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/observer-io/observer/pkg/events"
	"github.com/observer-io/observer/pkg/repo"
)

func (d Deps) listFlows(w http.ResponseWriter, r *http.Request) {
	list, err := repo.ListFlows(r.Context(), d.Pool, DevTenantID)
	if err != nil { writeError(w, 500, err.Error()); return }
	writeJSON(w, 200, list)
}

func (d Deps) getFlow(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil { writeError(w, 400, "bad id"); return }
	f, err := repo.GetFlow(r.Context(), d.Pool, DevTenantID, id)
	if err != nil { writeError(w, 404, err.Error()); return }
	writeJSON(w, 200, f)
}

func (d Deps) createFlow(w http.ResponseWriter, r *http.Request) {
	var in repo.FlowInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil { writeError(w, 400, err.Error()); return }
	if in.Name == "" { writeError(w, 400, "name required"); return }
	f, err := repo.CreateFlow(r.Context(), d.Pool, DevTenantID, in)
	if err != nil { writeError(w, 500, err.Error()); return }
	if d.Bus != nil { d.Bus.Publish(events.Event{Type: "flows_changed", Data: []byte(`{}`)}) }
	writeJSON(w, 201, f)
}

func (d Deps) updateFlow(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil { writeError(w, 400, "bad id"); return }
	var in repo.FlowInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil { writeError(w, 400, err.Error()); return }
	if in.Name == "" { writeError(w, 400, "name required"); return }
	f, err := repo.UpdateFlow(r.Context(), d.Pool, DevTenantID, id, in)
	if err != nil { writeError(w, 500, err.Error()); return }
	if d.Bus != nil { d.Bus.Publish(events.Event{Type: "flows_changed", Data: []byte(`{}`)}) }
	writeJSON(w, 200, f)
}

func (d Deps) deleteFlow(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil { writeError(w, 400, "bad id"); return }
	if err := repo.DeleteFlow(r.Context(), d.Pool, DevTenantID, id); err != nil { writeError(w, 500, err.Error()); return }
	if d.Bus != nil { d.Bus.Publish(events.Event{Type: "flows_changed", Data: []byte(`{}`)}) }
	w.WriteHeader(204)
}

package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/observer-io/observer/pkg/repo"
)

func (d Deps) listActions(w http.ResponseWriter, r *http.Request) {
	list, err := repo.ListActions(r.Context(), d.Pool, DevTenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

type createActionReq struct {
	Kind   string          `json:"kind"`
	Config json.RawMessage `json:"config"`
}

func (d Deps) createAction(w http.ResponseWriter, r *http.Request) {
	var in createActionReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.Kind != "log" && in.Kind != "webhook" && in.Kind != "email" && in.Kind != "workflow" {
		writeError(w, http.StatusBadRequest, "kind must be log|webhook|email|workflow")
		return
	}
	a, err := repo.CreateAction(r.Context(), d.Pool, DevTenantID, in.Kind, in.Config)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, a)
}

func (d Deps) updateAction(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad id")
		return
	}
	var in createActionReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.Kind != "log" && in.Kind != "webhook" && in.Kind != "email" && in.Kind != "workflow" {
		writeError(w, http.StatusBadRequest, "kind must be log|webhook|email|workflow")
		return
	}
	a, err := repo.UpdateAction(r.Context(), d.Pool, DevTenantID, id, in.Kind, in.Config)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func (d Deps) deleteAction(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad id")
		return
	}
	if err := repo.DeleteAction(r.Context(), d.Pool, DevTenantID, id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

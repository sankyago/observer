package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/observer-io/observer/pkg/repo"
)

func (d Deps) listRules(w http.ResponseWriter, r *http.Request) {
	list, err := repo.ListRules(r.Context(), d.Pool, DevTenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (d Deps) createRule(w http.ResponseWriter, r *http.Request) {
	var in repo.RuleInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateRule(in); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	out, err := repo.CreateRule(r.Context(), d.Pool, DevTenantID, in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (d Deps) updateRule(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad id")
		return
	}
	var in repo.RuleInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateRule(in); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	out, err := repo.UpdateRule(r.Context(), d.Pool, DevTenantID, id, in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (d Deps) deleteRule(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad id")
		return
	}
	if err := repo.DeleteRule(r.Context(), d.Pool, DevTenantID, id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func validateRule(in repo.RuleInput) error {
	switch in.Op {
	case ">", "<", ">=", "<=", "=", "!=":
	default:
		return &apiError{"op must be one of > < >= <= = !="}
	}
	if in.Field == "" {
		return &apiError{"field required"}
	}
	return nil
}

type apiError struct{ msg string }

func (e *apiError) Error() string { return e.msg }

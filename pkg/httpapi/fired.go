package httpapi

import (
	"net/http"
	"strconv"

	"github.com/observer-io/observer/pkg/repo"
)

func (d Deps) recentFired(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	rows, err := repo.RecentFired(r.Context(), d.Pool, DevTenantID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

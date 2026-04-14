package httpapi

import (
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/observer-io/observer/pkg/repo"
)

func (d Deps) recentTelemetry(w http.ResponseWriter, r *http.Request) {
	devIDStr := r.URL.Query().Get("device_id")
	deviceID, err := uuid.Parse(devIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "device_id required (uuid)")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	rows, err := repo.RecentTelemetry(r.Context(), d.Pool, DevTenantID, deviceID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

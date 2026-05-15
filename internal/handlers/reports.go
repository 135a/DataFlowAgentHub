package handlers

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/dataflowagenthub/hub/internal/middleware"
	"github.com/go-chi/chi/v5"
)

// DownloadReport allows downloading the generated excel report for a run
func (a *App) DownloadReport(w http.ResponseWriter, r *http.Request) {
	_ = middleware.ClaimsFromContext(r.Context())
	runID := chi.URLParam(r, "runID")

	// Ensure the run belongs to the user/workspace
	var status string
	err := a.DB.QueryRow(r.Context(), `SELECT status FROM runs WHERE id = $1::uuid`, runID).Scan(&status)
	if err != nil {
		errJSON(w, http.StatusNotFound, "run not found")
		return
	}

	if status != "completed" {
		errJSON(w, http.StatusNotFound, "report not ready")
		return
	}

	// MVP: check /tmp/reports
	// In reality, runID could map to reportID or we store the report ID in async_tasks result.
	// We'll just look for runID.xlsx assuming we used runID as report ID in python.
	path := filepath.Join("/tmp/reports", runID+".xlsx")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		errJSON(w, http.StatusNotFound, "report file not found")
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename="+runID+".xlsx")
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	http.ServeFile(w, r, path)
}

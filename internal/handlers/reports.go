package handlers

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	"github.com/dataflowagenthub/hub/internal/middleware"
	"github.com/go-chi/chi/v5"
)

var uuidRE = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// DownloadReport allows downloading the generated excel report for a run
func (a *App) DownloadReport(w http.ResponseWriter, r *http.Request) {
	_ = middleware.ClaimsFromContext(r.Context())
	runID := chi.URLParam(r, "runID")

	// Validate UUID v4 format to prevent path traversal
	if !uuidRE.MatchString(runID) {
		errJSON(w, http.StatusBadRequest, "invalid run id format")
		return
	}

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

	reportsDir := a.Cfg.ReportsDir
	path := filepath.Join(reportsDir, runID+".xlsx")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		errJSON(w, http.StatusNotFound, "report file not found")
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename="+runID+".xlsx")
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	http.ServeFile(w, r, path)
}

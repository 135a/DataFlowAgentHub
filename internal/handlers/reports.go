package handlers

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
)

var uuidRE = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

var reportFormats = map[string]struct {
	ext         string
	contentType string
}{
	"pdf":  {ext: ".pdf", contentType: "application/pdf"},
	"md":   {ext: ".md", contentType: "text/markdown"},
	"docx": {ext: ".docx", contentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
}

// DownloadReport 允许下载指定运行的报告（PDF/MD/DOCX 格式）
// GET /v1/runs/{runID}/report?format=pdf|md|docx
func (a *App) DownloadReport(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runID")

	// 验证 UUID v4 格式以防止路径遍历攻击
	if !uuidRE.MatchString(runID) {
		errJSON(w, http.StatusBadRequest, "invalid run id format")
		return
	}

	// 解析 format 查询参数（默认 pdf）
	format := strings.ToLower(r.URL.Query().Get("format"))
	if format == "" {
		format = "pdf"
	}

	fmtInfo, ok := reportFormats[format]
	if !ok {
		errJSON(w, http.StatusBadRequest, "unsupported format, use: pdf, md, docx")
		return
	}

	// 确保运行属于当前用户/工作区
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
	path := filepath.Join(reportsDir, runID+fmtInfo.ext)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		errJSON(w, http.StatusNotFound, "report file not found")
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename="+runID+fmtInfo.ext)
	w.Header().Set("Content-Type", fmtInfo.contentType)
	http.ServeFile(w, r, path)
}

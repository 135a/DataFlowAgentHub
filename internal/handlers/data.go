package handlers

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/dataflowagenthub/hub/internal/auth"
	"github.com/dataflowagenthub/hub/internal/middleware"
	"github.com/dataflowagenthub/hub/internal/sqlrun"
	"github.com/xuri/excelize/v2"
	"go.uber.org/zap"
)

// 文件导入阈值：超过此行数走异步处理
const importAsyncThreshold = 1000

// uploadForm 上传表单解析结果
type uploadForm struct {
	FileData  []byte
	FileName  string
	DatasetID string // 数据集 ID
	TableID   string // 数据表 ID
	Operation string // insert / update
	AIHint    string
}

// parseResult 文件解析结果
type parseResult struct {
	Columns  []string
	Rows     [][]string
	RowCount int
}

// UploadData 处理文件上传导入数据（操作流程：选择数据集 → 选择数据表 → 上传）
// POST /v1/data/upload
// multipart/form-data: file, dataset_id, table_id, operation, ai_hint
func (a *App) UploadData(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	if c == nil {
		errJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := sqlrun.IsAllowedForRole(sqlrun.SQLTypeInsert, c.Role); err != nil {
		errJSON(w, http.StatusForbidden, "insufficient permissions to import data")
		return
	}

	form, err := parseUploadForm(r)
	if err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	a.uploadToDataset(w, r, form, c)
}

// uploadToDataset 上传到数据集内的指定表（MySQL）
func (a *App) uploadToDataset(w http.ResponseWriter, r *http.Request, form uploadForm, c *auth.Claims) {
	// 检查数据集访问权限
	if !a.hasDatasetAccess(r.Context(), c.UserID, form.DatasetID, c.Role) {
		errJSON(w, http.StatusForbidden, "no access to this dataset")
		return
	}

	// 获取表元数据
	var mysqlDB, mysqlTableName string
	err := a.DB.QueryRow(r.Context(), `
		SELECT d.mysql_database, dt.mysql_table_name
		FROM dataset_tables dt
		JOIN datasets d ON d.id = dt.dataset_id
		WHERE dt.id = $1::uuid AND dt.dataset_id = $2::uuid AND dt.status = 'active'`,
		form.TableID, form.DatasetID).Scan(&mysqlDB, &mysqlTableName)
	if err != nil {
		errJSON(w, http.StatusNotFound, "table not found in dataset")
		return
	}

	// 解析文件
	ext := strings.ToLower(filepath.Ext(form.FileName))
	var parsed parseResult
	switch ext {
	case ".csv":
		parsed, err = parseCSV(form.FileData)
	case ".xlsx":
		parsed, err = parseXLSX(form.FileData)
	default:
		errJSON(w, http.StatusBadRequest, "unsupported file type, only .csv / .xlsx are supported")
		return
	}
	if err != nil {
		errJSON(w, http.StatusBadRequest, "file parse failed: "+err.Error())
		return
	}

	// 从 table_fields 读取目标字段定义
	targetCols, err := a.getDatasetTableFields(r, form.TableID)
	if err != nil {
		errJSON(w, http.StatusBadRequest, "cannot read table fields: "+err.Error())
		return
	}

	// 校验列名
	validation, err := a.validateColumns(r, form, parsed, targetCols)
	if err != nil {
		validation, err = basicColumnCheck(parsed.Columns, targetCols)
		if err != nil {
			errJSON(w, http.StatusBadRequest, "column validation failed: "+err.Error())
			return
		}
	}
	if !validation.OK {
		errJSON(w, http.StatusBadRequest, validation.Error)
		return
	}

	// 获取 MySQL 连接池
	pool, ok := a.MySQLMgr.GetPool(form.DatasetID)
	if !ok {
		pool, err = a.MySQLMgr.Connect(form.DatasetID, mysqlDB)
		if err != nil {
			a.Log.Error("connect mysql", zap.Error(err))
			errJSON(w, http.StatusInternalServerError, "failed to connect to dataset database")
			return
		}
	}

	// 执行 INSERT
	affected, execErr := a.executeMySQLInsert(r.Context(), pool, mysqlTableName, parsed.Columns, parsed.Rows, validation.ColumnMap)
	if execErr != nil {
		errJSON(w, http.StatusInternalServerError, "data import failed: "+execErr.Error())
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"ok":            true,
		"rows_affected": affected,
		"validation":    validation,
	})
}

// parseUploadForm 解析 multipart 上传表单
func parseUploadForm(r *http.Request) (uploadForm, error) {
	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB
		return uploadForm{}, fmt.Errorf("file too large or parse failed")
	}

	f, header, err := r.FormFile("file")
	if err != nil {
		return uploadForm{}, fmt.Errorf("missing file field")
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return uploadForm{}, fmt.Errorf("failed to read file")
	}

	op := strings.TrimSpace(r.FormValue("operation"))
	if op == "" {
		op = "insert"
	}
	if op != "insert" && op != "update" {
		return uploadForm{}, fmt.Errorf("operation must be insert or update")
	}

	return uploadForm{
		FileData:  data,
		FileName:  header.Filename,
		DatasetID: strings.TrimSpace(r.FormValue("dataset_id")),
		TableID:   strings.TrimSpace(r.FormValue("table_id")),
		Operation: op,
		AIHint:    strings.TrimSpace(r.FormValue("ai_hint")),
	}, nil
}

// parseCSV 解析 CSV 为 parseResult
func parseCSV(data []byte) (parseResult, error) {
	r := csv.NewReader(strings.NewReader(string(data)))
	r.LazyQuotes = true
	records, err := r.ReadAll()
	if err != nil {
		return parseResult{}, fmt.Errorf("CSV format error: %w", err)
	}
	if len(records) < 1 {
		return parseResult{}, fmt.Errorf("CSV file is empty")
	}
	cols := records[0]
	var rows [][]string
	if len(records) > 1 {
		rows = records[1:]
	}
	return parseResult{Columns: cols, Rows: rows, RowCount: len(rows)}, nil
}

// parseXLSX 解析 XLSX 第一个 sheet
func parseXLSX(data []byte) (parseResult, error) {
	f, err := excelize.OpenReader(strings.NewReader(string(data)))
	if err != nil {
		return parseResult{}, fmt.Errorf("XLSX parse failed: %w", err)
	}
	defer f.Close()

	sheet := f.GetSheetName(0)
	rows, err := f.GetRows(sheet)
	if err != nil {
		return parseResult{}, fmt.Errorf("read sheet failed: %w", err)
	}
	if len(rows) < 1 {
		return parseResult{}, fmt.Errorf("XLSX file is empty")
	}
	cols := rows[0]
	var dataRows [][]string
	if len(rows) > 1 {
		dataRows = rows[1:]
	}
	return parseResult{Columns: cols, Rows: dataRows, RowCount: len(dataRows)}, nil
}

// getDatasetTableFields 从 table_fields 表读取数据集的字段定义
func (a *App) getDatasetTableFields(r *http.Request, tableID string) (map[string]string, error) {
	rows, err := a.DB.Query(r.Context(), `
		SELECT name, field_type FROM table_fields
		WHERE table_id = $1::uuid
		ORDER BY ordinal_position ASC`, tableID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols := make(map[string]string)
	for rows.Next() {
		var name, ftype string
		if err := rows.Scan(&name, &ftype); err != nil {
			return nil, err
		}
		cols[strings.ToLower(name)] = strings.ToLower(ftype)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate table fields: %w", err)
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("table has no fields defined")
	}
	return cols, nil
}

// executeMySQLInsert 在 MySQL 连接池上执行批量 INSERT
func (a *App) executeMySQLInsert(ctx context.Context, pool *sql.DB, table string, cols []string, rows [][]string, colMap map[string]string) (int64, error) {
	var targetCols []string
	var placeholders []string
	for _, c := range cols {
		mapped, ok := colMap[c]
		if !ok {
			mapped = c
		}
		targetCols = append(targetCols, mapped)
		placeholders = append(placeholders, "?")
	}

	colList := strings.Join(targetCols, ", ")
	phList := strings.Join(placeholders, ", ")

	var totalAffected int64
	for _, row := range rows {
		args := make([]any, len(row))
		for i, v := range row {
			args[i] = v
		}
		sqlStr := fmt.Sprintf("INSERT INTO `%s` (%s) VALUES (%s)", table, colList, phList)
		res, err := pool.ExecContext(ctx, sqlStr, args...)
		if err != nil {
			return totalAffected, fmt.Errorf("row %d insert failed: %w", totalAffected+1, err)
		}
		n, _ := res.RowsAffected()
		totalAffected += n
	}
	return totalAffected, nil
}

// columnValidation AI 校验结果
type columnValidation struct {
	OK        bool              `json:"ok"`
	Error     string            `json:"error,omitempty"`
	ColumnMap map[string]string `json:"column_map"` // 上传列名 → 目标表列名
	Warnings  []string          `json:"warnings,omitempty"`
}

// validateColumns 调用 LLM 校验上传列名与目标表列的匹配
func (a *App) validateColumns(r *http.Request, form uploadForm, parsed parseResult, targetCols map[string]string) (columnValidation, error) {
	if a.LlmClient == nil || a.LlmClient.APIKey == "" {
		return basicColumnCheck(parsed.Columns, targetCols)
	}

	var targetColsList []string
	for name, typ := range targetCols {
		targetColsList = append(targetColsList, name+" ("+typ+")")
	}
	uploads := strings.Join(parsed.Columns, ", ")

	hint := ""
	if form.AIHint != "" {
		hint = "\n用户提示：" + form.AIHint
	}

	prompt := fmt.Sprintf(
		`你是一个数据库列名校验专家。目标表有以下列：%s
上传文件包含以下列名：%s
%s

请判断上传列名能否匹配到目标表的列。同一个意思不同拼写（如 "id" vs "ID"、"用户名" vs "username"）属于正常匹配。
如果是笔误（如拼写错误），请在错误中指出正确列名。

返回 JSON（仅 JSON，无其他文字）：
{"ok": true/false, "error": "错误说明（ok为false时填写）", "column_map": {"上传列名": "目标表列名"}, "warnings": ["注意项"]}`,
		strings.Join(targetColsList, ", "), uploads, hint)

	ctx, cancel := contextWithTimeout(r, 15*time.Second)
	defer cancel()
	resp, err := a.LlmClient.ChatCompletion(ctx, a.Cfg.LLMModel, prompt)
	if err != nil {
		return columnValidation{}, err
	}

	// 提取 JSON
	resp = extractJSON(resp)
	var v columnValidation
	if err := json.Unmarshal([]byte(resp), &v); err != nil {
		return columnValidation{}, fmt.Errorf("AI returned unexpected format: %w", err)
	}
	return v, nil
}

// basicColumnCheck 基本的列名匹配（大小写不敏感）
func basicColumnCheck(uploadCols []string, targetCols map[string]string) (columnValidation, error) {
	cm := make(map[string]string)
	for _, uc := range uploadCols {
		lc := strings.ToLower(strings.TrimSpace(uc))
		if _, ok := targetCols[lc]; ok {
			cm[uc] = lc
		} else {
			return columnValidation{
				OK:    false,
				Error: fmt.Sprintf("column %s not found in target table, available columns: %v", uc, mapKeys(targetCols)),
			}, nil
		}
	}
	return columnValidation{OK: true, ColumnMap: cm}, nil
}

// contextWithTimeout 创建带超时的 context
func contextWithTimeout(r *http.Request, d time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(r.Context(), d)
	return ctx, cancel
}

// extractJSON 从 LLM 响应中提取 JSON 文本
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	// 去掉 ```json 或 ``` 包裹
	if strings.HasPrefix(s, "```") {
		idx := strings.Index(s, "\n")
		if idx > 0 {
			s = s[idx+1:]
		} else {
			s = s[3:]
		}
		if strings.HasSuffix(s, "```") {
			s = s[:len(s)-3]
		}
	}
	return strings.TrimSpace(s)
}

// mapKeys 返回 map 的所有 key
func mapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

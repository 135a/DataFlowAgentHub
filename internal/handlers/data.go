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

	// 获取 MySQL 连接池（SQL 文件和 CSV/XLSX 都需要）
	pool, ok := a.MySQLMgr.GetPool(form.DatasetID)
	if !ok {
		pool, err = a.MySQLMgr.Connect(form.DatasetID, mysqlDB)
		if err != nil {
			a.Log.Error("connect mysql", zap.Error(err))
			errJSON(w, http.StatusInternalServerError, "failed to connect to dataset database")
			return
		}
	}

	// .sql 文件走独立的解析路径
	if ext == ".sql" {
		result := a.executeSQLFile(r.Context(), pool, form.FileData, form.DatasetID)
		status := http.StatusOK
		if !result.OK {
			status = http.StatusOK // 即使有错误也返回 200（部分成功）
		}
		JSON(w, status, result)
		return
	}

	// 以下为 CSV/XLSX 解析路径
	var parsed parseResult
	switch ext {
	case ".csv":
		parsed, err = parseCSV(form.FileData)
	case ".xlsx":
		parsed, err = parseXLSX(form.FileData)
	default:
		errJSON(w, http.StatusBadRequest, "unsupported file type, only .csv / .xlsx / .sql are supported")
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

// ==================== SQL 文件解析（任务 3） ====================

// parseSQL 解析 SQL 文件内容，按分号分割语句，去除注释
func parseSQL(content string) []string {
	// 移除块注释 /* ... */
	cleaned := removeBlockComments(content)

	// 逐行处理，移除行注释 --
	var lines []string
	for _, line := range strings.Split(cleaned, "\n") {
		if idx := strings.Index(line, "--"); idx >= 0 {
			line = line[:idx]
		}
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			lines = append(lines, trimmed)
		}
	}

	// 合并并按分号分割
	joined := strings.Join(lines, " ")
	var statements []string
	for _, stmt := range strings.Split(joined, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt != "" {
			statements = append(statements, stmt)
		}
	}
	return statements
}

// removeBlockComments 移除 SQL 中的 /* */ 块注释
func removeBlockComments(s string) string {
	for {
		start := strings.Index(s, "/*")
		if start == -1 {
			break
		}
		end := strings.Index(s[start+2:], "*/")
		if end == -1 {
			// 未闭合注释，移除从此起到结尾
			s = s[:start]
			break
		}
		s = s[:start] + s[start+end+4:]
	}
	return s
}

// sqlFileResult SQL 文件执行结果
type sqlFileResult struct {
	OK           bool     `json:"ok"`
	RowsAffected int64    `json:"rows_affected"`
	TotalStmts   int      `json:"total_statements"`
	Succeeded    int      `json:"succeeded"`
	Failed       int      `json:"failed"`
	Errors       []string `json:"errors,omitempty"`
}

// validateSQLStatement 校验单条 SQL 语句是否允许在 SQL 文件中执行
// 禁止：SELECT（只读）、DROP/ALTER/TRUNCATE/DELETE（危险操作）
func validateSQLStatement(stmt string) error {
	st := sqlrun.ClassifySQL(stmt)
	switch st {
	case sqlrun.SQLTypeSelect:
		return fmt.Errorf("SELECT is not allowed in SQL files")
	case sqlrun.SQLTypeDrop, sqlrun.SQLTypeAlter, sqlrun.SQLTypeTruncate, sqlrun.SQLTypeDelete:
		return fmt.Errorf("dangerous statement not allowed: %s", st)
	case sqlrun.SQLTypeInsert, sqlrun.SQLTypeUpdate:
		return nil // 允许
	default:
		return fmt.Errorf("unsupported statement type: %s", st)
	}
}

// executeSQLFile 执行 SQL 文件中的所有合法语句，逐条收集结果
func (a *App) executeSQLFile(ctx context.Context, pool *sql.DB, fileData []byte, datasetID string) sqlFileResult {
	statements := parseSQL(string(fileData))
	result := sqlFileResult{
		TotalStmts: len(statements),
	}

	var totalAffected int64
	for i, stmt := range statements {
		if err := validateSQLStatement(stmt); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("statement %d: %s", i+1, err.Error()))
			continue
		}

		affected, err := sqlrun.ExecuteWriteMySQL(ctx, pool, stmt, 30*time.Second)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("statement %d: %s", i+1, err.Error()))
			continue
		}
		totalAffected += affected
		result.Succeeded++
	}

	result.RowsAffected = totalAffected
	result.OK = result.Failed == 0
	return result
}

// ==================== SQL 终端 API（任务 4） ====================

// 关键字白名单 — SQL 终端只允许执行 SELECT/INSERT/UPDATE
var terminalKeywordWhitelist = map[string]bool{
	"SELECT": true, "INSERT": true, "UPDATE": true,
}

// validateSQLTerminal 对 SQL 终端语句执行三层校验
func validateSQLTerminal(sql string) (sqlrun.SQLType, error) {
	st := sqlrun.ClassifySQL(sql)

	// 第一层：关键字白名单 — 只允许 SELECT/INSERT/UPDATE
	if _, ok := terminalKeywordWhitelist[string(st)]; !ok {
		return st, fmt.Errorf("only SELECT/INSERT/UPDATE are allowed, got: %s. DROP/ALTER/TRUNCATE/DELETE are not permitted", st)
	}

	// 第二层：UPDATE 强制 WHERE 条件
	if st == sqlrun.SQLTypeUpdate {
		upper := strings.ToUpper(sql)
		if !strings.Contains(upper, "WHERE") {
			return st, fmt.Errorf("UPDATE must include a WHERE clause for safety")
		}
	}

	return st, nil
}

// explainValidate 使用 MySQL EXPLAIN 做语法验证（不实际执行）
func explainValidate(sql string, pool *sql.DB) error {
	explainSQL := fmt.Sprintf("EXPLAIN %s", sql)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rows, err := pool.QueryContext(ctx, explainSQL)
	if err != nil {
		return err
	}
	rows.Close()
	return nil
}

// getDatasetPool 获取数据集的 MySQL 连接池（自动连接）
func (a *App) getDatasetPool(ctx context.Context, datasetID string) (*sql.DB, error) {
	if pool, ok := a.MySQLMgr.GetPool(datasetID); ok {
		return pool, nil
	}
	var mysqlDB string
	err := a.DB.QueryRow(ctx, `
		SELECT mysql_database FROM datasets
		WHERE id = $1::uuid AND status = 'active'`, datasetID).Scan(&mysqlDB)
	if err != nil {
		return nil, fmt.Errorf("dataset not found or inactive")
	}
	return a.MySQLMgr.Connect(datasetID, mysqlDB)
}

// ExecuteDataSQL SQL 终端执行端点
// POST /v1/data/execute
// Body: {"dataset_id": "...", "sql": "..."}
func (a *App) ExecuteDataSQL(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	if c == nil {
		errJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// 4.6 data_admin+ 权限校验
	if err := sqlrun.IsAllowedForRole(sqlrun.SQLTypeInsert, c.Role); err != nil {
		errJSON(w, http.StatusForbidden, "insufficient permissions to execute SQL")
		return
	}

	var body struct {
		DatasetID string `json:"dataset_id"`
		SQL       string `json:"sql"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid payload")
		return
	}
	body.SQL = strings.TrimSpace(body.SQL)
	body.DatasetID = strings.TrimSpace(body.DatasetID)
	if body.SQL == "" || body.DatasetID == "" {
		errJSON(w, http.StatusBadRequest, "dataset_id and sql are required")
		return
	}

	// 数据集访问校验
	if !a.hasDatasetAccess(r.Context(), c.UserID, body.DatasetID, c.Role) {
		errJSON(w, http.StatusForbidden, "no access to this dataset")
		return
	}

	// 三层校验（关键字白名单 + UPDATE WHERE 检查 + EXPLAIN）
	st, err := validateSQLTerminal(body.SQL)
	if err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	// 获取 MySQL 连接池
	pool, err := a.getDatasetPool(r.Context(), body.DatasetID)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}

	const maxRows = 500
	const queryTimeout = 30 * time.Second
	const writeTimeout = 30 * time.Second

	if st == sqlrun.SQLTypeSelect {
		// MySQL EXPLAIN 语法验证
		if err := explainValidate(body.SQL, pool); err != nil {
			errJSON(w, http.StatusBadRequest, fmt.Sprintf("syntax error: %v", err))
			return
		}

		rows, err := sqlrun.QueryRowsMySQL(r.Context(), pool, body.SQL, maxRows+1, queryTimeout)
		if err != nil {
			errJSON(w, http.StatusBadRequest, fmt.Sprintf("query failed: %s", err.Error()))
			return
		}

		// 提取列名
		var columns []string
		if len(rows) > 0 {
			for k := range rows[0] {
				columns = append(columns, k)
			}
		}

		totalCount := len(rows)
		var displayRows []map[string]any
		truncated := totalCount > maxRows
		if truncated {
			displayRows = rows[:maxRows]
		} else {
			displayRows = rows
		}

		JSON(w, http.StatusOK, map[string]any{
			"ok":          true,
			"type":        "select",
			"columns":     columns,
			"rows":        displayRows,
			"total_count": totalCount,
			"truncated":   truncated,
		})
		return
	}

	// INSERT / UPDATE — 执行写操作
	affected, err := sqlrun.ExecuteWriteMySQL(r.Context(), pool, body.SQL, writeTimeout)
	if err != nil {
		errJSON(w, http.StatusBadRequest, fmt.Sprintf("execute failed: %s", err.Error()))
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"ok":            true,
		"type":          strings.ToLower(string(st)),
		"rows_affected": affected,
	})
}

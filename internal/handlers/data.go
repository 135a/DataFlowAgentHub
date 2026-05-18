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
	FileData    []byte
	FileName    string
	DatasetID   string // 新: 数据集 ID
	TableID     string // 新: 数据表 ID
	TargetTable string // 旧: 目标表名（已弃用）
	Operation   string // insert / update / create_table
	AIHint      string
}

// parseResult 文件解析结果
type parseResult struct {
	Columns  []string
	Rows     [][]string
	RowCount int
	SQLStmts []string // SQL 文件专用
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

	// 新流程：使用 dataset_id + table_id
	if form.DatasetID != "" && form.TableID != "" {
		a.uploadToDataset(w, r, form, c)
		return
	}

	// 旧流程：使用 target_table（向后兼容）
	if form.TargetTable != "" {
		a.uploadToLegacy(w, r, form, c)
		return
	}

	errJSON(w, http.StatusBadRequest, "dataset_id + table_id (new) or target_table (legacy) is required")
}

// uploadToDataset 新流程：上传到数据集内的指定表（MySQL）
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
	case ".sql":
		errJSON(w, http.StatusBadRequest, "SQL file upload is not supported for dataset tables")
		return
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

// uploadToLegacy 旧流程：上传到 PostgreSQL 中的指定表（向后兼容）
func (a *App) uploadToLegacy(w http.ResponseWriter, r *http.Request, form uploadForm, c *auth.Claims) {
	ext := strings.ToLower(filepath.Ext(form.FileName))
	var parsed parseResult
	var err error
	switch ext {
	case ".csv":
		parsed, err = parseCSV(form.FileData)
	case ".xlsx":
		parsed, err = parseXLSX(form.FileData)
	case ".sql":
		parsed, err = parseSQLFile(form.FileData)
	default:
		errJSON(w, http.StatusBadRequest, "unsupported file type, only .csv / .xlsx / .sql are supported")
		return
	}
	if err != nil {
		errJSON(w, http.StatusBadRequest, "file parse failed: "+err.Error())
		return
	}

	if ext == ".sql" {
		a.executeSQLImport(w, r, form, parsed, c.Role)
		return
	}

	targetCols, err := a.getTableColumns(r, form.TargetTable)
	if err != nil {
		errJSON(w, http.StatusBadRequest, "target table not found or cannot read schema: "+err.Error())
		return
	}

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

	switch form.Operation {
	case "insert":
		affected, execErr := a.executeLegacyInsert(r, form.TargetTable, parsed.Columns, parsed.Rows, validation.ColumnMap)
		if execErr != nil {
			errJSON(w, http.StatusInternalServerError, "data import failed: "+execErr.Error())
			return
		}
		JSON(w, http.StatusOK, map[string]any{
			"ok":            true,
			"rows_affected": affected,
			"validation":    validation,
		})
	case "update":
		affected, execErr := a.executeLegacyUpdate(r, form.TargetTable, parsed.Columns, parsed.Rows, validation.ColumnMap)
		if execErr != nil {
			errJSON(w, http.StatusInternalServerError, "data update failed: "+execErr.Error())
			return
		}
		JSON(w, http.StatusOK, map[string]any{
			"ok":            true,
			"rows_affected": affected,
			"validation":    validation,
		})
	default:
		errJSON(w, http.StatusBadRequest, "operation must be insert or update")
	}
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
	if op != "insert" && op != "update" && op != "create_table" {
		return uploadForm{}, fmt.Errorf("operation must be insert/update/create_table")
	}

	return uploadForm{
		FileData:    data,
		FileName:    header.Filename,
		DatasetID:   strings.TrimSpace(r.FormValue("dataset_id")),
		TableID:     strings.TrimSpace(r.FormValue("table_id")),
		TargetTable: strings.TrimSpace(r.FormValue("target_table")),
		Operation:   op,
		AIHint:      strings.TrimSpace(r.FormValue("ai_hint")),
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

// parseSQLFile 按 ; 分割 SQL 语句
func parseSQLFile(data []byte) (parseResult, error) {
	raw := strings.TrimSpace(string(data))
	if raw == "" {
		return parseResult{}, fmt.Errorf("SQL file is empty")
	}
	// 按 ; 分割并过滤空语句
	parts := strings.Split(raw, ";")
	var stmts []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			stmts = append(stmts, p)
		}
	}
	if len(stmts) == 0 {
		return parseResult{}, fmt.Errorf("no valid SQL statements found")
	}
	return parseResult{SQLStmts: stmts, RowCount: len(stmts)}, nil
}

// getTableColumns 获取 PostgreSQL 目标表的列信息（旧流程）
func (a *App) getTableColumns(r *http.Request, tableName string) (map[string]string, error) {
	if sqlrun.IsSystemTable(tableName) {
		return nil, fmt.Errorf("cannot operate on system table %s", tableName)
	}
	rows, err := a.DB.Query(r.Context(), `
		SELECT column_name, data_type FROM information_schema.columns
		WHERE table_schema='public' AND table_name=$1
		ORDER BY ordinal_position`, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols := make(map[string]string)
	for rows.Next() {
		var name, dtype string
		if err := rows.Scan(&name, &dtype); err != nil {
			return nil, err
		}
		cols[strings.ToLower(name)] = dtype
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate table columns: %w", err)
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("table %s does not exist or has no columns", tableName)
	}
	return cols, nil
}

// getDatasetTableFields 从 table_fields 表读取数据集的字段定义（新流程）
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
	for i, c := range cols {
		mapped, ok := colMap[c]
		if !ok {
			mapped = c
		}
		targetCols = append(targetCols, mapped)
		placeholders = append(placeholders, fmt.Sprintf("?", i+1))
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

// executeLegacyInsert 批量 INSERT 数据（旧流程：PostgreSQL）
func (a *App) executeLegacyInsert(r *http.Request, table string, cols []string, rows [][]string, colMap map[string]string) (int64, error) {
	if sqlrun.IsSystemTable(table) {
		return 0, fmt.Errorf("cannot operate on system table %s", table)
	}
	if err := sqlrun.IsAllowedForRole(sqlrun.SQLTypeInsert, middleware.ClaimsFromContext(r.Context()).Role); err != nil {
		return 0, err
	}

	// 构建目标列列表（按上传顺序映射）
	var targetCols []string
	var placeholders []string
	for i, c := range cols {
		mapped, ok := colMap[c]
		if !ok {
			mapped = c
		}
		targetCols = append(targetCols, mapped)
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
	}

	colList := strings.Join(targetCols, ", ")
	phList := strings.Join(placeholders, ", ")

	var totalAffected int64
	for _, row := range rows {
		args := make([]any, len(row))
		for i, v := range row {
			args[i] = v
		}
		sql := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, colList, phList)
		affected, err := sqlrun.ExecuteWrite(r.Context(), a.DB, sql, 30*time.Second)
		if err != nil {
			return totalAffected, fmt.Errorf("row %d insert failed: %w", totalAffected+1, err)
		}
		totalAffected += affected
	}
	return totalAffected, nil
}

// executeLegacyUpdate 批量 UPDATE 数据（旧流程：PostgreSQL）
func (a *App) executeLegacyUpdate(r *http.Request, table string, cols []string, rows [][]string, colMap map[string]string) (int64, error) {
	if sqlrun.IsSystemTable(table) {
		return 0, fmt.Errorf("cannot operate on system table %s", table)
	}
	if err := sqlrun.IsAllowedForRole(sqlrun.SQLTypeUpdate, middleware.ClaimsFromContext(r.Context()).Role); err != nil {
		return 0, err
	}

	if len(cols) < 2 {
		return 0, fmt.Errorf("UPDATE mode requires at least two columns (first column as primary key condition)")
	}

	var totalAffected int64
	for _, row := range rows {
		if len(row) < 2 {
			continue
		}
		var sets []string
		args := make([]any, 0)
		argIdx := 1
		for i := 1; i < len(cols); i++ {
			mapped := colMap[cols[i]]
			if mapped == "" {
				mapped = cols[i]
			}
			sets = append(sets, fmt.Sprintf("%s = $%d", mapped, argIdx))
			argIdx++
			args = append(args, row[i])
		}
		// 第一列作为 WHERE 条件
		pkCol := colMap[cols[0]]
		if pkCol == "" {
			pkCol = cols[0]
		}
		args = append(args, row[0])
		sql := fmt.Sprintf("UPDATE %s SET %s WHERE %s = $%d",
			table, strings.Join(sets, ", "), pkCol, argIdx)
		affected, err := sqlrun.ExecuteWrite(r.Context(), a.DB, sql, 30*time.Second)
		if err != nil {
			return totalAffected, fmt.Errorf("row %d update failed: %w", totalAffected+1, err)
		}
		totalAffected += affected
	}
	return totalAffected, nil
}

// executeSQLImport 执行 SQL 文件中的语句
func (a *App) executeSQLImport(w http.ResponseWriter, r *http.Request, form uploadForm, parsed parseResult, role string) {
	var results []map[string]any
	var totalAffected int64
	for i, stmt := range parsed.SQLStmts {
		sqlType := sqlrun.ClassifySQL(stmt)
		if err := sqlrun.IsAllowedForRole(sqlType, role); err != nil {
			errJSON(w, http.StatusForbidden,
				fmt.Sprintf("statement %d: no permission for %s operation", i+1, sqlType))
			return
		}
		if tbl, ok := sqlrun.CheckSystemTableInSQL(stmt); ok {
			errJSON(w, http.StatusForbidden,
				fmt.Sprintf("statement %d: cannot operate on system table %s", i+1, tbl))
			return
		}
		affected, err := sqlrun.ExecuteWrite(r.Context(), a.DB, stmt, 30*time.Second)
		if err != nil {
			results = append(results, map[string]any{
				"index":  i + 1,
				"sql":    stmt,
				"error":  err.Error(),
				"status": "failed",
			})
			continue
		}
		totalAffected += affected
		results = append(results, map[string]any{
			"index":         i + 1,
			"sql":           stmt,
			"rows_affected": affected,
			"status":        "ok",
		})
	}
	JSON(w, http.StatusOK, map[string]any{
		"ok":             true,
		"total_affected": totalAffected,
		"statements":     len(parsed.SQLStmts),
		"results":        results,
	})
}

// SuggestTable 已禁用（动态建表不再支持）
func (a *App) SuggestTable(w http.ResponseWriter, r *http.Request) {
	errJSON(w, http.StatusGone, "dynamic table creation is disabled. Use the dataset workflow: create a dataset first, then define tables with predefined fields.")
}

// CreateTable 已禁用（动态建表不再支持）
func (a *App) CreateTable(w http.ResponseWriter, r *http.Request) {
	errJSON(w, http.StatusGone, "dynamic table creation is disabled. Use the dataset workflow: create a dataset first, then define tables with predefined fields.")
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

// ListTables 返回 public schema 中所有用户表的表结构，含行数和最后更新时间
// GET /v1/schema/tables
func (a *App) ListTables(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	type colInfo struct {
		Name     string `json:"name"`
		Type     string `json:"type"`
		Nullable bool   `json:"nullable"`
	}
	type tblInfo struct {
		Name       string    `json:"name"`
		Columns    []colInfo `json:"columns"`
		RowEst     int64     `json:"row_estimate"`
		LastVacuum *string   `json:"last_vacuum,omitempty"`
	}

	rows, err := a.DB.Query(ctx, `
		SELECT
			c.table_name,
			c.column_name,
			c.data_type,
			c.is_nullable,
			COALESCE(s.n_live_tup, 0) AS row_estimate,
			s.last_vacuum
		FROM information_schema.columns c
		LEFT JOIN pg_stat_user_tables s ON c.table_name = s.relname AND c.table_schema = s.schemaname
		WHERE c.table_schema = 'public'
		ORDER BY c.table_name, c.ordinal_position
	`)
	if err != nil {
		a.Log.Error("list tables query", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "failed to query table schema")
		return
	}
	defer rows.Close()

	tableMap := make(map[string]*tblInfo)
	var tableOrder []string
	for rows.Next() {
		var tName, cName, cType, nullable string
		var rowEst int64
		var lastVac *string
		if err := rows.Scan(&tName, &cName, &cType, &nullable, &rowEst, &lastVac); err != nil {
			errJSON(w, http.StatusInternalServerError, "failed to read table schema")
			return
		}
		if sqlrun.IsSystemTable(tName) {
			continue
		}
		t, ok := tableMap[tName]
		if !ok {
			tableOrder = append(tableOrder, tName)
			t = &tblInfo{Name: tName, RowEst: rowEst}
			if lastVac != nil {
				t.LastVacuum = lastVac
			}
			tableMap[tName] = t
		}
		t.Columns = append(t.Columns, colInfo{
			Name:     cName,
			Type:     cType,
			Nullable: nullable == "YES",
		})
		if rowEst > t.RowEst {
			t.RowEst = rowEst
		}
		if lastVac != nil && (t.LastVacuum == nil || *lastVac > *t.LastVacuum) {
			t.LastVacuum = lastVac
		}
	}
	if err := rows.Err(); err != nil {
		errJSON(w, http.StatusInternalServerError, "failed to iterate table schema")
		return
	}

	tables := make([]tblInfo, 0, len(tableOrder))
	for _, name := range tableOrder {
		tables = append(tables, *tableMap[name])
	}
	JSON(w, http.StatusOK, map[string]any{"tables": tables})
}

// mapKeys 返回 map 的所有 key
func mapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

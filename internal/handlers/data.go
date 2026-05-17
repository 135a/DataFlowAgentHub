package handlers

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

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
	TargetTable string
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

// UploadData 处理文件上传导入数据（operator+）
// POST /v1/data/upload
// multipart/form-data: file, target_table, operation, ai_hint
func (a *App) UploadData(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	if c == nil {
		errJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// 仅 operator+ 可使用
	if err := sqlrun.IsAllowedForRole(sqlrun.SQLTypeInsert, c.Role); err != nil {
		errJSON(w, http.StatusForbidden, "仅 operator 及以上角色可导入数据")
		return
	}

	form, err := parseUploadForm(r)
	if err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	// 解析文件内容
	ext := strings.ToLower(filepath.Ext(form.FileName))
	var parsed parseResult
	switch ext {
	case ".csv":
		parsed, err = parseCSV(form.FileData)
	case ".xlsx":
		parsed, err = parseXLSX(form.FileData)
	case ".sql":
		parsed, err = parseSQLFile(form.FileData)
	default:
		errJSON(w, http.StatusBadRequest, "不支持的文件类型，仅支持 .csv / .xlsx / .sql")
		return
	}
	if err != nil {
		errJSON(w, http.StatusBadRequest, "文件解析失败: "+err.Error())
		return
	}

	// SQL 文件走独立的执行路径
	if ext == ".sql" {
		a.executeSQLImport(w, r, form, parsed, c.Role)
		return
	}

	// 获取目标表列结构
	targetCols, err := a.getTableColumns(r, form.TargetTable)
	if err != nil {
		errJSON(w, http.StatusBadRequest, "目标表不存在或无法读取结构: "+err.Error())
		return
	}

	// AI 校验列名
	validation, err := a.validateColumns(r, form, parsed, targetCols)
	if err != nil {
		// AI 调用失败时降级为基本列名匹配
		validation, err = basicColumnCheck(parsed.Columns, targetCols)
		if err != nil {
			errJSON(w, http.StatusBadRequest, "列名校验失败: "+err.Error())
			return
		}
	}
	if !validation.OK {
		errJSON(w, http.StatusBadRequest, validation.Error)
		return
	}

	// 根据操作类型执行
	switch form.Operation {
	case "insert":
		affected, execErr := a.executeInsert(r, form.TargetTable, parsed.Columns, parsed.Rows, validation.ColumnMap)
		if execErr != nil {
			errJSON(w, http.StatusInternalServerError, "数据导入失败: "+execErr.Error())
			return
		}
		JSON(w, http.StatusOK, map[string]any{
			"ok":            true,
			"rows_affected": affected,
			"validation":    validation,
		})

	case "update":
		affected, execErr := a.executeUpdate(r, form.TargetTable, parsed.Columns, parsed.Rows, validation.ColumnMap)
		if execErr != nil {
			errJSON(w, http.StatusInternalServerError, "数据更新失败: "+execErr.Error())
			return
		}
		JSON(w, http.StatusOK, map[string]any{
			"ok":            true,
			"rows_affected": affected,
			"validation":    validation,
		})

	default:
		errJSON(w, http.StatusBadRequest, "operation 必须为 insert 或 update")
	}
}

// parseUploadForm 解析 multipart 上传表单
func parseUploadForm(r *http.Request) (uploadForm, error) {
	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB
		return uploadForm{}, fmt.Errorf("文件过大或解析失败")
	}

	f, header, err := r.FormFile("file")
	if err != nil {
		return uploadForm{}, fmt.Errorf("缺少 file 字段")
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return uploadForm{}, fmt.Errorf("读取文件失败")
	}

	op := strings.TrimSpace(r.FormValue("operation"))
	if op == "" {
		op = "insert"
	}
	if op != "insert" && op != "update" && op != "create_table" {
		return uploadForm{}, fmt.Errorf("operation 必须为 insert/update/create_table")
	}

	return uploadForm{
		FileData:    data,
		FileName:    header.Filename,
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
		return parseResult{}, fmt.Errorf("CSV 格式错误: %w", err)
	}
	if len(records) < 1 {
		return parseResult{}, fmt.Errorf("CSV 文件为空")
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
		return parseResult{}, fmt.Errorf("XLSX 解析失败: %w", err)
	}
	defer f.Close()

	sheet := f.GetSheetName(0)
	rows, err := f.GetRows(sheet)
	if err != nil {
		return parseResult{}, fmt.Errorf("读取 sheet 失败: %w", err)
	}
	if len(rows) < 1 {
		return parseResult{}, fmt.Errorf("XLSX 文件为空")
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
		return parseResult{}, fmt.Errorf("SQL 文件为空")
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
		return parseResult{}, fmt.Errorf("SQL 文件中无有效语句")
	}
	return parseResult{SQLStmts: stmts, RowCount: len(stmts)}, nil
}

// getTableColumns 获取目标表的列信息
func (a *App) getTableColumns(r *http.Request, tableName string) (map[string]string, error) {
	if sqlrun.IsSystemTable(tableName) {
		return nil, fmt.Errorf("无权操作系统表 %s", tableName)
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
	if len(cols) == 0 {
		return nil, fmt.Errorf("表 %s 不存在或没有列", tableName)
	}
	return cols, nil
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
		return columnValidation{}, fmt.Errorf("AI 返回格式异常: %w", err)
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
				Error: fmt.Sprintf("列 %s 在目标表中不存在，可用的列：%v", uc, mapKeys(targetCols)),
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

// executeInsert 批量 INSERT 数据
func (a *App) executeInsert(r *http.Request, table string, cols []string, rows [][]string, colMap map[string]string) (int64, error) {
	if sqlrun.IsSystemTable(table) {
		return 0, fmt.Errorf("无权操作系统表 %s", table)
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
			return totalAffected, fmt.Errorf("第 %d 行插入失败: %w", totalAffected+1, err)
		}
		totalAffected += affected
	}
	return totalAffected, nil
}

// executeUpdate 批量 UPDATE 数据（第一列作为主键条件）
func (a *App) executeUpdate(r *http.Request, table string, cols []string, rows [][]string, colMap map[string]string) (int64, error) {
	if sqlrun.IsSystemTable(table) {
		return 0, fmt.Errorf("无权操作系统表 %s", table)
	}
	if err := sqlrun.IsAllowedForRole(sqlrun.SQLTypeUpdate, middleware.ClaimsFromContext(r.Context()).Role); err != nil {
		return 0, err
	}

	if len(cols) < 2 {
		return 0, fmt.Errorf("UPDATE 模式需要至少两列（第一列为主键条件）")
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
			return totalAffected, fmt.Errorf("第 %d 行更新失败: %w", totalAffected+1, err)
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
				fmt.Sprintf("第 %d 条语句：无权限执行 %s 操作", i+1, sqlType))
			return
		}
		if tbl, ok := sqlrun.CheckSystemTableInSQL(stmt); ok {
			errJSON(w, http.StatusForbidden,
				fmt.Sprintf("第 %d 条语句：无权操作系统表 %s", i+1, tbl))
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

// SuggestTable AI 建议建表
// POST /v1/data/suggest-table
// Body: {"description": "用户描述", "ai_hint": "可选提示"}
func (a *App) SuggestTable(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	if err := sqlrun.IsAllowedForRole(sqlrun.SQLTypeCreateTable, c.Role); err != nil {
		errJSON(w, http.StatusForbidden, "仅 operator 及以上角色可建表")
		return
	}

	var body struct {
		Description string `json:"description"`
		AIHint      string `json:"ai_hint"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(body.Description) == "" {
		errJSON(w, http.StatusBadRequest, "description 为必填项")
		return
	}

	if a.LlmClient == nil || a.LlmClient.APIKey == "" {
		errJSON(w, http.StatusServiceUnavailable, "AI 服务未配置 (HUB_LLM_API_KEY)")
		return
	}

	hint := ""
	if body.AIHint != "" {
		hint = "\n额外要求：" + body.AIHint
	}

	prompt := fmt.Sprintf(
		`你是一个 PostgreSQL 数据库设计专家。根据用户描述设计表结构。

用户描述：%s
%s

请返回 JSON（仅 JSON，无其他文字）：
{
  "table_name": "建议的表名（英文，snake_case）",
  "columns": [
    {"name": "列名", "type": "PostgreSQL类型", "nullable": true, "default": "默认值或空", "comment": "列说明"}
  ],
  "ddl": "完整的 CREATE TABLE DDL 语句",
  "explanation": "设计说明（中文）"
}`,
		body.Description, hint)

	ctx, cancel := contextWithTimeout(r, 20*time.Second)
	defer cancel()
	resp, err := a.LlmClient.ChatCompletion(ctx, a.Cfg.LLMModel, prompt)
	if err != nil {
		errJSON(w, http.StatusBadGateway, "AI 服务调用失败: "+err.Error())
		return
	}

	resp = extractJSON(resp)
	var suggestion map[string]any
	if err := json.Unmarshal([]byte(resp), &suggestion); err != nil {
		errJSON(w, http.StatusInternalServerError, "AI 返回格式异常: "+err.Error())
		return
	}

	JSON(w, http.StatusOK, suggestion)
}

// CreateTable 用户确认后执行建表
// POST /v1/data/create-table
// Body: {"ddl": "CREATE TABLE ..."}
func (a *App) CreateTable(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	if err := sqlrun.IsAllowedForRole(sqlrun.SQLTypeCreateTable, c.Role); err != nil {
		errJSON(w, http.StatusForbidden, "仅 operator 及以上角色可建表")
		return
	}

	var body struct {
		DDL string `json:"ddl"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid json")
		return
	}
	ddl := strings.TrimSpace(body.DDL)
	if ddl == "" {
		errJSON(w, http.StatusBadRequest, "ddl 为必填项")
		return
	}

	// 安全校验
	sqlType := sqlrun.ClassifySQL(ddl)
	if sqlType != sqlrun.SQLTypeCreateTable {
		errJSON(w, http.StatusBadRequest, "仅允许 CREATE TABLE 操作")
		return
	}
	if err := sqlrun.IsAllowedForRole(sqlType, c.Role); err != nil {
		errJSON(w, http.StatusForbidden, err.Error())
		return
	}
	if tbl, ok := sqlrun.CheckSystemTableInSQL(ddl); ok {
		errJSON(w, http.StatusForbidden, fmt.Sprintf("无权操作系统表 %s", tbl))
		return
	}

	affected, err := sqlrun.ExecuteWrite(r.Context(), a.DB, ddl, 30*time.Second)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, "建表失败: "+err.Error())
		return
	}

	JSON(w, http.StatusCreated, map[string]any{
		"ok":      true,
		"message": "表已创建，此操作不可回退",
		"ddl":     ddl,
		"rows":    affected,
	})
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
		errJSON(w, http.StatusInternalServerError, "查询表结构失败")
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
			errJSON(w, http.StatusInternalServerError, "读取表结构失败")
			return
		}
		if sqlrun.IsSystemTable(tName) {
			continue
		}
		t, ok := tableMap[tName]
		if !ok {
			tableOrder = append(tableOrder, tName)
			lastVacStr := ""
			if lastVac != nil {
				lastVacStr = *lastVac
			}
			t = &tblInfo{Name: tName, RowEst: rowEst}
			if lastVac != nil {
				t.LastVacuum = lastVac
			}
			_ = lastVacStr
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
		errJSON(w, http.StatusInternalServerError, "遍历表结构失败")
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

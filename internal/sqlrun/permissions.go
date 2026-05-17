package sqlrun

import (
	"fmt"
	"strings"
)

// 角色等级映射（与 middleware.RequireMinRole 保持一致）
var roleOrder = map[string]int{
	"viewer":   1,
	"operator": 2,
	"admin":    3,
}

// 系统表黑名单 — schema 发现和 SQL 执行双端引用
var systemTables = map[string]bool{
	"users":           true,
	"workspaces":      true,
	"sessions":        true,
	"messages":        true,
	"runs":            true,
	"audit_events":    true,
	"async_tasks":     true,
	"knowledge_docs":  true,
	"data_sources":    true,
	"agent_run_steps": true,
}

// 每种 SQL 类型所需的最低角色
var minRoleForSQLType = map[SQLType]string{
	SQLTypeSelect:      "viewer",
	SQLTypeInsert:      "operator",
	SQLTypeUpdate:      "operator",
	SQLTypeCreateTable: "operator",
	SQLTypeCreateDB:    "operator",
	SQLTypeDelete:      "admin",
	SQLTypeDrop:        "admin",
	SQLTypeAlter:       "admin",
	SQLTypeTruncate:    "admin",
	SQLTypeUnknown:     "admin", // 未知类型仅允许 admin
}

// IsSystemTable 检查表名是否在系统表黑名单中
func IsSystemTable(tableName string) bool {
	return systemTables[tableName]
}

// CheckSystemTableInSQL 检查 SQL 中是否引用了系统表，返回第一个匹配的表名
// 使用简单的关键词匹配，用于写操作路径拦截
func CheckSystemTableInSQL(sql string) (string, bool) {
	upper := strings.ToUpper(sql)
	for tbl := range systemTables {
		// 匹配 "tbl" 或 "tbl " 或 "tbl," 或 "tbl)" 等
		upperTbl := strings.ToUpper(tbl)
		idx := strings.Index(upper, upperTbl)
		if idx == -1 {
			continue
		}
		// 检查匹配位置前后的字符，避免部分匹配
		before := idx == 0 || !isIdentChar(upper[idx-1])
		after := idx+len(upperTbl) >= len(upper) || !isIdentChar(upper[idx+len(upperTbl)])
		if before && after {
			return tbl, true
		}
	}
	return "", false
}

// isIdentChar 判断字符是否为 SQL 标识符字符
func isIdentChar(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_'
}

// IsAllowedForRole 判断指定角色是否有权执行指定类型的 SQL
func IsAllowedForRole(sqlType SQLType, role string) error {
	minRole, ok := minRoleForSQLType[sqlType]
	if !ok {
		return fmt.Errorf("unsupported SQL type: %s", sqlType)
	}
	if roleOrder[role] < roleOrder[minRole] {
		return fmt.Errorf("角色 %s 无权执行 %s 操作（需要 %s 及以上）", role, sqlType, minRole)
	}
	return nil
}

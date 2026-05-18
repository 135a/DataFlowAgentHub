package sqlrun

import (
	"fmt"
	"strings"

	"github.com/dataflowagenthub/hub/internal/rbac"
)

// 系统表黑名单 — schema 发现和 SQL 执行双端引用
var systemTables = map[string]bool{
	"users":               true,
	"workspaces":          true,
	"sessions":            true,
	"messages":            true,
	"runs":                true,
	"audit_events":        true,
	"async_tasks":         true,
	"knowledge_docs":      true,
	"data_sources":        true,
	"agent_run_steps":     true,
	"datasets":            true,
	"dataset_permissions": true,
	"dataset_tables":      true,
	"table_fields":        true,
	"permission_requests": true,
}

// 每种 SQL 类型所需的最低角色
var minRoleForSQLType = map[SQLType]string{
	SQLTypeSelect:      "read_only_visitor",
	SQLTypeInsert:      "normal_user",
	SQLTypeUpdate:      "normal_user",
	SQLTypeCreateTable: "data_admin",
	SQLTypeCreateDB:    "super_admin",
	SQLTypeDelete:      "data_admin",
	SQLTypeDrop:        "super_admin",
	SQLTypeAlter:       "data_admin",
	SQLTypeTruncate:    "data_admin",
	SQLTypeUnknown:     "super_admin",
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
	if rbac.RoleOrder[role] < rbac.RoleOrder[minRole] {
		return fmt.Errorf("role %s is not allowed to perform %s (requires %s or above)", role, sqlType, minRole)
	}
	return nil
}

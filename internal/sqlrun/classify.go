package sqlrun

import "strings"

// SQLType 表示 SQL 操作类型
type SQLType string

const (
	SQLTypeSelect        SQLType = "SELECT"
	SQLTypeInsert        SQLType = "INSERT"
	SQLTypeUpdate        SQLType = "UPDATE"
	SQLTypeDelete        SQLType = "DELETE"
	SQLTypeCreateTable   SQLType = "CREATE_TABLE"
	SQLTypeCreateDB      SQLType = "CREATE_DATABASE"
	SQLTypeDrop          SQLType = "DROP"
	SQLTypeAlter         SQLType = "ALTER"
	SQLTypeTruncate      SQLType = "TRUNCATE"
	SQLTypeUnknown       SQLType = "UNKNOWN"
)

// ClassifySQL 基于关键词前缀识别 SQL 操作类型
func ClassifySQL(sql string) SQLType {
	u := strings.ToUpper(strings.TrimSpace(sql))
	// 去掉开头的括号或 WITH
	u = stripParens(u)

	switch {
	case hasPrefixWord(u, "SELECT"):
		return SQLTypeSelect
	case hasPrefixWord(u, "WITH"):
		return SQLTypeSelect // CTE 也是只读查询
	case hasPrefixWord(u, "INSERT"):
		return SQLTypeInsert
	case hasPrefixWord(u, "UPDATE"):
		return SQLTypeUpdate
	case hasPrefixWord(u, "DELETE"):
		return SQLTypeDelete
	case hasPrefixWord(u, "TRUNCATE"):
		return SQLTypeTruncate
	case hasPrefixWord(u, "CREATE TABLE"):
		return SQLTypeCreateTable
	case hasPrefixWord(u, "CREATE DATABASE"):
		return SQLTypeCreateDB
	case hasPrefixWord(u, "CREATE"):
		return SQLTypeCreateTable // 默认 CREATE 当建表处理
	case hasPrefixWord(u, "DROP"):
		return SQLTypeDrop
	case hasPrefixWord(u, "ALTER"):
		return SQLTypeAlter
	default:
		return SQLTypeUnknown
	}
}

// hasPrefixWord 检查 s 是否以 word 开头（word 后必须是空格或结尾）
func hasPrefixWord(s, word string) bool {
	if !strings.HasPrefix(s, word) {
		return false
	}
	rest := s[len(word):]
	return len(rest) == 0 || rest[0] == ' ' || rest[0] == '\t' || rest[0] == '\n'
}

// stripParens 去掉开头的括号（处理多层嵌套）
func stripParens(s string) string {
	for strings.HasPrefix(s, "(") {
		s = strings.TrimSpace(s[1:])
	}
	return s
}

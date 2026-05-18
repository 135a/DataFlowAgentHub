package sqlrun

import (
	"fmt"
	"strings"
)

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

// writeKeywords 定义所有写操作关键字（用于 IsReadOnlySQL 检测）
var writeKeywords = map[string]bool{
	"INSERT": true, "UPDATE": true, "DELETE": true, "DROP": true,
	"ALTER": true, "TRUNCATE": true, "CREATE": true, "REPLACE": true,
	"MERGE": true, "EXEC": true, "EXECUTE": true, "CALL": true,
	"GRANT": true, "REVOKE": true, "LOAD": true, "IMPORT": true, "COPY": true,
}

// IsReadOnlySQL 通过词法分析判断 SQL 是否只读。
// 使用手写简单词法分析器，正确跳过注释与字符串字面量，
// 防止 /**/INSERT/**/、字符串内嵌关键字等绕过手段。
func IsReadOnlySQL(sql string) error {
	words := tokenize(sql)
	for _, w := range words {
		upper := strings.ToUpper(w)
		if writeKeywords[upper] {
			return fmt.Errorf("only read-only SQL is allowed (found keyword: %s)", upper)
		}
	}
	return nil
}

// tokenize 对 SQL 进行简单词法分析，提取所有"单词"（标识符/关键字），
// 自动跳过注释（-- 和 /* */）和字符串字面量（'...'、'...' 内转义引号）的内容。
func tokenize(sql string) []string {
	var words []string
	i := 0
	for i < len(sql) {
		c := sql[i]

		// 跳过空白字符
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			i++
			continue
		}

		// 单行注释 --
		if c == '-' && i+1 < len(sql) && sql[i+1] == '-' {
			end := strings.Index(sql[i:], "\n")
			if end == -1 {
				break // 文件末尾，无换行
			}
			i += end + 1
			continue
		}

		// 多行注释 /* */
		if c == '/' && i+1 < len(sql) && sql[i+1] == '*' {
			end := strings.Index(sql[i+2:], "*/")
			if end == -1 {
				break // 未闭合注释，直接截断
			}
			i += end + 4
			continue
		}

		// 字符串字面量 '...'（处理内嵌转义单引号 ''）
		if c == '\'' {
			j := i + 1
			for j < len(sql) {
				if sql[j] == '\'' {
					if j+1 < len(sql) && sql[j+1] == '\'' {
						j += 2 // 转义单引号 ''
						continue
					}
					j++ // 结束引号
					break
				}
				j++
			}
			i = j
			continue
		}

		// 双引号标识符 "..."（跳过内容，防止 "INSERT" 被误判）
		if c == '"' {
			j := i + 1
			for j < len(sql) && sql[j] != '"' {
				if sql[j] == '\\' {
					j++ // 跳过转义
				}
				j++
			}
			if j < len(sql) {
				j++ // 跳过结束引号
			}
			i = j
			continue
		}

		// 反引号标识符 `...`
		if c == '`' {
			j := i + 1
			for j < len(sql) && sql[j] != '`' {
				j++
			}
			if j < len(sql) {
				j++
			}
			i = j
			continue
		}

		// 单词（标识符或关键字）：以字母或下划线开头
		if isAlpha(c) || c == '_' {
			j := i
			for j < len(sql) && (isAlnum(sql[j]) || sql[j] == '_') {
				j++
			}
			words = append(words, sql[i:j])
			i = j
			continue
		}

		// 数字：跳过
		if c >= '0' && c <= '9' {
			j := i
			for j < len(sql) && sql[j] >= '0' && sql[j] <= '9' {
				j++
			}
			i = j
			continue
		}

		// 其他字符（运算符、标点等）：跳过
		i++
	}
	return words
}

// isAlpha 判断字节是否为字母
func isAlpha(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// isAlnum 判断字节是否为字母或数字
func isAlnum(c byte) bool {
	return isAlpha(c) || (c >= '0' && c <= '9')
}

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

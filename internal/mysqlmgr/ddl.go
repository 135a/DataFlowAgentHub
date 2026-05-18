package mysqlmgr

import (
	"database/sql"
	"fmt"
	"strings"
)

// FieldDef 定义 MySQL 表中的字段结构。
type FieldDef struct {
	Name       string
	FieldType  string // MySQL 类型：VARCHAR, INT, DECIMAL 等
	FieldLen   int    // 字段长度（如 VARCHAR(100) 中的 100）
	IsNullable bool
}

// validTypes 是允许的 MySQL 字段类型白名单。
var validTypes = map[string]bool{
	"VARCHAR":  true,
	"INT":      true,
	"BIGINT":   true,
	"DECIMAL":  true,
	"DATE":     true,
	"DATETIME": true,
	"TEXT":     true,
	"BOOLEAN":  true,
	"FLOAT":    true,
	"DOUBLE":   true,
}

// IsValidFieldType 检查字段类型是否在白名单中。
func IsValidFieldType(t string) bool {
	return validTypes[strings.ToUpper(t)]
}

// ValidateFields 校验字段定义合法性。
func ValidateFields(fields []FieldDef) error {
	names := make(map[string]bool)
	for _, f := range fields {
		upper := strings.ToUpper(f.FieldType)
		if !validTypes[upper] {
			return fmt.Errorf("invalid field type %q for %q", f.FieldType, f.Name)
		}
		if (upper == "VARCHAR" || upper == "DECIMAL") && f.FieldLen <= 0 {
			return fmt.Errorf("field %q with type %s requires length > 0", f.Name, upper)
		}
		if names[f.Name] {
			return fmt.Errorf("duplicate field name %q", f.Name)
		}
		names[f.Name] = true
	}
	return nil
}

// buildCreateTableSQL 根据字段定义生成 CREATE TABLE SQL。
func buildCreateTableSQL(tableName string, fields []FieldDef) string {
	var cols []string
	for _, f := range fields {
		upper := strings.ToUpper(f.FieldType)
		var colDef string
		switch upper {
		case "VARCHAR":
			colDef = fmt.Sprintf("`%s` VARCHAR(%d)", f.Name, f.FieldLen)
		case "DECIMAL":
			colDef = fmt.Sprintf("`%s` DECIMAL(%d,2)", f.Name, f.FieldLen)
		case "INT", "BIGINT", "FLOAT", "DOUBLE":
			colDef = fmt.Sprintf("`%s` %s", f.Name, upper)
		case "BOOLEAN":
			colDef = fmt.Sprintf("`%s` TINYINT(1)", f.Name)
		default:
			colDef = fmt.Sprintf("`%s` %s", f.Name, upper)
		}
		if !f.IsNullable {
			colDef += " NOT NULL"
		}
		cols = append(cols, colDef)
	}
	return fmt.Sprintf("CREATE TABLE `%s` (\n  %s\n) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4",
		tableName, strings.Join(cols, ",\n  "))
}

// CreateDatabase 在 MySQL 中创建数据库。
func (m *Manager) CreateDatabase(dbName string) error {
	rootDB, err := sql.Open("mysql", m.RootDSN())
	if err != nil {
		return fmt.Errorf("mysqlmgr: open root: %w", err)
	}
	defer rootDB.Close()

	_, err = rootDB.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` DEFAULT CHARSET utf8mb4", dbName))
	if err != nil {
		return fmt.Errorf("mysqlmgr: create database %s: %w", dbName, err)
	}
	return nil
}

// DropDatabase 在 MySQL 中删除数据库。
func (m *Manager) DropDatabase(dbName string) error {
	rootDB, err := sql.Open("mysql", m.RootDSN())
	if err != nil {
		return fmt.Errorf("mysqlmgr: open root: %w", err)
	}
	defer rootDB.Close()

	_, err = rootDB.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", dbName))
	if err != nil {
		return fmt.Errorf("mysqlmgr: drop database %s: %w", dbName, err)
	}
	return nil
}

// CreateTable 在指定数据集的 MySQL 数据库中创建表。
// fields 中的字段顺序决定了 ordinal_position。
func (m *Manager) CreateTable(datasetID, mysqlDB, mysqlTableName string, fields []FieldDef) error {
	if err := ValidateFields(fields); err != nil {
		return fmt.Errorf("mysqlmgr: validate fields: %w", err)
	}

	pool, ok := m.GetPool(datasetID)
	if !ok {
		var err error
		pool, err = m.Connect(datasetID, mysqlDB)
		if err != nil {
			return err
		}
	}

	ddl := buildCreateTableSQL(mysqlTableName, fields)
	_, err := pool.Exec(ddl)
	if err != nil {
		return fmt.Errorf("mysqlmgr: create table %s: %w", mysqlTableName, err)
	}
	return nil
}

// DropTable 在指定数据集的 MySQL 数据库中删除表。
func (m *Manager) DropTable(datasetID, mysqlDB, mysqlTableName string) error {
	var pool *sql.DB
	var ok bool
	if pool, ok = m.GetPool(datasetID); !ok {
		var err error
		pool, err = m.Connect(datasetID, mysqlDB)
		if err != nil {
			return err
		}
	}

	_, err := pool.Exec(fmt.Sprintf("DROP TABLE IF EXISTS `%s`", mysqlTableName))
	if err != nil {
		return fmt.Errorf("mysqlmgr: drop table %s: %w", mysqlTableName, err)
	}
	return nil
}

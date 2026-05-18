## 1. Clean data.go

- [x] 1.1 移除 `uploadForm.TargetTable` 字段和 `uploadForm` 中 `create_table` 操作文档
- [x] 1.2 简化 `UploadData` handler：移除旧路径分支，仅保留 `dataset_id + table_id` 路径
- [x] 1.3 移除 `uploadToLegacy` 方法
- [x] 1.4 移除 `getTableColumns` 方法
- [x] 1.5 移除 `executeLegacyInsert` 方法
- [x] 1.6 移除 `executeLegacyUpdate` 方法
- [x] 1.7 移除 `executeSQLImport` 方法
- [x] 1.8 移除 `ListTables` 方法
- [x] 1.9 移除 `SuggestTable` 方法
- [x] 1.10 移除 `CreateTable` 方法
- [x] 1.11 清理 `parseUploadForm`：移除 `target_table` 解析和 `create_table` 操作校验

## 2. Clean routes

- [x] 2.1 从 `handlers.go` 中移除 `SuggestTable`、`CreateTable`、`ListTables` 路由注册

## 3. Clean tests

- [x] 3.1 从 `datasources_test.go` 中移除 `TestListTablesAsOperator` 测试

## 4. Verify

- [x] 4.1 运行 `go build ./...` 确认编译通过
- [x] 4.2 运行 `go test ./internal/handlers/...` 确认测试通过

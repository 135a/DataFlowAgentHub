## Why

删除旧版 PostgreSQL 上传数据逻辑。重构后的系统已使用基于数据集（MySQL）的 uploadToDataset 流程，旧版 uploadToLegacy（含 ListTables、SuggestTable、CreateTable）已成为死代码，增加维护负担和混淆。

## What Changes

移除以下旧版代码：

**BREAKING**:
- `internal/handlers/data.go`: 移除 `uploadToLegacy`、`executeLegacyInsert`、`executeLegacyUpdate`、`executeSQLImport`、`getTableColumns`、`ListTables`、`SuggestTable`、`CreateTable` 方法
- `internal/handlers/handlers.go`: 移除 `/data/suggest-table`、`/data/create-table`、`/schema/tables` 路由注册
- `internal/handlers/datasources_test.go`: 移除 `TestListTablesAsOperator` 测试
- `uploadForm` 结构体: 移除 `TargetTable` 字段和 `create_table` 操作
- `UploadData` handler: 仅保留数据集上传路径（dataset_id + table_id）

## Capabilities

### New Capabilities
<!-- No new capabilities — pure deletion of dead code -->

### Modified Capabilities
<!-- No spec-level requirement changes -->

## Impact

- `internal/handlers/data.go` — 删除约 250 行旧代码
- `internal/handlers/handlers.go` — 删除 3 条路由
- `internal/handlers/datasources_test.go` — 删除 1 个测试函数
- 前端 `DataManagementPanel.tsx` — 已有对应修改（上一阶段完成），无需额外变更

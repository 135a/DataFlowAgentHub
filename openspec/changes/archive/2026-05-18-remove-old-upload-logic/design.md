## Context

系统重构后，数据上传已统一走「数据集 + 数据表」的 MySQL 流程（`uploadToDataset`）。旧版 PostgreSQL 上传路径（`uploadToLegacy`）及相关辅助功能（`ListTables`、`SuggestTable`、`CreateTable`）已不再被调用，属于死代码。

当前状态：
- `UploadData` handler 同时支持新旧两条路径，根据 `dataset_id`/`table_id` 或 `target_table` 判断
- 旧路径依赖 `getTableColumns`（查询 PostgreSQL `information_schema`）、`executeLegacyInsert`、`executeLegacyUpdate`、`executeSQLImport`
- `ListTables` 查询 PostgreSQL `pg_stat_user_tables`，已被数据集表管理 API 替代
- `SuggestTable`/`CreateTable` 已返回 410 Gone，改为纯删除

## Goals / Non-Goals

**Goals:**
- 移除所有旧版 PostgreSQL 上传逻辑（约 250 行死代码）
- 移除不再使用的路由和测试
- 简化 `UploadData` handler，仅保留数据集路径
- `uploadForm` 结构体清理：移除 `TargetTable` 字段和 `create_table` 操作

**Non-Goals:**
- 不修改 `sqlrun.ExecuteWrite` 通用写入方法（仍在 `nl2sqlexec` 中被使用）
- 不修改数据集相关的新流程代码
- 不修改前端（前端的旧逻辑已在上一阶段移除）

## Decisions

1. **直接删除而非注释**：死代码不应保留注释掉的版本，Git 历史可回溯。
2. **一次性清理**：不采用分阶段弃用，因为旧路径已无调用者。
3. **保留 `parseCSV`/`parseXLSX` 等通用解析函数**：它们在新旧路径中共享，仅删除旧路径的调用代码。
4. **保留 `validateColumns`/`basicColumnCheck`/`extractJSON` 等辅助函数**：这些在新路径中仍被使用。
5. **保留 `importAsyncThreshold` 常量**：虽然当前未使用，但可能是未来异步导入扩展点。如确认无用可后续删除。

## Risks / Trade-offs

- **向后兼容风险**：如果外部系统仍调用 `/v1/schema/tables` 或旧上传端点，会收到 404。→ 旧前端已不再使用，影响极小。
- **测试覆盖**：`TestListTablesAsOperator` 被删除，但数据集表管理已有新的测试。→ 确认测试覆盖已迁移。

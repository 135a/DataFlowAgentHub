## Why

数据库迁移文件当前分散在两个目录中——`internal/migrate/`（001-004）和 `migrations/`（005-007）——且 `migrations/` 目录下的 005-007 与 `internal/migrate/` 中的 002-004 内容完全相同。这种重复不仅造成维护困惑，还增加了新增迁移时选错目录的风险。应统一到单一的 `internal/migrate/` 目录，由 Go 的 `embed.FS` 集中管理。

## What Changes

- 删除 `migrations/` 目录及其下的 005-007 三个重复 SQL 文件
- 将 `migrations/` 中不存在的 `001_init.sql` 保留在 `internal/migrate/`（该文件仅存在于 `internal/migrate/`）
- 更新项目文档中对迁移文件布局的描述

## Capabilities

### New Capabilities

- `migration-file-management`: 定义迁移文件的存放、命名和执行规则，确保所有迁移文件集中在 `internal/migrate/` 并由 Go 的 embed 机制管理

### Modified Capabilities

<!-- 不存在需要修改规范的需求变更，仅为文件清理操作 -->

## Impact

- 受影响文件：`migrations/005_async_tasks.sql`、`migrations/006_knowledge_docs.sql`、`migrations/007_agent_run_steps.sql`（删除）
- 受影响文档：`CLAUDE.md` 中引用 `migrations/` 目录的部分（更新描述）
- 不影响运行时代码，Go 迁移逻辑（`internal/migrate/migrate.go`）仅使用 `embed.FS` 读取自身目录的 `.sql` 文件

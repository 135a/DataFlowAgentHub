## Context

当前项目存在两个迁移文件目录：

- `internal/migrate/` — Go `embed.FS` 管理的迁移目录，包含 `001_init.sql` 到 `004_agent_run_steps.sql`，是运行时唯一生效的迁移来源
- `migrations/` — 独立的迁移目录，包含 `005_async_tasks.sql`、`006_knowledge_docs.sql`、`007_agent_run_steps.sql`，内容与 `internal/migrate/` 中的 002-004 完全相同，无任何代码引用

Go 迁移逻辑（`internal/migrate/migrate.go`）通过 `//go:embed *.sql` 仅读取自身包内的 `.sql` 文件，`migrations/` 目录不参与构建和运行时。CLAUDE.md 中的架构文档引用了 `migrations/` 目录，造成开发者误解。

## Goals / Non-Goals

**Goals:**
- 删除冗余的 `migrations/` 目录及其内容
- 更新 CLAUDE.md 中对迁移文件布局的描述
- 确保所有迁移文件集中在 `internal/migrate/` 单一目录

**Non-Goals:**
- 不修改迁移文件内容（不合并、不重构）
- 不改变 `internal/migrate/migrate.go` 的迁移逻辑
- 不引入新的迁移机制或工具
- 不修改 Docker Compose 或其他部署配置（它们不依赖 `migrations/` 目录）

## Decisions

| 决策 | 选择 | 理由 |
|------|------|------|
| 保留哪个目录 | `internal/migrate/` | Go `embed.FS` 要求文件与 Go 源码同包，这是运行时唯一生效路径 |
| 如何处理 005-007 | 直接删除 | 内容与 002-004 完全相同，删除不会丢失任何信息 |
| 文件是否需要重编号 | 否 | 001-004 的命名已正确反映执行顺序，无需变更 |

## Risks / Trade-offs

- **误引用风险**：若有外部脚本或 CI 直接引用 `migrations/` 路径 → 排查 `Makefile`、CI 配置、Dockerfile 中是否存在此类引用，确认无引用后删除
- **文档同步风险**：CLAUDE.md 更新可能遗漏其他文档引用 → 执行全仓库搜索 `migrations/` 关键词确保覆盖完整

## Migration Plan

1. 全仓库搜索对 `migrations/` 目录的引用（排除 `internal/migrate/`）
2. 更新 CLAUDE.md 中架构描述，将 `migrations/` 引用改为 `internal/migrate/`
3. 删除 `migrations/` 目录及其下三个文件
4. 验证 `go build` 和 `go test ./...` 通过

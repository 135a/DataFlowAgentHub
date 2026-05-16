# migration-file-management

## Purpose

定义项目中数据库迁移文件的存放、命名和执行规则，确保所有迁移文件集中在 `internal/migrate/` 单一目录并由 Go 的 `embed.FS` 机制管理。

## Requirements

### Requirement: 迁移文件统一存放

所有数据库迁移 SQL 文件 SHALL 存放在 `internal/migrate/` 目录内，并由 Go 的 `embed.FS` 机制在启动时按文件名排序执行。

#### Scenario: 不存在冗余迁移目录

- **WHEN** 仓库中包含数据库迁移文件
- **THEN** 所有迁移文件 SHALL 仅存在于 `internal/migrate/` 目录中，不存在 `migrations/` 或其他迁移目录

#### Scenario: 迁移文件命名有序

- **WHEN** 新增迁移文件
- **THEN** 文件名 SHALL 以数字序号前缀（如 `005_xxx.sql`）命名，确保按字典序排列即对应正确的执行顺序

#### Scenario: 迁移文件被 Go embed 加载

- **WHEN** Go 服务启动并执行 `migrate.Up()`
- **THEN** 系统 SHALL 仅读取 `internal/migrate/` 目录下的 `.sql` 文件并执行尚未记录的迁移

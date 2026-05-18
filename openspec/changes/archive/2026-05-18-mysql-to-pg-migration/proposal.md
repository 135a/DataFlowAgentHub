## Why

当前项目维护双数据库架构：PostgreSQL 存储平台元数据（用户、会话、workspace 等），MySQL 存储数据集业务数据。这导致 AI 需要生成两种 SQL 方言、代码维护两套 SQL 执行引擎（pgxpool + database/sql）、部署需管理两个数据库实例。MVP 阶段业务量级完全在单 MySQL 实例能力范围内，所有 PG 存储的元数据都可以用等价 MySQL 表替代。移除 PG 后架构大幅简化，部署和运维成本显著降低。

## What Changes

- **BREAKING**: 移除 `internal/migrate/` 中所有 PG 迁移脚本，平台元数据表迁移到 MySQL
- **BREAKING**: 移除 `internal/connector/` 包中的 PG 连接检测逻辑
- **BREAKING**: 移除 `internal/schema/` 包中的 PG `information_schema` 查询
- 合并 SQL 执行引擎：删除 `sqlrun.QueryRows()` / `ExecuteWrite()`（pgxpool 版本），统一使用 MySQL 版本
- 合并 NL2SQL 执行路径：删除 PG dialect 的 `Execute()`，统一使用 MySQL 方言
- 移除 `github.com/jackc/pgx/v5` 依赖和相关配置
- 移除 PG 环境变量（`HUB_DATABASE_URL` 等 PG 连接配置）
- 平台元数据（users、workspaces、sessions、datasets、messages 等）迁移到 MySQL
- 数据集/表 CRUD：从两阶段（MySQL DDL + PG 元数据）改为单事务 MySQL 操作
- 删除 `internal/otelsetup/` 中与 PG 相关的 tracing 逻辑（如果仅关联 PG）

## Capabilities

### New Capabilities
- `dataset-storage`: 纯 MySQL 架构下的数据集物理存储方案，包括 database 的创建/删除以及内部表的 DDL 操作

### Modified Capabilities
- 无（纯重构，无功能行为变更）

## Impact

- **后端**: 移除 `internal/connector/`、`internal/schema/` 中 PG 相关逻辑；修改约 10+ 个 handler 文件
- **数据库**: 所有 PG 元数据表迁移到 MySQL，删除 PG 迁移脚本
- **配置**: 移除 PG 环境变量，保留并统一 MySQL 配置
- **依赖**: 移除 `github.com/jackc/pgx/v5`，保留 `github.com/go-sql-driver/mysql`
- **前端**: 无需变更（API 响应格式不变）
- **Docker**: 移除 PG 容器，仅保留 MySQL 容器

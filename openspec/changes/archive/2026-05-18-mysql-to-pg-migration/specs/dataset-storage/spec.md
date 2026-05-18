# dataset-storage

纯 MySQL 架构下的数据集物理存储方案。

## ADDED Requirements

### Requirement: 数据集创建时自动创建 MySQL Database

系统 SHALL 在创建数据集时自动创建对应的 MySQL Database，而非 PostgreSQL Schema。

**命名规则**: `ds_<dataset_id>`（16 位小写 hex），例如数据集 ID 为 `abc123` 则 Database 名为 `ds_abc123`。

**SQL**: `CREATE DATABASE IF NOT EXISTS \`ds_<id>\``

#### Scenario: 正常创建数据集并生成 Database

- **WHEN** 用户创建新数据集
- **THEN** 系统在 MySQL 中执行 `CREATE DATABASE IF NOT EXISTS \`ds_<id>\``
- **AND** 数据集创建完成后元数据写入 `datasets` 表（存储在同一 MySQL 实例的 `hub_platform` Database 中）

#### Scenario: Database 已存在时重复创建

- **WHEN** 用户创建数据集且对应 Database `ds_<id>` 已存在
- **THEN** 系统不报错，正常返回创建成功（`IF NOT EXISTS` 保证幂等）

#### Scenario: 创建时包含表定义

- **WHEN** 用户创建数据集时附带一组表的列定义
- **THEN** 系统在创建的 Database 下执行 `CREATE TABLE` 语句
- **AND** 使用应用层补偿模式（先 DDL 后 DML，失败时执行反向 DDL）

---

### Requirement: 数据集删除时级联删除 MySQL Database

系统 SHALL 在删除数据集时级联删除对应的 MySQL Database 及其所有内部表。

**SQL**: `DROP DATABASE IF EXISTS \`ds_<id>\``

#### Scenario: 正常删除数据集

- **WHEN** 用户删除数据集
- **THEN** 系统执行 `DROP DATABASE IF EXISTS \`ds_<id>\``
- **AND** 删除 `datasets` 表中的元数据记录
- **AND** 删除 `dataset_tables` 表中的关联记录

#### Scenario: Database 不存在时删除

- **WHEN** 用户删除数据集但对应 Database 已被手动删除
- **THEN** 系统不报错，正常完成删除（`IF EXISTS` 保证幂等）

---

### Requirement: 数据集内建表

系统 SHALL 支持在数据集 Database 下创建、查询、修改和删除业务表。

**表命名规则**: `tbl_<16位hex>` — 不直接暴露原始表名。

**DDL 类型**: AI 直接生成 MySQL 方言，无需类型映射。

#### Scenario: 在数据集下创建表

- **WHEN** 用户在数据集下创建表，提供列定义（类型、名称等）
- **THEN** 系统使用 `CREATE TABLE \`ds_<id>\`.\`tbl_<hex>\` (...)` 创建表
- **AND** 写入 `dataset_tables` 元数据记录

#### Scenario: 查询数据集中的表列表

- **WHEN** 用户查询数据集的表列表
- **THEN** 系统查询 `information_schema.tables WHERE table_schema = 'ds_<id>'`
- **AND** 返回表名、列定义、行数估计等信息

#### Scenario: 删除数据集中的表

- **WHEN** 用户删除数据集中的某张表
- **THEN** 系统执行 `DROP TABLE IF EXISTS \`ds_<id>\`.\`tbl_<hex>\``
- **AND** 删除 `dataset_tables` 中的对应元数据

---

### Requirement: 独立 MySQL Database 连接池管理

系统 SHALL 继续使用独立 `*sql.DB` 连接池管理每个数据集的 Database 连接（因为 MySQL 的 `USE database` 是连接级状态）。

**现有方案保持不变**: `internal/mysqlmgr/` 包中的 map 管理和 lazy connect 逻辑继续使用。

**注意**: 不再需要切换 Database，每个 `*sql.DB` 连接池已绑定到对应 Dataset 的 Database。

#### Scenario: 查询数据集数据

- **WHEN** 用户在数据集上下文中查询数据
- **THEN** 系统使用该数据集对应的 `*sql.DB` 连接池执行查询
- **AND** 连接自动使用 `ds_<id>` Database

#### Scenario: 数据集连接池懒加载

- **WHEN** 首次访问某数据集
- **THEN** 系统按需创建对应的 `*sql.DB` 连接池
- **AND** 缓存连接池以便后续复用

---

### Requirement: 平台元数据表迁移到 MySQL

系统 SHALL 将所有原本存储在 PostgreSQL 中的平台元数据表迁移到 MySQL 同一实例中。

**迁移目标 Database**: `hub_platform`

**迁移的表**: `workspaces`、`users`、`data_sources`、`sessions`、`messages`、`runs`、`approval_tasks`、`audit_events`、`async_tasks`、`knowledge_docs`、`agent_run_steps`、`datasets`、`dataset_tables`

**DDL 转换规则**:

| PG 类型 | MySQL 类型 |
|---|---|
| `SERIAL` | `INT AUTO_INCREMENT` |
| `BIGSERIAL` | `BIGINT AUTO_INCREMENT` |
| `UUID` | `VARCHAR(36)` |
| `TIMESTAMPTZ` | `DATETIME` |
| `BOOLEAN` | `TINYINT(1)` |
| `JSONB` | `JSON` |
| `TEXT[]` | `JSON` 或 `TEXT` |
| `VARCHAR(n)` | `VARCHAR(n)` |

#### Scenario: 迁移脚本创建平台元数据表

- **WHEN** 系统首次启动
- **THEN** 执行 MySQL 迁移脚本，在 `hub_platform` Database 中创建所有元数据表
- **AND** 原有 PG 迁移脚本不再执行

#### Scenario: 元数据读写走 MySQL

- **WHEN** handler 读写用户、workspace、会话等元数据
- **THEN** 系统使用 MySQL `hub_platform` 连接读写元数据
- **AND** 不再连接 PostgreSQL

---

### Requirement: 统一 SQL 执行引擎为 MySQL 版本

系统 SHALL 删除 PostgreSQL 专属的 SQL 执行函数，统一使用 MySQL (`database/sql` + `go-sql-driver/mysql`) 版本。

**删除的函数**:
- `sqlrun.QueryRows()` — pgxpool 版本
- `sqlrun.ExecuteWrite()` — pgxpool 版本
- `handlers.Execute()` — PG dialect 版本

**重命名的函数**:
- `sqlrun.QueryRowsMySQL()` → `sqlrun.QueryRows()`
- `sqlrun.ExecuteWriteMySQL()` → `sqlrun.ExecuteWrite()`
- `handlers.ExecuteMySQL()` → `handlers.Execute()`

#### Scenario: 原 PG 路径的调用方走 MySQL 路径

- **WHEN** 任何调用方发起数据查询请求
- **THEN** 系统统一通过 `database/sql` + MySQL driver 执行查询
- **AND** 使用 MySQL 方言
- **AND** 原来的 pgxpool 路径函数不再存在

#### Scenario: 重命名后编译通过

- **WHEN** 所有引用点完成重命名后执行 `go build`
- **THEN** 编译通过，无未解析的 `QueryRowsMySQL` 或 `ExecuteMySQL` 引用

---

### Requirement: 移除 PG 环境变量和依赖

系统 SHALL 清理所有 PostgreSQL 相关的配置项和 Go 模块依赖。

**移除的 PG 配置**:
- 数据库连接 URL（`HUB_DATABASE_URL` 等）
- pgxpool 初始化逻辑
- PG 相关的 config 结构体字段

**移除的依赖**:
- `github.com/jackc/pgx/v5`
- `github.com/jackc/pgx/v5/pgxpool`
- 其他仅 PG 使用的间接依赖

**保留的依赖**:
- `github.com/go-sql-driver/mysql` — 核心驱动
- `github.com/jmoiron/sqlx`（如有）— 简化 MySQL 操作

#### Scenario: 启动时不再连接 PG

- **WHEN** 系统启动
- **THEN** 不再初始化 pgxpool 连接池
- **AND** 不再检查 PG 配置（即使缺失也不报错）
- **AND** 仅初始化 MySQL 连接

#### Scenario: go.mod 不含 pgx 驱动

- **WHEN** 执行 `go mod tidy` 后
- **THEN** `go.mod` 中不包含 `github.com/jackc/pgx/v5`
- **AND** `go.sum` 中相关条目被清理

---

### Requirement: 迁移脚本从 PG 改为 MySQL

系统 SHALL 用 MySQL 兼容的迁移脚本替换 PG 迁移脚本。

**现有 PG 迁移脚本**: `internal/migrate/001_init.sql` ~ `004_*.sql`
**新的 MySQL 迁移脚本**: `internal/migrate/001_init.sql` 原地替换（内容改为 MySQL DDL）

执行机制不变：通过 `embed.FS` 打包，启动时按字母序自动执行。

#### Scenario: 全部迁移脚本执行成功

- **WHEN** 系统首次启动，执行迁移
- **THEN** 所有 MySQL 迁移脚本顺序执行
- **AND** `hub_platform` Database 中成功创建全部元数据表

#### Scenario: 迁移后 Schema 验证

- **WHEN** 迁移完成后查询表结构
- **THEN** 所有表列定义与 MySQL DDL 一致
- **AND** 无 PG 特有类型残留

---

### Requirement: Docker Compose 移除 PG 容器

系统 SHALL 在 Docker Compose 配置中移除 PostgreSQL 容器，仅保留 MySQL 容器。

#### Scenario: Docker Compose 启动

- **WHEN** 执行 `docker compose up -d`
- **THEN** 仅启动 MySQL 容器（以及其他非 PG 容器如 Redis、Chroma、NATS 等）
- **AND** PostgreSQL 容器不再出现在 compose 文件中

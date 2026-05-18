## Context

当前平台使用双数据库架构：PostgreSQL 存储用户、会话、数据集元数据等平台数据；MySQL 存储每个数据集的业务数据（每个数据集 = 一个独立 MySQL Database）。代码中存在两套平行的 SQL 执行引擎：

- `sqlrun.QueryRows` / `ExecuteWrite` → pgxpool.Pool（PostgreSQL）
- `sqlrun.QueryRowsMySQL` / `ExecuteWriteMySQL` → *sql.DB（MySQL）

NL2SQL 同样需要两个路径 `Execute()` / `ExecuteMySQL()`，AI 需根据 dialect 参数生成不同方言 SQL。

MVP 阶段所有业务量级完全在单 MySQL 实例能力范围内，完全可以统一到 MySQL。

## Goals / Non-Goals

**Goals:**
- 移除 PostgreSQL 作为外部依赖，所有数据存储在 MySQL 中
- 统一 SQL 执行引擎为 database/sql + MySQL driver 一套
- 统一 NL2SQL 方言为 MySQL dialect
- 平台元数据表（users、workspaces、sessions 等）迁移到 MySQL
- 数据集/表 CRUD 从两阶段操作改为单事务 MySQL 操作
- 移除 `github.com/jackc/pgx/v5` 依赖和 PG 环境变量

**Non-Goals:**
- 不改变 API 响应格式（前端无需修改）
- 不改变权限/角色体系
- 不修改前端代码
- 不强制数据迁移（当前无存量业务数据，元数据可从 PG 导出再导入 MySQL）

## Decisions

### D1: 所有元数据表迁移到 MySQL

当前 PG 中的元数据表：`workspaces`、`users`、`data_sources`、`sessions`、`messages`、`runs`、`approval_tasks`、`audit_events`、`async_tasks`、`knowledge_docs`、`agent_run_steps`、`datasets`、`dataset_tables`

全部迁移到 MySQL 的同一实例中，创建独立数据库（如 `hub_platform`）存储。

**理由**: 消除双数据库架构，统一管理。MySQL 完全胜任 MVP 阶段的数据量。
**替代方案考虑**: 继续双数据库（复杂度高）、迁移到 MySQL 不同实例（部署复杂）— 均不如统一到同一 MySQL 实例。

### D2: 移除 pgxpool，统一使用 database/sql + MySQL

当前 `internal/sqlrun/` 包中存在两套实现：
- `QueryRows` / `ExecuteWrite` → pgxpool.Pool
- `QueryRowsMySQL` / `ExecuteWriteMySQL` → *sql.DB

迁移后：
- `QueryRowsMySQL` / `ExecuteWriteMySQL` 重命名为 `QueryRows` / `ExecuteWrite`（去掉 MySQL 后缀）
- 删除 pgxpool 版本
- `sqlrun.New()` 不再接收 pgxpool.Pool 参数

**理由**: 一套实现，消除重复代码和 dialect 判断逻辑。

### D3: MySQL Database 作为数据集隔离单位

保持现有方案不动：
- `CREATE DATABASE IF NOT EXISTS \`ds_xxx\``
- `DROP DATABASE IF EXISTS \`ds_xxx\``
- 每个 database 中的表名保持 `tbl_<hex>` 格式

每个数据集继续使用独立 MySQL Database 作为命名空间隔离。

### D4: 两阶段 → 单事务 MySQL 操作

当前创建数据集/表的流程是两阶段操作：
1. MySQL 执行 DDL（CREATE DATABASE/TABLE）
2. PG 写入元数据（INSERT INTO datasets/dataset_tables）

迁移后，元数据也存储在 MySQL 中：
1. BEGIN
2. CREATE DATABASE IF NOT EXISTS `ds_<id>` / CREATE TABLE
3. INSERT INTO hub_platform.datasets/dataset_tables
4. COMMIT

有任何步骤失败可 ROLLBACK，保证一致性。

**特别注意**: MySQL 的 DDL 语句（CREATE DATABASE、CREATE TABLE、DROP DATABASE、DROP TABLE）会隐式提交当前事务。这意味着 DDL + DML 混用的单事务方案在 MySQL 中行不通。需要在应用层处理：

**替代方案（应用层补偿）**:
1. 先执行 DDL（自动提交）
2. 再执行 DML（INSERT INTO 元数据）
3. 若 DML 失败，执行补偿 DDL（DROP DATABASE/TABLE）

或者使用 MySQL 8.0+ 的原子 DDL（InnoDB 支持原子 DDL，CREATE/DROP DATABASE/TABLE 是事务安全的）。

### D5: 类型映射（AI 生成 SQL 场景）

AI 只需生成 MySQL 方言，无需类型映射：
```
MySQL 方言
─────────────────
VARCHAR(n)
INT
BIGINT
DECIMAL(n,2)
FLOAT
DOUBLE
TINYINT(1) 作为 BOOLEAN
DATETIME
DATE
TEXT
```

**理由**: 不需要转换，AI 直接生成 MySQL 方言 SQL，减少一层心智负担和出错概率。

### D6: 元数据表 DDL 迁移方案

PG 迁移脚本（`internal/migrate/001_init.sql` ~ `004_*.sql`）中的 DDL 需要改写为 MySQL 语法：
- `SERIAL` → `INT AUTO_INCREMENT`
- `TEXT[]` / `VARCHAR[]` → 单值字段或 JSON
- `TIMESTAMPTZ` → `DATETIME`
- `UUID` → `VARCHAR(36)` 或 `BINARY(16)`
- `BOOLEAN` → `TINYINT(1)`
- `JSONB` → `JSON`

直接编写新的 MySQL 迁移脚本（`internal/migrate/mysql/001_init.sql` 等），按字母序自动执行。

## Risks / Trade-offs

| 风险 | 缓解措施 |
|------|---------|
| PG 特有类型（UUID、JSONB、数组类型）迁移到 MySQL 需要适配 | UUID → VARCHAR(36)，JSONB → JSON，数组 → JSON |
| MySQL DDL 隐式提交，无法 DDL+DML 同事务 | 采用应用层补偿模式：DDL 失败时回滚 DML，DML 失败时执行反向 DDL |
| 元数据迁移需要导出 PG 数据再导入 MySQL | 使用 `pg_dump → CSV → mysqlimport` 一次性迁移脚本 |
| pgx 的 `$N` 参数格式改为 MySQL 的 `?` | 全局替换参数占位符格式 |

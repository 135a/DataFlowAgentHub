## MODIFIED Requirements

### Requirement: Schema 自动发现

系统 SHALL 在 NL2SQL 请求前，自动从目标数据库的 `information_schema` 获取表与列的结构化元数据，并 MUST 将结果组装为结构化 JSON 注入 gRPC 请求的 `schema_json` 字段。

#### Scenario: 成功发现 demo 数据库 schema

- **WHEN** 会话未关联外部数据源，且 hub 自身 Postgres 的 `public` schema 包含 `demo_sales` 表
- **THEN** 系统 MUST 查询 `information_schema.columns` 获取列名与数据类型，返回包含 `{"tables": [{"name": "demo_sales", "columns": [...]}]}` 结构的 JSON

#### Scenario: 发现外部数据源 schema

- **WHEN** 会话关联了已注册的外部 Postgres 数据源
- **THEN** 系统 MUST 使用该数据源的凭据建立临时连接，查询其 `information_schema`，返回对应表结构

### Requirement: Schema 缓存

系统 SHALL 以 workspace 和数据源为粒度缓存 schema 结果，并 MUST 支持可配置的 TTL 过期策略。

#### Scenario: 缓存命中跳过数据库扫描

- **WHEN** 同一 workspace 的同一数据源在 TTL 内被再次请求
- **THEN** 系统 MUST 直接从 Redis 返回缓存的 schema，不重复执行 `information_schema` 查询

#### Scenario: 缓存未命中时回退到数据库查询

- **WHEN** Redis 中不存在对应 key 或缓存已过期
- **THEN** 系统 MUST 执行数据库 schema 查询并将结果写入 Redis

### Requirement: 表与列数量限制

系统 SHALL 对 schema 发现结果做数量上限控制，防止超大库导致上下文溢出。

#### Scenario: 超限截断

- **WHEN** 目标数据库的表数量超过 `HUB_SCHEMA_MAX_TABLES` 或某表的列数超过 `HUB_SCHEMA_MAX_COLUMNS_PER_TABLE`
- **THEN** 系统 MUST 按字母序截断并记录 warning 日志，不得静默丢弃

### Requirement: 发现失败必须阻断

系统 MUST NOT 在 schema 发现失败时降级为空 schema 继续执行 NL2SQL 请求。

#### Scenario: 数据源不可达时阻断

- **WHEN** 目标数据库连接超时或凭据错误
- **THEN** 系统 MUST 返回明确错误信息（含失败原因），将 run 状态置为 `failed`，并 MUST NOT 将空 schema 传入 NL2SQL worker

## ADDED Requirements

### Requirement: 多 Schema 发现

系统 SHALL 发现目标数据库中所有非系统 schema 的表结构，MUST NOT 仅局限于 `public` schema。

#### Scenario: 发现非 public schema 中的表

- **WHEN** 目标数据库存在 `sales` 和 `inventory` 两个非系统 schema，各自包含表
- **THEN** 系统 MUST 返回两个 schema 下所有表的列信息，在 `table_name` 中以前缀区分（如 `sales.orders`、`inventory.products`）

### Requirement: 数据库迁移自动执行

系统 SHALL 在启动时自动执行所有内嵌的数据库迁移文件，MUST 通过 `schema_migrations` 表追踪已应用的迁移版本，MUST NOT 重复执行已应用的迁移。

#### Scenario: 首次启动执行全部迁移

- **WHEN** 数据库为空（无 `schema_migrations` 表）
- **THEN** 系统 MUST 按文件名排序执行所有内嵌迁移，并将每个版本号写入 `schema_migrations` 表

#### Scenario: 后续启动仅执行新迁移

- **WHEN** `schema_migrations` 表中已记录 001、002、003 号迁移
- **THEN** 系统 MUST 跳过 001-003，仅执行 004 及更高版本的迁移

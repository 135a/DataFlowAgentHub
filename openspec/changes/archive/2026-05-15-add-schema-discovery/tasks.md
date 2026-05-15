## 1. Go 配置与基础设施

- [x] 1.1 在 `internal/config/config.go` 的 `Config` 结构体中新增 `SchemaCacheTTL`、`SchemaMaxTables`、`SchemaMaxColumnsPerTable` 字段
- [x] 1.2 在 `config.Load()` 中从环境变量 `HUB_SCHEMA_CACHE_TTL`、`HUB_SCHEMA_MAX_TABLES`、`HUB_SCHEMA_MAX_COLUMNS_PER_TABLE` 读取并设置默认值（300s / 50 / 100）

## 2. Schema 发现核心模块（internal/schema/）

- [x] 2.1 新建 `internal/schema/discovery.go`：实现 `DiscoverSchema` 函数，接收 `*pgxpool.Pool` 或临时连接，查询 `information_schema.columns`，按表名聚合为 `[]TableSchema` 结构，返回序列化 JSON
- [x] 2.2 实现 `SchemaResult` 结构体（`Tables []TableSchema`、`TableSchema` 含 `Name` + `Columns`、`ColumnSchema` 含 `Name`/`Type`/`Nullable`）
- [x] 2.3 实现表/列数量上限裁剪逻辑：按 `SchemaMaxTables` 和 `SchemaMaxColumnsPerTable` 截断，截断时输出 `zap.Warn` 日志
- [x] 2.4 实现从 `data_sources` 记录创建临时 pgx 连接的工具函数（5 秒连接超时，用完即关）

## 3. Schema 缓存层

- [x] 3.1 新建 `internal/schema/cache.go`：实现 `CachedSchema` 函数，逻辑为：查 Redis → 命中返回 → 未命中则调 `DiscoverSchema` → 写入 Redis → 返回
- [x] 3.2 Redis key 格式：`schema:{workspace_id}:{data_source_id_or_"hub"}`，TTL 使用 `SchemaCacheTTL`

## 4. 会话与数据源关联

- [x] 4.1 修改 `internal/handlers/handlers.go` 的 `CreateSession`：支持可选 `data_source_id` 请求体字段，创建会话时写入关联
- [x] 4.2 修改 `PostMessage`：在调用 `a.Nl2sql.GenerateSQL` 之前，根据会话的 `data_source_id` 决定 schema 来源（hub 自身 DB 或外部数据源），调用 `CachedSchema` 获取 schema JSON

## 5. 接入 NL2SQL 调用链路

- [x] 5.1 修改 `PostMessage` 中 `GenerateSQL` 调用：将 `"{}"` 替换为 schema 发现返回的真实 JSON；`dialect` 参数从数据源的 `kind` 字段读取（而非硬编码 `"postgres"`）
- [x] 5.2 处理 schema 发现失败路径：捕获错误 → `finishRunFailed` → 返回明确错误消息给前端

## 6. Python Worker 适配

- [x] 6.1 修改 `services/ai/hub_ai/__main__.py` 的 `_openai_sql` 函数中的 prompt 构造：将 `Schema context (JSON): {schema}` 改为更自然的自然语言描述格式（如 `Tables:\n- {name}: {col1} ({type}), col2 ({type})...`）
- [x] 6.2 实现 schema 文本长度检查：超过约 6000 字符时按表粒度截断，末尾追加 `"(已截断，超出部分未显示)"` 提示

## 7. 验证与文档

- [ ] 7.1 本地 `docker compose up` 启动全栈，验证 `demo_sales` 表的 schema 能正确发现并注入 prompt（检查日志或构造查询验证 SQL 质量）
- [ ] 7.2 用不少于 2 张表的业务场景做冒烟测试（如手动在 hub 数据库额外建一张 `products` 表，确认两张表都出现在 schema 中）
- [x] 7.3 更新 `docs/SMOKE_CHECKLIST.md`：增加 schema 发现相关的验证步骤

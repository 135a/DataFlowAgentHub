## Why

当前 NL2SQL 链路中，Go 控制面调用 Python worker 时传入的 `schema_json` 是硬编码的空 `"{}"`——LLM 完全看不到目标数据库的表结构。系统虽然具备数据源注册能力（`data_sources` 表 + CRUD API），但从未执行 schema 自动发现。结果是 LLM 只能根据用户自然语言中提到的表名"盲猜"SQL，在真实多表场景中完全不可用。这个缺口阻断了 NL2SQL 从 demo 走向可用产品的路径，必须优先解决。

## What Changes

- **NEW** Go 侧新增 Schema 发现模块：根据会话关联的数据源连接，自动查询 `information_schema` 获取表/列/类型信息，组装为结构化 JSON 传递给 Python NL2SQL worker。
- **NEW** Schema 缓存机制：以 `workspace_id + data_source_id` 为 key 缓存 schema 结果到 Redis，TTL 可配置，避免每次请求重复扫描数据库。
- **NEW** Schema 选择策略：将 schema JSON 按表聚合后注入 NL2SQL 请求的 `schema_json` 字段，替换当前硬编码的 `"{}"`。
- **NEW** 会话可关联数据源：在 `PostMessage` 流程中，根据会话的 `data_source_id` 选择对应的数据源连接来发现 schema。
- **MODIFIED** `nl2sql-engine` capability：Python NL2SQL worker 的 prompt 构造须能处理增强后的 schema JSON（含多表信息），并在 schema 上下文过长时做智能截断。
- **MODIFIED** `data-connectivity` capability：补充基于 `information_schema` 的受限元数据发现接口（当前 spec 已声明但未在 Go 侧实现，仅 Python worker 端有 mock）。

## Capabilities

### New Capabilities

- `schema-discovery`：Schema 自动发现与缓存模块。负责连接已注册数据源、执行 `information_schema` 查询、将结果组装为 NL2SQL 可用的结构化 schema JSON，并提供带 TTL 的 Redis 缓存。

### Modified Capabilities

- `nl2sql-engine`：NL2SQL worker 的输入契约增强——`schema_json` 从空值变为包含真实多表列信息的结构化 JSON；worker 须支持 schema 上下文智能截断（当列/表数量超过 token 预算时）。
- `data-connectivity`：在已有「连接器元数据发现」需求基础上，补充 Go 侧的实际实现与具体行为约定（缓存策略、失败处理、表/列数量上限）。

## Impact

- **Go 代码**：新增 `internal/schema/discovery.go`（schema 发现核心逻辑）+ `internal/schema/cache.go`（Redis 缓存层）；修改 `internal/handlers/handlers.go` 的 `PostMessage` 链路，在调用 NL2SQL 前注入 schema 发现步骤。
- **Python 代码**：修改 `services/ai/hub_ai/__main__.py` 的 `_openai_sql` prompt 构造，支持多表 schema 输入并增加 token 预算感知的截断逻辑。
- **数据库**：无需 Schema 变更；复用已有 `data_sources` 表和连接信息。
- **配置**：新增 `HUB_SCHEMA_CACHE_TTL`（默认 5 分钟）、`HUB_SCHEMA_MAX_TABLES`（默认 50）、`HUB_SCHEMA_MAX_COLUMNS_PER_TABLE`（默认 100）环境变量。
- **运维**：无新依赖，仅使用已有 Postgres 连接与 Redis。

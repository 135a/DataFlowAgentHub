## Context

当前 `PostMessage` 调用 NL2SQL worker 时 `schema_json` 硬编码为 `"{}"`，Python worker 在 prompt 里告诉 LLM 的 schema context 为空。平台已有 `data_sources` 表存储数据库连接信息，也有 `sessions.data_source_id` 外键关联数据源，但 schema 发现链路从未被实现。本设计在已有架构上增加 schema 自动发现模块，不改动数据库 schema，不新增外部依赖。

## Goals / Non-Goals

**Goals:**

- 在 Go 侧实现 schema 自动发现：连接受会话关联的数据源，查询 `information_schema`，获取表/列/类型信息。
- 将发现结果结构化，替换当前 `"{}"` 传入 Python NL2SQL worker。
- Redis 缓存 schema 结果，避免每次请求重新扫描数据库。
- token 预算保护：单次传给 LLM 的 schema 不超过合理上限，超出时做截断。
- 容错处理：schema 发现失败时返回明确错误，不降级为"盲猜"模式。

**Non-Goals:**

- 跨数据库类型的 schema 发现（本变更仅实现 Postgres 的 `information_schema`；MySQL/ClickHouse 连接器本身就不完整）。
- 语义增强层（业务术语映射、指标字典、列描述标注）——留给后续变更。
- 智能表选择（根据用户问题语义筛选相关表）——MVP 阶段传全部表结构，靠 token 上限截断。
- Schema 变更检测与缓存失效——依赖 TTL 过期重新拉取。

## Decisions

### D1：Schema 发现放在 Go 侧

- **选择**：Schema 发现逻辑在 Go 控制面实现，不放到 Python worker。
- **理由**：
  - 数据源连接凭据存储在 Go 侧（`data_sources` 表），Python worker 没有数据库访问能力。
  - 让 Python worker 直接连数据源会破坏安全边界（凭据泄露风险）。
  - Go 侧已有 `connector` 包和 pgx 连接池能力，复用成本低。
- **替代方案**：Python worker 通过单独 gRPC 调用反向请求 Go 获取 schema——多一次 RPC 往返，无必要。

### D2：复用 data_sources 连接，不使用 hub 元数据库

- **选择**：对于关联了外部数据源的会话，schema 发现连接**目标数据源**执行；对于未关联数据源的会话（MVP demo 场景），直接查询 hub 自身的 Postgres 元数据库的 `public` schema。
- **理由**：demo 场景下用户直接问 hub 数据库里的 `demo_sales` 表，不需要额外注册数据源；外部数据源场景则需要通过 `information_schema` 提供真实表结构。
- **流程**：
  ```
  session.data_source_id
    ├─ NULL → 用 hub 自身 DB Pool 查询 information_schema (public schema)
    └─ 有值 → 从 data_sources 读取凭据 → 建临时连接 → 查询 information_schema
  ```

### D3：Schema JSON 格式

- **选择**：采用简洁的扁平结构，仅包含 NL2SQL 必需字段：

```json
{
  "tables": [
    {
      "name": "demo_sales",
      "columns": [
        {"name": "id", "type": "integer", "nullable": false},
        {"name": "region", "type": "text", "nullable": false}
      ]
    }
  ]
}
```

- **理由**：与 proposal 中的示例格式一致；Python prompt 直接拼接即可，无需解析；避免复杂的嵌套（如外键关系）增加 token 消耗。
- **替代方案**：包含主键/外键/索引信息——MCP 阶段不必要，后续可按需扩展。

### D4：缓存策略

- **选择**：Redis 缓存，key = `schema:{workspace_id}:{data_source_id_or_"hub"}`，value = 序列化后的 schema JSON，TTL = `HUB_SCHEMA_CACHE_TTL`（默认 300 秒）。
- **理由**：同一 workspace 多次查询通常命中同一批表，缓存能显著降低 information_schema 重复查询；TTL 短保底使得表结构变更后 5 分钟内生效。
- **替代方案**：应用内存缓存（不选——多实例不共享，且 Docker 重启即丢）；不缓存（不选——每次请求都扫 information_schema 太浪费）。

### D5：Token 预算保护

- **选择**：Go 侧不做截断，仅做数量限制（`HUB_SCHEMA_MAX_TABLES` 默认 50 张表、`HUB_SCHEMA_MAX_COLUMNS_PER_TABLE` 默认 100 列）；Python 侧在构造 prompt 时如果 schema 文本超过约 6000 字符，按表粒度做尾部截断并附加提示。
- **理由**：Go 侧不了解 LLM tokenizer 细节；Python 侧更接近 prompt 构造，能精确控制最终注入的上下文长度。
- **替代方案**：Go 侧用 tiktoken 库做精确 token 计数——目前 Go 生态的 tiktoken 实现不够成熟，引入成本高。

### D6：失败处理

- **选择**：Schema 发现失败时，**不降级为 `"{}"`**，直接返回错误到前端（`"schema discovery failed: <reason>"`），run 状态为 `failed`。
- **理由**：用空 schema 让 LLM 盲猜 SQL 是静默的数据风险——LLM 可能幻觉出不存在的表名并生成语法正确但指向错误表的 SQL。必须在发现阶段就阻断。
- **替代方案**：降级为空 schema + 警告——不选，数据安全优先。

## Risks / Trade-offs

- **[风险] 大库 schema 扫描性能** → **[缓解]** `information_schema.columns` 查询本身很快（毫秒级），且加了 Redis 缓存；极端场景（数千张表）由 `HUB_SCHEMA_MAX_TABLES` 兜底。
- **[风险] 外部数据源连接池泄漏** → **[缓解]** 临时连接在执行完 schema 查询后立即关闭（不保持长连接池）；超时 5 秒。
- **[风险] 缓存与真实 schema 不一致（表刚被 DBA 改了）** → **[缓解]** TTL 短（5 分钟）；后续可考虑增加手动刷新 API。
- **[权衡] Python 截断可能导致 LLM 看不到某些表** → 可接受——如果用户问题涉及的表被截断了，LLM 返回的错误提示能引导用户缩小范围，比传全量 schema 让 LLM 混淆更好。

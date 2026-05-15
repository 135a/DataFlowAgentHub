## MODIFIED Requirements

### Requirement: 连接器元数据发现

系统 SHALL 支持对目标库进行受限元数据发现（catalog/schema/table/column 基础集合），供 NL2SQL 上下文构建使用。Go 控制面 MUST 在调用 NL2SQL worker 之前执行 schema 发现；发现结果 MUST 以结构化 JSON 格式注入 gRPC 请求。MUST 支持 Redis 缓存以降低重复扫描开销，缓存 TTL 可配置（默认 300 秒）。

#### Scenario: 元数据发现失败可恢复

- **WHEN** 目标库临时不可达或权限不足
- **THEN** 系统 MUST 返回明确错误并阻止进入执行阶段，且 MUST NOT 缓存错误状态为成功元数据

#### Scenario: 通过 information_schema 获取完整列信息

- **WHEN** Go 控制面连接至已注册的 Postgres 数据源
- **THEN** 系统 MUST 查询 `information_schema.columns WHERE table_schema = 'public'` 并按表名聚合列信息，返回包含表名、列名、数据类型的结构化结果

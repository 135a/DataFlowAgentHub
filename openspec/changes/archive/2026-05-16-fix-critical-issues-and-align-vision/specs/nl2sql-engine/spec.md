## MODIFIED Requirements

### Requirement: NL2SQL 生成与自检

Python NL2SQL worker SHALL 接收结构化上下文（含受限 schema 摘要、方言、业务规则片段），并 MUST 输出可供策略校验的 SQL（含置信度或自检说明字段）。当 `schema_json` 包含多表/多列信息时，worker MUST 能够正确解析并将其纳入 prompt 上下文；当 schema 文本超过约 6000 字符时，worker SHOULD 执行智能截断（按表粒度尾部截断并附加 `"(schema truncated, N tables omitted)"` 提示）。

#### Scenario: 生成可解析的 SQL 工件

- **WHEN** 控制面发送带 schema 上下文与用户问题的 GenerateSQL 请求
- **THEN** worker MUST 返回 SQL 字符串与结构化自检结果（例如语法/方言提示、风险提示），且 MUST 在输入不充足时返回可行动的错误而非空响应

#### Scenario: 多表 schema 输入下正确构造 prompt

- **WHEN** `schema_json` 包含 3 张表共计 20 列的完整描述
- **THEN** worker MUST 将所有表名列名信息嵌入 prompt，SQL 生成结果 MUST 能够正确引用 schema 中存在的表名和列名

#### Scenario: Schema 过长时智能截断

- **WHEN** `schema_json` 展开后超过约 6000 字符
- **THEN** worker SHOULD 按表粒度截断尾部超出的表，并在 prompt 中附加截断提示；MUST NOT 产生格式损坏的 JSON 注入

## ADDED Requirements

### Requirement: Multi-Agent 编排真实化

LangGraph 编排器中的 `nl2sql_node` SHALL 通过 HTTP 回调 Go API 执行真实的 NL2SQL 流程（而非返回 Mock 数据），MUST 遵循 Go 端的安全边界（只读 SQL 执行）。

#### Scenario: nl2sql_node 调用真实 NL2SQL

- **WHEN** LangGraph 工作流执行到 `nl2sql_node`
- **THEN** 节点 MUST 向 Go API 的 `/internal/nl2sql` 端点发送 HTTP 请求（含 user_message、schema_json、trace_id），并 MUST 将返回的查询结果存入 AgentState

#### Scenario: nl2sql_node 调用失败时传播错误

- **WHEN** `/internal/nl2sql` 返回非 200 状态码或连接超时
- **THEN** 节点 MUST 将错误信息存入 AgentState，并 MUST NOT 以假数据继续后续节点

### Requirement: 显式工作流选择

用户 SHALL 通过请求中的 `workflow` 参数显式选择 NL2SQL 执行路径，系统 MUST 支持 `simple`、`agent_pipeline`、`auto` 三种模式。

#### Scenario: workflow=simple 强制同步路径

- **WHEN** 请求中 `workflow` 参数为 `simple`
- **THEN** 系统 MUST 走同步 NL2SQL 路径，不触发 Agent 编排

#### Scenario: workflow=agent_pipeline 强制异步编排

- **WHEN** 请求中 `workflow` 参数为 `agent_pipeline`
- **THEN** 系统 MUST 走 Multi-Agent 编排路径（NATS 发布 + LangGraph），不论消息内容是否包含关键词

#### Scenario: workflow=auto 关键词检测

- **WHEN** 请求中 `workflow` 参数为 `auto` 或未设置
- **THEN** 系统 SHOULD 基于消息内容的关键词（中文"分析"/"报告"、英文"analyze"/"report"）判断是否触发 Multi-Agent 路径，保持向后兼容

### Requirement: RunAgentPipeline gRPC 客户端完整

Go API 的 gRPC 客户端 `internal/worker/nl2sql.go` SHALL 包含所有 proto 定义的 RPC 方法的客户端封装，包括 `RunAgentPipeline`。

#### Scenario: Go 端调用 RunAgentPipeline

- **WHEN** Go API 通过 gRPC 客户端调用 Python Worker 的 `RunAgentPipeline` RPC
- **THEN** 调用 MUST 成功建立连接并接收响应，MUST 支持 deadline/timeout 传播

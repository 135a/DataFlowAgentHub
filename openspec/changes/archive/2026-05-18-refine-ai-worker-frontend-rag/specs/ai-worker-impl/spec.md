## ADDED Requirements

### Requirement: AI Worker gRPC GenerateSQL 服务端实现

AI Worker SHALL 实现 `GenerateSQL` gRPC 方法，接收用户自然语言和数据库 schema，调用 LLM 生成 SQL 并返回。

#### Scenario: 成功生成 SQL
- **WHEN** `GenerateSQL` 收到有效的 `GenerateSQLRequest`（含 user_message 和 schema_json）
- **THEN** 调用 LLM 生成 SQL，返回 `GenerateSQLResponse` 其中 `ok=true`、`sql` 为生成的 SQL 语句

#### Scenario: LLM 调用失败返回错误
- **WHEN** LLM 调用超时或返回错误
- **THEN** 返回 `GenerateSQLResponse` 其中 `ok=false`、`error_message` 描述错误详情

#### Scenario: 输入参数为空
- **WHEN** `user_message` 或 `schema_json` 为空
- **THEN** 返回 `GenerateSQLResponse` 其中 `ok=false`、`error_message` 为 "user_message and schema_json are required"

### Requirement: AI Worker gRPC RunAgentPipeline 服务端实现

AI Worker SHALL 实现 `RunAgentPipeline` gRPC 方法，接收 agent 运行请求，启动 LangGraph 图并返回运行结果。

#### Scenario: 成功运行 Agent 流水线
- **WHEN** `RunAgentPipeline` 收到有效的 `RunAgentPipelineRequest`（含 user_message 和 schema_json）
- **THEN** 启动 LangGraph StateGraph 执行 NL2SQL → Analysis → Chart → Report 流水线，返回结果包含各步骤输出

#### Scenario: Agent 步骤失败返回错误信息
- **WHEN** 流水线中某一 agent 步骤（如 analysis_node）执行失败
- **THEN** 返回结果中包含 `error` 字段描述失败步骤和错误原因，已完成步骤的结果仍然返回

### Requirement: AI Worker gRPC Health 服务端实现

AI Worker SHALL 实现 `Health` gRPC 方法，返回服务健康状态。

#### Scenario: 服务正常
- **WHEN** AI Worker 正常运行且依赖（LLM、ChromaDB）可用
- **THEN** 返回 `HealthResponse` 其中 `status=SERVING`

#### Scenario: 服务依赖不可用
- **WHEN** ChromaDB 或 LLM 连接失败
- **THEN** 返回 `HealthResponse` 其中 `status=NOT_SERVING`、`message` 描述不可用依赖

### Requirement: 移除内部 HTTP 回调回路

Go API SHALL 直接通过 gRPC 调用 Python AI Worker，不再通过内部 HTTP 回调自身。

#### Scenario: API 发送消息时直接调用 Worker gRPC
- **WHEN** 用户发送消息且触发 agent pipeline
- **THEN** Go API 通过 gRPC 调用 Worker 的 `RunAgentPipeline` 方法，不再使用 HTTP 回调

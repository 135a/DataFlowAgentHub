## ADDED Requirements

### Requirement: OpenTelemetry 链路导出

系统 SHALL 支持通过 OTLP 协议导出分布式链路追踪数据，MUST 在 `HUB_OTEL_EXPORTER_ENDPOINT` 环境变量被设置时启用导出。

#### Scenario: 配置导出端点后链路数据被导出

- **WHEN** `HUB_OTEL_EXPORTER_ENDPOINT` 被设置为有效的 OTLP Collector 地址
- **THEN** Go API 服务的每个 HTTP 请求 MUST 生成包含 TraceID 和 SpanID 的 trace 数据，并发送至 Collector

#### Scenario: Collector 不可达时不阻塞 API 启动

- **WHEN** `HUB_OTEL_EXPORTER_ENDPOINT` 被设置为不可达的地址
- **THEN** API 服务 MUST 正常启动，trace 导出在后台重连，HTTP 请求正常处理

### Requirement: 全链路 TraceID 传播

Go API 生成的 TraceID SHALL 通过 gRPC metadata 和 HTTP 头传播到 Python AI Worker，MUST 在 Python 端日志中可追溯。

#### Scenario: gRPC 调用携带 TraceID

- **WHEN** Go API 调用 Python Worker 的 `GenerateSQL` gRPC
- **THEN** gRPC metadata 中 MUST 包含 W3C 格式的 `traceparent` 头

#### Scenario: Python 日志包含 TraceID

- **WHEN** Python Worker 处理来自 Go 的带 TraceID 的请求
- **THEN** Python 日志输出 MUST 包含对应的 `trace_id` 字段以便于日志关联

### Requirement: Prometheus 指标完善

系统 SHALL 暴露除 HTTP 请求指标外的业务指标，包括：NL2SQL 调用耗时分布、Agent Pipeline 任务状态计数、数据库连接池状态。

#### Scenario: NL2SQL 耗时直方图

- **WHEN** Go API 通过 gRPC 调用 Python Worker 的 `GenerateSQL`
- **THEN** 调用耗时 MUST 被记录到 `hub_nl2sql_duration_seconds` 直方图指标中

#### Scenario: Agent Pipeline 任务计数

- **WHEN** 异步任务状态发生变化（pending/running/completed/failed）
- **THEN** `hub_agent_pipeline_tasks_total` 计数器 MUST 按状态标签累加

## ADDED Requirements

### Requirement: 请求关联 id

系统 SHALL 为每个进入控制面的请求生成或传播 `trace_id`（或 W3C traceparent 等价物），并 MUST 在 Go 日志字段与对 Python gRPC 的 metadata 中传递同一关联 id。

#### Scenario: 端到端关联一次问答

- **WHEN** 客户端发起会话消息并在随后订阅 SSE
- **THEN** 与该问答相关的控制面日志与 worker 日志 MUST 可通过同一 trace id 关联检索（面试版允许简化实现为单一 header）

### Requirement: 结构化日志基线

控制面与 worker SHALL 输出 JSON 或键值结构化日志，且 MUST 包含级别、时间戳、服务名、trace id、关键业务 id（session_id/run_id）。

#### Scenario: 错误路径可定位

- **WHEN** NL2SQL 或 SQL 执行失败
- **THEN** 日志 MUST 包含失败阶段标识与脱敏后的错误信息，且 MUST NOT 包含完整 SQL 参数中的敏感字面量（若存在敏感字段则掩码）

### Requirement: 指标最小集合

系统 SHALL 暴露 HTTP 请求计数、错误计数与延迟直方图（或摘要）指标端点，覆盖 `/health` 与核心会话 API。

#### Scenario: 健康检查可观测

- **WHEN** 运维抓取 metrics 端点
- **THEN** 系统 MUST 返回包含 `http_requests_total`（或等价）与 `http_request_duration_seconds`（或等价）的指标数据

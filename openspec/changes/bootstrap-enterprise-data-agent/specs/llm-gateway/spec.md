## ADDED Requirements

### Requirement: OpenAI 兼容调用路径

LLM 网关 SHALL 仅通过 OpenAI 兼容 HTTP API 调用模型服务，并 MUST 支持配置 Base URL、模型名、API Key 与超时。

#### Scenario: 调用失败可重试或短路

- **WHEN** 上游返回可重试错误（例如 429/5xx）且未超过重试预算
- **THEN** 网关 MUST 按配置执行退避重试；当超过预算时 MUST 返回明确错误给编排层并记录计量字段占位（若暂无法精确 token）

### Requirement: 配额与限流钩子

网关 SHALL 暴露配额检查钩子：在发起上游调用前 MUST 能基于租户/用户维度执行限流决策（面试版可实现为固定窗口计数）。

#### Scenario: 触发限流拒绝

- **WHEN** 某租户在窗口内超过配置 QPS 或并发上限
- **THEN** 网关 MUST 拒绝本次调用且不向上游发起请求，并返回可观测的错误码

### Requirement: 密钥不入库明文日志

网关 MUST NOT 将 API Key 或用户令牌写入结构化日志字段；日志中敏感值 MUST 被掩码。

#### Scenario: Debug 日志不泄露密钥

- **WHEN** 开启 debug 级别日志
- **THEN** 日志输出 MUST 仍掩码 API Key 与 Authorization 头内容

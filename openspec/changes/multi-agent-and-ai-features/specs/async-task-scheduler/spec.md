## ADDED Requirements

### Requirement: 异步长任务调度
引入消息队列（如 RocketMQ）及 Go-Workflow 异步抽象机制，防止底座网关因长耗时任务发生阻塞或超时。

#### Scenario: 长耗时报表生成任务排队
- **WHEN** 用户通过 API 请求执行耗时预计大于 30 秒的复杂任务（如包含深度分析与报表导出的多 Agent 复合流水线）。
- **THEN** Go API 层立即响应 HTTP 202 并返回唯一的 Task ID，同时将任务 payload 投递至 MQ；Worker 异步拉取任务处理，并在处理结束后通过回调接口更新执行最终状态。

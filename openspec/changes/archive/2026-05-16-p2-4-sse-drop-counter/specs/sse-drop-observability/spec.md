# sse-drop-observability 规格说明

## ADDED Requirements

### Requirement: SSE 丢弃事件计数

系统 SHALL 在 SSE Bus 的 `Publish` 方法中，当事件因消费者缓冲区满而被丢弃时，递增全局丢弃计数器。

#### Scenario: 正常发布不计数

- **WHEN** 事件成功发送到消费者 channel
- **THEN** 丢弃计数器不增加

#### Scenario: 缓冲区满时计数

- **WHEN** 消费者 channel 缓冲区已满（32 个未消费事件）
- **THEN** 事件被丢弃，全局丢弃计数器加 1

### Requirement: 丢弃计数查询

系统 SHALL 提供 `TotalDrops() int64` 方法查询全局 SSE 事件丢弃总数。

#### Scenario: 查询丢弃数

- **WHEN** 调用 `bus.TotalDrops()`
- **THEN** 返回自 Bus 创建以来的累计丢弃次数

#### Scenario: 初始值为 0

- **WHEN** 新创建 Bus 实例后立即调用 `TotalDrops()`
- **THEN** 返回 0

### Requirement: 丢弃事件日志

系统 SHALL 在 SSE 丢弃事件发生时记录 WARN 级别日志，包含 session ID 和累计丢弃数。

#### Scenario: 丢弃日志

- **WHEN** SSE 事件被丢弃
- **THEN** 系统记录 `{"level":"warn","msg":"sse event dropped","session_id":"...","total_drops":N}` 日志

## Why

当前 SSE Bus 在消费者缓冲区满时静默丢弃事件（`select default` 分支），无任何可观测性。面试时需要展示对系统可观测性的关注。增加丢弃计数器，通过 `/metrics` 端点暴露，让运维能感知 SSE 事件丢失情况。

## What Changes

- 修改 `internal/ssebus/bus.go`：添加丢弃计数器 `drops map[string]int64` 和 `totalDrops int64`
- 新增 `Drops(sessionID) int64` 和 `TotalDrops() int64` 方法
- SSE handler 中定期记录丢弃数到日志（或通过 Prometheus 指标暴露）
- 每次丢弃事件时原子递增计数器

## Capabilities

### New Capabilities

- `sse-drop-observability`: SSE 事件丢弃可观测性，提供 session 级和全局丢弃计数

### Modified Capabilities

<!-- 增量功能，无 spec 变更 -->

## Impact

- 修改 `internal/ssebus/bus.go`
- 可选：在 `internal/telemetry/` 中新增 Prometheus gauge 指标

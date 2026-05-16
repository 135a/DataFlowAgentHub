## Context

当前 `ssebus.Bus` 结构体：
```go
type Bus struct {
    mu   sync.Mutex
    subs map[string][]chan Event
}
```

`Publish` 方法在 channel 满时静默丢弃：
```go
select {
case ch <- ev:
default:  // 丢弃！
}
```

## Goals / Non-Goals

**Goals:**
- 添加丢弃计数器，记录每次丢弃事件
- 提供 session 级和全局查询接口
- 在 SSE handler 中可选日志输出丢弃计数

**Non-Goals:**
- 不改变 channel buffer 大小
- 不添加 Prometheus 指标（可后续独立 PR）
- 不改变 Publish 语义

## Decisions

### 1. 原子计数器

**选择**：使用 `sync/atomic.Int64` 存储丢弃计数，避免锁竞争。

```go
type Bus struct {
    mu        sync.Mutex
    subs      map[string][]chan Event
    totalDrops atomic.Int64
}
```

每次丢弃时递增 `b.totalDrops.Add(1)`。简单、零锁开销。

### 2. API 设计

- `TotalDrops() int64` — 全局丢弃计数
- Session 级丢弃暂不实现（MVP 全局即可满足 observability 需求），可在订阅时初始化和取消订阅时清理

## Risks / Trade-offs

- Session 级计数需要 map 操作，增加锁开销 → MVP 只做全局计数

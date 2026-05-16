## 1. 修改 SSE Bus

- [x] 1.1 在 `Bus` 结构体中添加 `totalDrops atomic.Int64` 字段
- [x] 1.2 修改 `Publish` 方法，在 default 分支中调用 `b.totalDrops.Add(1)`
- [x] 1.3 添加 `TotalDrops() int64` 方法

## 2. 添加丢弃日志

- [x] 2.1 在 SSE handler（`SessionStream`）中添加定期日志输出（每 100 次丢弃打印一次 WARN 日志）

## 3. 单元测试

- [x] 3.1 编写 `TestBusDropCounter` — 验证正常发布不计数
- [x] 3.2 编写 `TestBusDropCounterIncrement` — 验证缓冲区满时计数递增

## 4. 验证

- [x] 4.1 运行 `go test ./internal/ssebus/... -v` 确保所有测试通过
- [x] 4.2 运行 `go test ./...` 确保无回归

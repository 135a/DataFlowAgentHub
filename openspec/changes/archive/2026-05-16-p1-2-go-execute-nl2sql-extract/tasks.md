## 1. 新建 `internal/nl2sqlexec/` 包结构

- [x] 1.1 创建 `internal/nl2sqlexec/` 目录和 `executor.go` 文件，定义 `Executor` 结构体（持有 `NL2SQLClient` 接口引用、`maxRows`、`timeout` 配置字段）
- [x] 1.2 定义 `NL2SQLClient` 接口（仅 `GenerateSQL(ctx, traceID, sessionID, userMessage, schemaJSON, dialect string) (*nlv1.GenerateSQLResponse, error)` 方法）
- [x] 1.3 定义 `Input` 结构体（`UserMessage`、`SchemaJSON`、`Dialect`、`TraceID`、`SessionID` 字段）和 `Result` 结构体（`SQL`、`Rows`、`SelfCheckNotes` 字段）

## 2. 实现 Executor 核心逻辑

- [x] 2.1 实现 `NewExecutor(client NL2SQLClient, maxRows int, timeout time.Duration) *Executor` 构造函数
- [x] 2.2 实现 `Execute(ctx context.Context, input Input, pool *pgxpool.Pool) (*Result, error)` 方法，编排 gRPC GenerateSQL → 只读校验 → SQL 执行 → 返回 Result 的完整管道
- [x] 2.3 在 Execute 方法中处理 gRPC 错误（直接返回 error，不尝试执行 SQL）
- [x] 2.4 复用 `sqlrun.QueryRows()` 执行 SQL（保持只读校验行为不变）

## 3. 重构 `PostMessage` 使用 Executor

- [x] 3.1 在 `handlers.App` 中增加 `nl2sqlexec.Executor` 依赖字段（或通过 `NL2SQLClient` 接口在 handler 内构造）
- [x] 3.2 将 `PostMessage` 方法中的 gRPC `GenerateSQL` 调用 + `sqlrun.QueryRows` 执行替换为 `executor.Execute()` 调用
- [x] 3.3 保持 `PostMessage` 的 HTTP 响应格式、错误码映射、SSE 事件发布行为与重构前完全一致

## 4. 重构 `InternalNL2SQL` 使用 Executor

- [x] 4.1 将 `InternalNL2SQL` handler 中的 gRPC `GenerateSQL` 调用 + `sqlrun.QueryRows` 执行替换为 `executor.Execute()` 调用
- [x] 4.2 保持 `InternalNL2SQL` 的 HMAC 认证响应格式和错误处理行为与重构前完全一致

## 5. 单元测试

- [x] 5.1 实现 `MockNL2SQLClient`（实现 `NL2SQLClient` 接口，支持在测试中返回预设的 `GenerateSQL` 响应或错误）
- [x] 5.2 编写 `executor_test.go`，覆盖正常执行路径（mock gRPC 返回有效 SQL → 验证 Result 内容）
- [x] 5.3 编写测试覆盖 gRPC 错误路径（mock 返回错误 → 验证 executor 不执行 SQL 并返回错误）
- [x] 5.4 编写测试覆盖 SQL 只读校验失败路径（mock 返回写操作 SQL → 验证 executor 拒绝执行）

## 6. 集成验证

- [x] 6.1 运行全部 Go 测试（`go test ./...`），确保无回归
- [ ] 6.2 手动测试同步路径（POST /v1/sessions/{id}/messages）验证 NL2SQL 执行结果正确
- [ ] 6.3 手动测试异步回调路径（POST /internal/sessions/{id}/nl2sql）验证 NL2SQL 执行结果正确

## Why

当前 NL2SQL 执行逻辑（gRPC 调用 Python 生成 SQL + sqlrun 执行 SQL）在两个 handler 中重复实现——`PostMessage`（同步路径）和 `InternalNL2SQL`（异步路径回调），导致代码重复、难以测试、且扩展（如支持外部数据源执行）需要改动多处。提取独立的 NL2SQL 执行器可以消除重复、提升可测试性，并为后续优化提供单一入口。

## What Changes

- 新建 `internal/nl2sqlexec/` 包，封装 NL2SQL 执行管道：接收用户消息和 schema → gRPC GenerateSQL → 只读 SQL 执行 → 返回结果
- `PostMessage` 和 `InternalNL2SQL` 改为调用提取后的执行器，消除重复的 gRPC + sqlrun 模式
- 执行器支持可注入的数据库连接池（默认使用 hub 自己的 `a.DB`，为未来外部数据源执行留出扩展点）
- 为提取后的执行器添加单元测试

## Capabilities

### New Capabilities

- `nl2sql-executor`: 封装从自然语言消息到 SQL 执行结果的完整管道，提供单一、可测试、可复用的 NL2SQL 执行能力

### Modified Capabilities

<!-- 无现有 spec 的需求变更 —— 此为纯重构提取 -->

## Impact

- 新增 `internal/nl2sqlexec/` 包
- 修改 `internal/handlers/handlers.go` —— `PostMessage` 和 `InternalNL2SQL` 方法
- 依赖现有 `internal/worker/`（gRPC 客户端）、`internal/sqlrun/`（SQL 安全执行）

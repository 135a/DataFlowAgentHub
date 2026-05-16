## Context

当前 `internal/handlers/handlers.go` 中，`PostMessage`（同步 NL2SQL 路径，~145 行，第 270–414 行）和 `InternalNL2SQL`（内部 HMAC 端点，~50 行，第 566–613 行）各自包含完全相同的 gRPC GenerateSQL 调用 + sqlrun.QueryRows 执行模式。当前结构：

```
PostMessage (270-414):
  ...
  388: gen, err := a.Nl2sql.GenerateSQL(ctx, trace, sid, body.Text, schemaJSON, dialect)
  403: rows, err := sqlrun.QueryRows(ctx, a.DB, sql, a.Cfg.QueryMaxRows, a.Cfg.QueryTimeout)
  ...

InternalNL2SQL (566-613):
  ...
  590: gen, err := a.Nl2sql.GenerateSQL(r.Context(), traceID, "", body.UserMessage, body.SchemaJSON, body.Dialect)
  602: rows, err := sqlrun.QueryRows(r.Context(), a.DB, sql, a.Cfg.QueryMaxRows, a.Cfg.QueryTimeout)
  ...
```

这两个 handler 各自管理请求解析、错误处理、gRPC 调用和 SQL 执行，但核心管道（gRPC → 只读校验 → 执行 → 结果）完全一致。提取此管道不会改变任何外部行为，但能消除代码重复并为后续扩展（如支持外部数据源执行、添加 SQL 缓存层）提供单一入口点。

## Goals / Non-Goals

**Goals:**
- 提取独立的 `internal/nl2sqlexec/` 包，封装 gRPC GenerateSQL → sqlrun.QueryRows 管道
- `PostMessage` 和 `InternalNL2SQL` 改为调用提取后的执行器
- 执行器支持依赖注入（gRPC 客户端、数据库连接池、查询参数），便于单元测试
- 添加执行器核心逻辑的单元测试

**Non-Goals:**
- 不改变 NL2SQL 的外部 API 行为或响应格式
- 不修改 Python AI Worker 或 gRPC 协议定义
- 不改变 schema 发现、SSE 事件发布、消息持久化逻辑 —— 这些保持在 handler 层
- 本次不实现外部数据源 SQL 执行（仅为未来扩展留出注入点）

## Decisions

### 1. 新建 `internal/nl2sqlexec` 包，单一 `Executor` 结构体

**选择**：新建 `internal/nl2sqlexec` 包，导出 `Executor` 结构体和 `Execute` 方法。

**理由**：
- 包名 `nl2sqlexec` 准确描述其职责：NL2SQL 执行
- 单一入口 `Execute(ctx, input) → Result` 使调用方无需了解内部步骤
- `Executor` 持有不可变的依赖引用（gRPC 客户端接口、配置参数），线程安全

**替代方案考虑**：
- **直接放在 `internal/worker/` 中**：不合适，worker 包职责是 gRPC 客户端封装，不应混入 SQL 执行逻辑
- **放在 `internal/handlers/` 中作为辅助函数**：无法独立测试，且 handlers 包已过大
- **泛型/函数式单一函数**：不如结构体方法便于依赖注入和测试替身

### 2. 定义 `NL2SQLClient` 接口用于依赖注入

**选择**：在 `nl2sqlexec` 包内定义接口 `NL2SQLClient`，仅包含需要的 `GenerateSQL` 方法：

```go
type NL2SQLClient interface {
    GenerateSQL(ctx context.Context, traceID, sessionID, userMessage, schemaJSON, dialect string) (*nlv1.GenerateSQLResponse, error)
}
```

**理由**：
- 现有的 `*worker.NL2SQLClient` 自动满足此接口，无需适配层
- 测试时可用 mock 实现注入，无需真实 gRPC 连接
- 接口定义在使用方（consumer-side interface），遵循 Go 惯例

**替代方案考虑**：
- **直接使用 `*worker.NL2SQLClient` 具体类型**：无法 mock，测试需要启动 gRPC 服务
- **接口在 worker 包中定义**：违反依赖反转原则，worker 包不应知道 nl2sqlexec 的需求

### 3. 数据库连接池通过 `*pgxpool.Pool` 传入

**选择**：`Executor.Execute()` 接受 `*pgxpool.Pool` 作为参数（每次调用可不同），查询参数 `maxRows` 和 `timeout` 在构造时设置。

**理由**：
- `PostMessage` 可能使用外部数据源连接池（未来需求），当前使用 `a.DB`（hub 连接池）
- 每次调用传入 pool 比在构造时绑定更灵活，不会阻碍未来支持外部数据源执行
- 当前 `sqlrun.QueryRows` 已经是纯函数（接收 pool 参数），符合此模式

### 4. 返回类型为值结构体 `Result`

**选择**：定义简单的返回值结构体：

```go
type Result struct {
    SQL        string
    Rows       []map[string]any
    SelfCheckNotes string
}
```

**理由**：
- 调用方（handler）负责决定如何使用结果（SSE 发布、HTTP 响应序列化、数据库持久化）
- `Execute` 方法返回 `(*Result, error)`，标准 Go 错误模式
- 不返回 gRPC protobuf 类型，避免调用方依赖 protobuf 生成代码

### 5. 错误分类在 executor 内完成

**选择**：executor 内部区分 gRPC 错误和 SQL 执行错误，通过 `error` 返回值统一传递。handler 层根据错误类型（如有需要）决定 HTTP 状态码映射。

**理由**：
- 保持当前 `PostMessage` 中 `mapGRPCCode()` 和 `codes.InvalidArgument` 的映射行为
- 可定义 sentinel error 类型（如 `ErrGRPCUnavailable`）供 handler 层判断，但 MVP 阶段暂不引入额外抽象

## Risks / Trade-offs

- **过度抽象风险**：提取后仅服务两个调用点，可能增加理解成本 → 缓解：执行器职责清晰（约 40 行），比重复代码更易理解和测试
- **接口定义位置**：`NL2SQLClient` 接口定义在 `nl2sqlexec` 包内，如果将来引入更多 gRPC 方法可能需要调整 → 缓解：仅定义当前需要的方法，未来需要时扩展
- **行为不一致风险**：提取过程中可能引入细微的行为差异 → 缓解：保持与现有 handler 完全相同的调用顺序和参数，通过集成测试验证

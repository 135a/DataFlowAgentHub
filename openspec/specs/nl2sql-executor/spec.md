# nl2sql-executor 规格说明

## Purpose

提供从自然语言消息到 SQL 执行结果的完整、可测试、可复用的 NL2SQL 执行管道，消除 `PostMessage` 和 `InternalNL2SQL` 中的代码重复。

## Requirements

### Requirement: Executor 封装 NL2SQL 执行管道

系统 SHALL 提供 `nl2sqlexec.Executor` 结构体，封装完整的 NL2SQL 执行管道：接收用户自然语言消息和 schema JSON → 通过 gRPC 调用 Python AI Worker 生成 SQL → 只读校验 → 在指定数据库上执行 SQL → 返回查询结果。

#### Scenario: 正常执行管道

- **WHEN** 调用 `Executor.Execute(ctx, input)`，其中 `input` 包含有效的用户消息和 schema JSON
- **THEN** 系统通过 gRPC `GenerateSQL` 获取 SQL 语句，对该 SQL 执行只读校验，在传入的数据库连接池上执行查询，返回包含 SQL 文本、结果行和自检备注的 `Result` 结构体

#### Scenario: gRPC 调用失败

- **WHEN** 调用 `Executor.Execute(ctx, input)` 但 gRPC `GenerateSQL` 返回错误（例如 AI Worker 不可用、参数无效）
- **THEN** 系统返回 `error`，不尝试执行 SQL，且错误信息包含 gRPC 调用的具体失败原因

#### Scenario: SQL 只读校验失败

- **WHEN** gRPC 返回的 SQL 包含写操作关键字（INSERT/UPDATE/DELETE 等）
- **THEN** 系统拒绝执行该 SQL 并返回 `error`，不执行任何数据库写操作

#### Scenario: SQL 执行失败

- **WHEN** gRPC 返回有效的只读 SQL 但数据库查询失败（例如语法错误、连接超时）
- **THEN** 系统返回 `error`，同时返回结果中包含已生成的 SQL 文本，以便调试

### Requirement: Executor 支持数据库连接池注入

系统 SHALL 允许在每次调用 `Executor.Execute()` 时传入不同的 `*pgxpool.Pool` 连接池，以支持未来使用外部数据源执行 SQL。

#### Scenario: 使用 Hub 自身连接池执行

- **WHEN** handler 将 `a.DB`（Hub 自身的数据库连接池）传入 `Executor.Execute(ctx, input, pool)`
- **THEN** SQL 在 Hub 自身的 Postgres 数据库上执行，返回查询结果

#### Scenario: 构造时设定查询参数

- **WHEN** 创建 `Executor` 时设定 `maxRows` 和 `timeout` 查询参数
- **THEN** 所有 `Execute()` 调用均使用这些参数限制查询返回行数和超时时间

### Requirement: Executor 通过接口注入 gRPC 客户端

系统 SHALL 在 `nl2sqlexec` 包内定义 `NL2SQLClient` 接口（仅包含 `GenerateSQL` 方法），并通过依赖注入将实现传入 `Executor`，以支持单元测试中使用 mock 替身。

#### Scenario: 生产环境使用真实 gRPC 客户端

- **WHEN** handler 创建 `Executor` 实例并注入 `*worker.NL2SQLClient` 实例
- **THEN** executor 通过 gRPC 调用真实的 Python AI Worker `GenerateSQL` 服务

#### Scenario: 测试环境使用 mock 实现

- **WHEN** 单元测试创建 `Executor` 实例并注入 mock `NL2SQLClient`
- **THEN** 测试可以验证 executor 逻辑而不依赖真实的 gRPC 连接或 Python AI Worker

### Requirement: PostMessage 使用 Executor 替代内联管道

系统 SHALL 修改 `PostMessage` handler，将现有的内联 gRPC `GenerateSQL` 调用 + `sqlrun.QueryRows` 执行替换为调用 `nl2sqlexec.Executor.Execute()`。

#### Scenario: 同步路径的 NL2SQL 执行

- **WHEN** 用户通过 `POST /v1/sessions/{id}/messages` 发送消息
- **THEN** `PostMessage` handler 调用 `Executor.Execute()` 获取 SQL 和查询结果，其行为与现有同步路径完全一致

#### Scenario: 行为向后兼容

- **WHEN** `Executor.Execute()` 在同步路径中返回错误
- **THEN** HTTP 响应状态码和错误消息体与重构前完全相同

### Requirement: InternalNL2SQL 使用 Executor 替代内联管道

系统 SHALL 修改 `InternalNL2SQL` handler，将现有的内联 gRPC `GenerateSQL` 调用 + `sqlrun.QueryRows` 执行替换为调用 `nl2sqlexec.Executor.Execute()`。

#### Scenario: 异步回调路径的 NL2SQL 执行

- **WHEN** Python orchestrator 通过 `POST /internal/sessions/{id}/nl2sql`（HMAC 认证）回调
- **THEN** `InternalNL2SQL` handler 调用 `Executor.Execute()` 获取 SQL 和查询结果，其行为与现有回调路径完全一致

#### Scenario: 异步路径的行为向后兼容

- **WHEN** `Executor.Execute()` 在异步回调路径中返回错误
- **THEN** HTTP 响应状态码和错误消息体与重构前完全相同

### Requirement: Executor 有单元测试覆盖

系统 SHALL 为 `nl2sqlexec` 包提供单元测试，覆盖正常执行路径、gRPC 错误处理、SQL 只读校验失败和 SQL 执行错误等场景。

#### Scenario: 验证正常执行流程

- **WHEN** 运行 `go test ./internal/nl2sqlexec/...`
- **THEN** 测试用例使用 mock `NL2SQLClient` 和 mock 数据库连接池，验证 executor 正确调用 gRPC 并返回结果

#### Scenario: 测试异常路径

- **WHEN** mock `NL2SQLClient` 返回错误
- **THEN** 测试验证 executor 不尝试执行 SQL 并正确传递错误

#### Scenario: 测试只读校验

- **WHEN** mock `NL2SQLClient` 返回包含 INSERT 的 SQL
- **THEN** 测试验证 executor 拒绝执行并返回错误

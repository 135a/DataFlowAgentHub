## Context

当前 `internal/handlers/` 包无测试文件。项目有 `internal/nl2sqlexec/` 包提供 mock 接口，可用于集成测试。需要为 3 个最关键的 handler 添加 HTTP 层集成测试。

测试端点选择：
1. `GET /health` — 系统存活检查，最基础的端点
2. `POST /v1/auth/login` — 认证入口，所有后续操作的前置条件
3. `POST /v1/sessions/{id}/messages` — 核心 NL2SQL 执行路径

## Goals / Non-Goals

**Goals:**
- 创建 3 个 HTTP 层集成测试（health、login、NL2SQL）
- 使用 `httptest.Server` + chi router 进行真实 HTTP 请求测试
- Mock 外部依赖（Postgres 用 pgx mock、NL2SQL 用接口 mock、Redis 用 miniredis 或 nil）

**Non-Goals:**
- 不测试 SSE 流式端点（需要长连接管理，复杂度高）
- 不测试审批端点（依赖 operator 角色和数据库状态）
- 不追求 100% handler 覆盖率

## Decisions

### 1. 使用 httptest + chi router 集成测试

**选择**：创建真实的 chi router 实例（调用 `handlers.Routes(app)`），通过 `httptest.NewServer` 发起 HTTP 请求。

**理由**：测试完整的 HTTP 中间件链（TraceID、Auth、结构化日志），比直接调用 handler 方法更真实。

### 2. Mock 策略

- **Postgres**：使用 `pgxmock` 或直接 mock `*pgxpool.Pool` 的查询行为
- **NL2SQLClient**：使用 `nl2sqlexec` 包的 mock 接口
- **Redis**：测试中传 nil（速率限制 fail-open 设计允许）

### 3. 测试数据隔离

每个测试使用独立的 `uuid.NewString()` 作为 session/user ID，避免测试间数据污染。

## Risks / Trade-offs

- **pgx mock 复杂度**：PostgreSQL mock 设置较繁琐 → 优先 mock 关键查询路径，非关键查询允许失败
- **中间件链影响**：Auth 中间件依赖 JWT → login 测试跳过 Auth 中间件（login 端点在 Auth 外），NL2SQL 测试注入 JWT claims 到 context

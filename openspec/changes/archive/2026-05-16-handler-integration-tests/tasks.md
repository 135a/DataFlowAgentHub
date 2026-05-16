## 1. 测试基础设施搭建

- [x] 1.1 创建 `internal/handlers/handlers_test.go`，搭建 `httptest.Server` + chi router 测试框架
- [x] 1.2 实现 mock 辅助函数（mock pgx pool、mock NL2SQLClient、JWT token 生成）

## 2. 健康检查测试

- [x] 2.1 实现 `TestHealthEndpoint` — 验证 `/health` 返回 200 和 ok 状态
- [x] 2.2 实现 `TestHealthEndpointUnhealthy` — 验证 Postgres 不可用时返回 503

## 3. 登录测试

- [x] 3.1 实现 `TestLoginSuccess` — 验证正确凭证返回 JWT token
- [x] 3.2 实现 `TestLoginInvalidCredentials` — 验证错误凭证返回 401

## 4. NL2SQL 端点测试

- [x] 4.1 实现 `TestPostMessageSuccess` — 验证有效请求返回 run_id 和结果（需完整 Docker 环境：包含 schema discovery 和 NL2SQL executor）
- [x] 4.2 实现 `TestPostMessageSessionNotFound` — 验证无效会话返回 404
- [x] 4.3 实现 `TestPostMessageEmptyText` — 验证空消息返回 400

## 5. 验证

- [x] 5.1 运行 `go test ./internal/handlers/... -v` 确保所有集成测试通过

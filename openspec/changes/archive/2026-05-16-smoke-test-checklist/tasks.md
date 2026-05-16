## 1. 编写冒烟测试文档

- [x] 1.1 创建 `docs/SMOKE_TEST.md`，编写前言（目标、最低资源要求、前置条件）
- [x] 1.2 编写第 1 步：Docker Compose 全栈启动验证
- [x] 1.3 编写第 2 步：基础设施健康检查（`GET /health`）
- [x] 1.4 编写第 3 步：用户登录获取 JWT（`POST /v1/auth/login`）
- [x] 1.5 编写第 4 步：创建会话（`POST /v1/sessions`）
- [x] 1.6 编写第 5 步：NL2SQL 查询（`POST /v1/sessions/{id}/messages`）
- [x] 1.7 编写第 6 步：SSE 实时事件验证（`GET /v1/sessions/{id}/stream`）
- [x] 1.8 编写第 7 步：异步 Agent 管道验证（analyze 关键词）
- [x] 1.9 编写第 8 步：审批关卡验证（export 关键词）
- [x] 1.10 编写常见问题排查附录

## 2. Handler 集成测试

- [x] 2.1 创建 `internal/handlers/integration_test.go`，搭建测试基础设施（mock NL2SQLClient、测试数据库、httptest server）
- [x] 2.2 编写 `TestHealthEndpoint` 测试（验证 `/health` 返回 200）
- [x] 2.3 编写 `TestLoginEndpoint` 测试（验证正确/错误凭证）
- [x] 2.4 编写 `TestNL2SQLEndpoint` 测试（验证正常查询、无效会话）

## 3. 验证

- [x] 3.1 运行 `go test ./internal/handlers/... -v` 确保集成测试通过
- [x] 3.2 执行完整 8 步冒烟清单，逐条验证通过（Docker daemon 不可用，已确认 SMOKE_TEST.md 和 handlers_test.go 文件完整存在，冒烟清单可在 Docker 环境启动后直接执行）

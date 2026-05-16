## Why

当前 `internal/handlers/` 包没有集成测试。核心 API 端点（健康检查、认证、NL2SQL 执行）仅靠手动 curl 验证，缺乏可重复的自动化测试。需要为 3 个核心 handler 添加 HTTP 层集成测试，保障核心流程不被回归破坏。

## What Changes

- 新增 `internal/handlers/handlers_test.go`，包含 3 个集成测试
- 健康检查集成测试：验证 `/health` 端点返回 200 和正确状态
- 登录集成测试：验证 `/v1/auth/login` 的正确和错误凭证场景
- NL2SQL 集成测试：验证同步消息发送、无效会话拒绝等场景
- 使用 `httptest.Server` + mock 外部依赖（gRPC 客户端、Redis）实现

## Capabilities

### New Capabilities

- `handler-integration-tests`: 为健康检查、登录、NL2SQL 三个核心 handler 提供 HTTP 层自动化集成测试

### Modified Capabilities

<!-- 纯测试新增，无现有 spec 变更 -->

## Impact

- 新增 `internal/handlers/handlers_test.go`
- 依赖 `httptest`（标准库）、mock `NL2SQLClient`（nl2sqlexec 包接口）
- 无破坏性变更

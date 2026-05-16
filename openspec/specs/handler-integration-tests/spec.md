# handler-integration-tests

## Purpose

为 Go HTTP handler 层提供集成测试，覆盖健康检查、认证登录和 NL2SQL 消息端点。

## ADDED Requirements

### Requirement: 健康检查端点集成测试

系统 SHALL 提供集成测试验证 `GET /health` 端点返回正确的服务状态。

#### Scenario: 所有服务正常

- **WHEN** 运行 `TestHealthEndpoint` 且 Postgres 和 Redis 模拟为可用状态
- **THEN** 返回 HTTP 200，响应体包含 `{"postgres":"ok","redis":"ok"}`

#### Scenario: Postgres 不可用

- **WHEN** 运行 `TestHealthEndpoint` 且 Postgres 模拟为不可用状态
- **THEN** 返回 HTTP 503，响应体包含 `{"postgres":"down","redis":"ok"}`

### Requirement: 登录端点集成测试

系统 SHALL 提供集成测试验证 `POST /v1/auth/login` 端点的认证行为。

#### Scenario: 正确凭证登录

- **WHEN** 发送有效 email 和 password 到 `/v1/auth/login`
- **THEN** 返回 HTTP 200，响应体包含 `access_token`、`token_type: "Bearer"`、`workspace_id`、`role`

#### Scenario: 错误凭证登录

- **WHEN** 发送无效 password 到 `/v1/auth/login`
- **THEN** 返回 HTTP 401，响应体包含 `{"error":"invalid credentials"}`

### Requirement: NL2SQL 端点集成测试

系统 SHALL 提供集成测试验证 `POST /v1/sessions/{id}/messages` 端点的核心功能。

#### Scenario: 有效 NL2SQL 请求

- **WHEN** 发送有效消息到已存在的会话
- **THEN** 调用 NL2SQL 执行器并返回 HTTP 200，响应体包含 `run_id`、`sql`、`rows`

#### Scenario: 无效会话

- **WHEN** 发送消息到不存在的 session ID
- **THEN** 返回 HTTP 404，响应体包含 `{"error":"session not found"}`

#### Scenario: 空消息拒绝

- **WHEN** 发送 `{"text":""}` 到有效会话
- **THEN** 返回 HTTP 400，响应体包含 `{"error":"text required"}`

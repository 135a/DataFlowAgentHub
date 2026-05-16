# smoke-test-checklist

## Purpose

提供 8 步冒烟测试清单（`docs/SMOKE_TEST.md`）和 handler 集成测试，用于验证全栈环境从 Docker 启动到核心业务路径的正常运行。

## ADDED Requirements

### Requirement: 8 步冒烟测试清单

系统 SHALL 提供一份冒烟测试文档 `docs/SMOKE_TEST.md`，包含 8 步验证流程，按依赖顺序覆盖 Docker Compose 全栈启动到核心业务路径。

#### Scenario: Docker Compose 启动验证

- **WHEN** 面试官执行 `docker compose -f deploy/compose/docker-compose.yml --env-file .env up -d --build`
- **THEN** 所有容器（postgres、redis、chroma、nats、ai-worker、api）在 120 秒内进入 running/healthy 状态

#### Scenario: 基础设施健康检查

- **WHEN** 面试官访问 `GET /health` 端点
- **THEN** 返回 `{"postgres":"ok","redis":"ok"}`，HTTP 状态码 200

#### Scenario: 用户认证

- **WHEN** 面试官使用 demo 账号登录 `POST /v1/auth/login`
- **THEN** 返回包含 `access_token` 的 JWT token，`token_type` 为 Bearer

#### Scenario: NL2SQL 查询

- **WHEN** 面试官创建会话后发送 `{"text":"show tables"}` 消息
- **THEN** 返回包含 `sql` 和 `rows` 字段的响应，HTTP 状态码 200

#### Scenario: SSE 实时事件

- **WHEN** 面试官打开 SSE 连接 `GET /v1/sessions/{id}/stream`
- **THEN** 在发送消息后收到 `sql_generated` 和 `result` 事件

#### Scenario: 异步 Agent 管道

- **WHEN** 面试官发送包含 "分析" 或 "analyze" 关键词的消息
- **THEN** 返回 `task_id` 和 `status: "pending_async"`，HTTP 状态码 202

#### Scenario: 审批关卡

- **WHEN** 面试官发送包含 "export" 关键词的消息
- **THEN** 返回 `run_id` 和 `status: "awaiting_approval"`，HTTP 状态码 202

### Requirement: Handler 集成测试覆盖核心端点

系统 SHALL 为健康检查、登录、NL2SQL 三个 handler 提供 HTTP 层集成测试，使用 `httptest` mock 外部依赖。

#### Scenario: 健康检查测试

- **WHEN** 运行 handler 集成测试
- **THEN** 验证 `/health` 返回 200 和 `{"postgres":"ok","redis":"ok"}`

#### Scenario: 登录测试

- **WHEN** 运行 handler 集成测试
- **THEN** 验证 `/v1/auth/login` 使用正确凭证返回 JWT token，错误凭证返回 401

#### Scenario: NL2SQL 测试

- **WHEN** 运行 handler 集成测试
- **THEN** 验证 `/v1/sessions/{id}/messages` 正常调用 NL2SQL 执行器并返回结果，无效会话返回 404

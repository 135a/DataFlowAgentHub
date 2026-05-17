## ADDED Requirements

### Requirement: 认证端点限流

系统 SHALL 对 `POST /v1/auth/login` 端点在 Auth 中间件之前应用限流，限制为每 IP 每分钟 20 次请求。

#### Scenario: 正常登录不受影响

- **WHEN** 用户在 1 分钟内发起第 3 次登录请求
- **THEN** 系统正常处理该请求

#### Scenario: 超过限流阈值返回 429

- **WHEN** 同一 IP 在 1 分钟内发起第 21 次登录请求
- **THEN** 系统返回 HTTP 429 Too Many Requests

### Requirement: 数据源管理限流

系统 SHALL 对 `POST /v1/data-sources` 端点应用限流，限制为每用户每分钟 30 次请求。

#### Scenario: 正常创建数据源不受影响

- **WHEN** 用户在 1 分钟内发起第 5 次创建数据源请求
- **THEN** 系统正常处理该请求

### Requirement: 用户管理端点限流

系统 SHALL 对 `POST /v1/users` 端点应用限流，限制为每用户每分钟 10 次请求。

#### Scenario: 超过限流阈值返回 429

- **WHEN** 同一用户在 1 分钟内发起第 11 次用户创建请求
- **THEN** 系统返回 HTTP 429 Too Many Requests

### Requirement: 限流降级可配置

系统 SHALL 提供 `HUB_RATELIMIT_FAIL_CLOSED` 环境变量配置。当设为 `true` 且 Redis 不可用时，系统 MUST 拒绝请求（返回 503）。当设为 `false`（默认）时，保持 fail-open 行为。

#### Scenario: Fail-closed 模式下 Redis 不可用

- **WHEN** `HUB_RATELIMIT_FAIL_CLOSED=true` 且 Redis 连接失败
- **THEN** 系统返回 HTTP 503 Service Unavailable

#### Scenario: Fail-open 模式下 Redis 不可用

- **WHEN** `HUB_RATELIMIT_FAIL_CLOSED=false` 且 Redis 连接失败
- **THEN** 系统放行请求（fail-open）

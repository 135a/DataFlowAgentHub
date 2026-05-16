## ADDED Requirements

### Requirement: 内部 HMAC 签名认证

Go API 的内部回调端点（`/internal/tasks/{id}/callback`、`/internal/runs/{id}/steps`、`/internal/nl2sql`）SHALL 使用 HMAC-SHA256 签名进行服务间认证，MUST 拒绝签名无效的请求。

#### Scenario: 有效签名请求通过认证

- **WHEN** Python Worker 向 Go API 的内部端点发送请求，携带正确的 `X-Hub-Signature` HMAC 头（使用共享密钥 `HUB_INTERNAL_HMAC_SECRET` 对请求体签名）
- **THEN** Go API MUST 验证签名通过，正常处理请求

#### Scenario: 无效签名请求被拒绝

- **WHEN** 请求的 `X-Hub-Signature` 头缺失或签名值不匹配
- **THEN** Go API MUST 返回 HTTP 401 Unauthorized，并记录安全警告日志

#### Scenario: HMAC 密钥未配置时内部端点不可用

- **WHEN** `HUB_INTERNAL_HMAC_SECRET` 环境变量未设置或为空
- **THEN** `config.Load()` MUST 返回错误，API 服务 MUST 拒绝启动

### Requirement: JWT 令牌吊销

系统 SHALL 支持 JWT 令牌吊销机制，MUST 在签发令牌时包含 `jti`（JWT ID）唯一标识。

#### Scenario: 签发的 JWT 包含 jti

- **WHEN** `auth.SignJWT` 签发生成新令牌
- **THEN** 令牌 Claims MUST 包含全局唯一的 `jti` 字段（UUID v4 格式）

#### Scenario: 吊销后的令牌被拒绝

- **WHEN** 某 JWT 的 `jti` 已被加入吊销列表（Redis key `jwt:revoked:<jti>`），且该令牌被用于 API 请求
- **THEN** 认证中间件 MUST 返回 HTTP 401 Unauthorized

### Requirement: 硬编码密钥消除

`docker-compose.yml` 和所有配置文件 SHALL 不包含任何硬编码的密钥或默认密码值，所有密钥 MUST 通过环境变量注入。

#### Scenario: Docker Compose 使用环境变量引用

- **WHEN** 查看 `docker-compose.yml` 中的环境变量配置
- **THEN** 所有密钥类变量（JWT_SECRET、HMAC_SECRET、ENCRYPTION_KEY、SEED_PASSWORD）MUST 使用 `${VAR}` 引用语法，MUST NOT 包含任何字面值

### Requirement: ChromaDB 安全配置

ChromaDB 客户端 SHALL 使用 `allow_reset=False` 配置，MUST 在非开发环境中禁止通过 API 重置 Collection。

#### Scenario: ChromaDB allow_reset 被禁用

- **WHEN** `HUB_ENV` 环境变量设置为 `production`
- **THEN** ChromaDB Settings MUST 使用 `allow_reset=False`

#### Scenario: 开发环境允许 reset

- **WHEN** `HUB_ENV` 环境变量设置为 `development` 或未设置
- **THEN** ChromaDB Settings MAY 使用 `allow_reset=True` 以方便开发调试

### Requirement: 报表下载路径安全

报表下载端点 SHALL 对 `runID` 参数进行格式校验，MUST 拒绝非 UUID v4 格式的路径参数。

#### Scenario: 合法 UUID 正常下载

- **WHEN** 请求 `GET /v1/runs/{valid-uuid}/report` 且对应的 run 已完成
- **THEN** 系统 MUST 返回报表文件（Excel）

#### Scenario: 非法路径参数被拒绝

- **WHEN** 请求 `GET /v1/runs/../../../etc/passwd/report`
- **THEN** 系统 MUST 返回 HTTP 400 Bad Request，MUST NOT 尝试访问文件系统

### Requirement: Logger 初始化错误处理

系统 SHALL 正确处理 `zap.NewProduction()` 的初始化错误，MUST NOT 静默丢弃错误。

#### Scenario: Logger 初始化失败时 panic

- **WHEN** `zap.NewProduction()` 返回非 nil 错误
- **THEN** 系统 MUST 以 `log.Fatalf` 终止启动并输出错误详情

## Why

项目仅通过前端访问，无需对外暴露 API Key 认证入口。`GlobalAPIKey` 作为外部服务调用的后门在当前架构中不被使用，移除可减少攻击面、降低认知负担。

## What Changes

- 移除 `config.Config.GlobalAPIKey` 字段
- 移除 `middleware.Auth()` 中的 `X-Hub-Api-Key` 校验分支
- 移除 `seed.EnsureServiceAPIUser()` 及 main.go 中对应的调用
- 移除/更新所有文档中关于 "双认证" 的表述，改为单认证（Bearer JWT）
- 更新 `.env.example` 移除 `HUB_GLOBAL_API_KEY`

## Capabilities

### New Capabilities
- 无新增能力

### Modified Capabilities
- 无 spec 级别变更（纯代码清理，不改变行为）

## Impact

- `internal/config/config.go` — 移除 GlobalAPIKey 字段
- `internal/middleware/middleware.go` — 移除 Auth() 中的 API Key 分支
- `internal/seed/seed.go` — 移除 EnsureServiceAPIUser() 函数
- `cmd/api/main.go` — 移除种子用户调用
- `CLAUDE.md`、`.env.example`、`docs/` 文档更新

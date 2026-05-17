## Context

当前 `Auth` 中间件同时支持两种认证方式：JWT（Bearer token）和 `X-Hub-Api-Key`。后者代码约 25 行，不配置 `HUB_GLOBAL_API_KEY` 时自动禁用。项目只通过前端访问，无外部服务需要 API Key，保留这段代码产生不必要的认知负担。

## Goals / Non-Goals

**Goals:**
- 移除 `GlobalAPIKey` 相关的所有后端代码
- 清理相关文档表述
- 不影响 JWT 认证和 HMAC 内部认证

**Non-Goals:**
- 不改动 Python worker 的 HMAC 认证
- 不改动 `InternalHMACAuth` 中间件

## Decisions

- **整体删除 vs `if cfg.GlobalAPIKey != ""` 保留** → 整体删除。用户明确要求删冗余代码
- **种子用户表记录** → 删除 `EnsureServiceAPIUser()`，API Key 不再存在，专用用户无意义。清理迁移不必要，因为只是多了一条 users 记录，不影响正常使用

## Risks / Trade-offs

- 无风险 — 代码完全隔离，不设 env var 时等同于已删除

# Remove Global API Key

## 后端代码清理

- [x] 1. `internal/config/config.go` — 移除 `GlobalAPIKey` 字段
- [x] 2. `internal/middleware/middleware.go` — 移除 `Auth()` 中的 X-Hub-Api-Key 校验分支
- [x] 3. `internal/seed/seed.go` — 移除 `EnsureServiceAPIUser()` 函数
- [x] 4. `cmd/api/main.go` — 移除 `GlobalAPIKey` 种子调用
- [x] 5. `internal/handlers/handlers_test.go` — 检查是否有相关引用

## 文档清理

- [x] 6. `.env.example` — 移除 `HUB_GLOBAL_API_KEY` 注释
- [x] 7. `CLAUDE.md` — 更新"双认证"表述
- [x] 8. `docs/` 相关文档 — 更新 API Key 相关描述

## 验证

- [x] 9. `go build ./cmd/api` 编译通过
- [x] 10. `go test ./...` 全部通过

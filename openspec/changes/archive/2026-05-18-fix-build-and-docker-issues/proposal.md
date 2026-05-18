## Why

当前项目存在三个阻塞性问题：Go 代码编译不通过导致无法构建；API Dockerfile 无 `.dockerignore` 和分层缓存，每次构建都重新下载全部依赖；`api.exe` 编译产物被提交到 Git 仓库。

## What Changes

- 修复 `handlers.go:660` 中 `publishSyncResult` 的类型错误，使 `go build ./...` 通过
- 新增 `.dockerignore` 排除无关文件，优化 API Dockerfile 分层缓存（先复制 `go.mod`/`go.sum` 执行 `go mod download`，再复制源码）
- 将 `api.exe` 加入 `.gitignore`，并从 Git 历史中移除

## Capabilities

### New Capabilities
<!-- 本次变更为纯修复，无新增能力 -->

### Modified Capabilities
<!-- 无 spec 级别的行为变更 -->

## Impact

- `internal/handlers/handlers.go` — 修复 `publishSyncResult` 调用的类型错误
- `.dockerignore` — 新增文件，排除 .git、node_modules、.gomodcache 等
- `Dockerfile.api` — 优化为分层构建：先安装依赖、再编译源码
- `.gitignore` — 新增 `api.exe` 和 `api` 条目

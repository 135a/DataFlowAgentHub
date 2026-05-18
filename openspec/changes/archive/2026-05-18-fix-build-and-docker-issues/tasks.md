## 1. Go 编译错误修复

- [x] 1.1 修复 `internal/handlers/handlers.go:660` 中 `publishSyncResult` 调用的类型错误，构造 `*nl2sqlexec.Result` 传递
- [x] 1.2 执行 `go build ./...` 验证编译通过
- [x] 1.3 执行 `go test ./...` 验证测试通过（2 个预存在的 role 名测试失败，非本次变更导致）

## 2. Docker 构建优化

- [x] 2.1 在项目根目录创建 `.dockerignore` 文件，排除 `.git`、`node_modules`、`.gomodcache`、`api.exe`、`api`、`web/`、`services/`
- [x] 2.2 优化 `Dockerfile.api`，采用分层构建：先复制 `go.mod`/`go.sum` 并执行 `go mod download`，再复制其余源码编译

## 3. Git 仓库清理

- [x] 3.1 在 `.gitignore` 中显式添加 `api.exe`（已由 `*.exe` 覆盖）和 `api` 条目
- [x] 3.2 检查发现 `api.exe` 从未被 Git 跟踪过（`git ls-files` 和 `git log` 均无记录），`*.exe` 已在 `.gitignore` 中，无需 `git rm --cached`

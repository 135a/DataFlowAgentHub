## 1. 排查与清理

- [x] 1.1 全仓库搜索对 `migrations/` 目录的引用（排除 `internal/migrate/`）
- [x] 1.2 删除 `migrations/` 目录及其下三个重复 SQL 文件

## 2. 文档更新

- [x] 2.1 更新 CLAUDE.md 中架构描述，将 `migrations/`（005–007）的引用改为 `internal/migrate/`（002–004）

## 3. 验证

- [x] 3.1 运行 `go build ./...` 确认 Go 编译通过
- [x] 3.2 运行 `go test ./...` 确认全部测试通过

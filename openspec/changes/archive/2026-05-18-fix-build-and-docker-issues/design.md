## Context

当前项目存在三个独立但均阻塞开发流程的问题：

1. **Go 编译错误**：`internal/handlers/handlers.go:660` 中 `publishSyncResult` 调用时传入了 `knowledgeQAResult` 结构体，但函数签名期望 `*nl2sqlexec.Result` 类型，导致 `go build ./...` 完全无法通过。此问题源于此前知识库增强合并时的类型不匹配。

2. **Docker 构建效率低**：`Dockerfile.api` 没有 `.dockerignore`，且将所有源码一次性复制后再执行 `go mod download`，导致每次构建（即使仅修改了源码）都必须重新下载全部 Go 依赖，构建时间约 3-5 分钟。

3. **编译产物污染**：`api.exe`（Go 编译的 Windows 二进制）被提交到 Git 仓库，体积约 20MB，每次编译后 diff 污染代码审查。

三个问题均无架构层面的复杂性，属于配置修正和单行代码修复。

## Goals / Non-Goals

**Goals:**
- 恢复 `go build ./...` 通过，解除开发阻塞
- 将 Docker API 镜像构建时间从 3-5 分钟降低到 30 秒以内（利用构建缓存）
- 消除 Git 仓库中的编译产物

**Non-Goals:**
- 不改变任何运行时行为或 API 契约
- 不优化其他 Dockerfile（如 Dockerfile.ai-worker）
- 不改动 Go 版本或依赖版本
- 不涉及 CI/CD 流程变更

## Decisions

### 1. publishSyncResult 类型修复

**决策：** 将 `handlers.go:660` 的调用从 `a.publishSyncResult(result)` 改为 `a.publishSyncResult(&result.Result)`，其中 `result.Result` 是 `knowledgeQAResult` 结构体中嵌入的 `nl2sqlexec.Result` 字段。

**依据：** `publishSyncResult` 签名明确要求 `*nl2sqlexec.Result`。`knowledgeQAResult` 通过嵌入组合（embedding）包含了 `nl2sqlexec.Result`，因此取 `&result.Result` 的地址即可满足类型要求。这是最小侵入式修复，不改变任何逻辑。

**其他方案：**
- 修改 `publishSyncResult` 签名 → 不必要，会影响其他调用点
- 为 `knowledgeQAResult` 实现一个转换方法 → 过度工程，单行即可解决

### 2. Dockerfile 分层缓存

**决策：** 采用标准 Go 多阶段构建的依赖分层策略：
- 第 1 层：仅复制 `go.mod` + `go.sum`，执行 `go mod download`
- 第 2 层：复制其余源码，执行 `go build`
- 新增 `.dockerignore` 排除 `.git`、`node_modules`、`.gomodcache`、`api.exe`、`web/`、`services/`

**依据：** Docker 构建缓存基于层（layer）的哈希。将变化频率最低的 `go.mod`/`go.sum` 放在独立层，可确保只要不修改依赖，`go mod download` 步骤从缓存命中（约 2 秒 vs 3 分钟）。

**.dockerignore 的额外好处：** `deploy/` 的 Docker 上下文默认是整个项目根目录，没有 `.dockerignore` 时会打包 `node_modules`（数百 MB）和 `.git` 历史。排除这些文件后构建上下文从 ~500MB 降至 ~50MB。

### 3. api.exe 从 Git 移除

**决策：**
- 在 `.gitignore` 中添加 `api.exe` 和 `api`（Linux 二进制）
- 执行 `git rm --cached api.exe` 从 Git 跟踪中移除已提交的文件

**依据：** `git rm --cached` 保留本地文件但停止跟踪。这是标准做法，不影响本地开发。`api.exe` 仅约 20MB，移除后可减小仓库体积，避免未来合入时的二进制冲突。

## Risks / Trade-offs

| 风险 | 缓解措施 |
|------|----------|
| `.gitignore` 变更后其他开发者可能未同步 `.gitignore` 导致重新提交 `api.exe` | 仅影响未来提交，本地已有 `.gitignore` 的开发者通过 `git pull` 获取更新 |
| `git rm --cached api.exe` 后若其他分支仍包含 `api.exe`，切换分支会重新出现 | Cherry-pick 修复到各活跃分支，或在合并时注意 |
| Dockerfile 分层缓存若 `go.mod` 频繁变动则收益有限 | 依赖变更频率低（周级别），收益远大于风险 |
| 修复代码后需验证 `go build ./...` 通过及所有测试 | 修复后执行 `go build ./...` 和 `go test ./...` 确认 |

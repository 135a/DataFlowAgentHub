## Why

项目有 6 个已识别的结构性问题影响开发效率、部署可行性和代码可维护性：Go 版本锁定导致无法编译、handler 文件过大、无自动化流水线、无生产部署方案、AI 供应商锁定，以及 Python 依赖不可复现。

## What Changes

1. **Fix Go 版本锁死** — 将 `go.mod` 从 `go 1.25.0` 降级到实际可用的版本
2. **拆分 handler.go** — 将 984 行的大文件按功能拆分为独立文件
3. **添加 CI/CD** — GitHub Actions 自动 build + vet + test
4. **添加 K8s 部署清单** — Helm charts 或 Kustomize 配置
5. **解耦 LLM 供应商** — 抽象 LLMProvider 接口，修复 prompt 中残留的 PostgreSQL 引用
6. **锁定 Python 依赖** — 生成 `requirements.lock`

## Capabilities

### New Capabilities
- `llm-provider-abstraction`: 定义 LLMProvider 接口，将 OpenAI 实现抽为插件，支持多供应商路由
- `kubernetes-deployment`: Kubernetes 部署清单（Deployment + Service + ConfigMap），支持生产环境水平扩展
- `ci-cd-pipeline`: GitHub Actions 自动化流水线（lint → build → test → docker）

### Modified Capabilities
- 无（本次不涉及现有能力层的需求变更，均为实现层面的改进）

## Impact

| 影响范围 | 说明 |
|---------|------|
| Go 代码 | handler.go 拆分为 ~5 个文件；新增 `internal/llm/provider.go` 接口定义 |
| Python 代码 | 新增 `llm_provider.py` 抽象层，现有 OpenAI 调用迁移至其下 |
| 基础设施 | 新增 `.github/workflows/`、`deploy/k8s/` 目录 |
| 构建流程 | `go.mod` Go 版本变更；Python 新增 lockfile |
| 依赖 | 无新增外部依赖（K8s manifests 仅文本文件，非 Go 依赖） |

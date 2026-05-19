## 1. Go 版本降级

- [x] 1.1 将 `go.mod` 中的 `go 1.25.0` 改为 `go 1.23`
- [x] 1.2 验证 `go build ./...` 和 `go vet ./...` 通过

## 2. handler.go 拆分

- [x] 2.1 从 `handlers.go` 提取 PostMessage 相关逻辑到 `handlers/messages.go`
- [x] 2.2 从 `handlers.go` 提取管理端点到 `handlers/admin.go`（admin 端点已在 auth.go 中，仅清理 handlers.go）
- [x] 2.3 验证拆分后 `go build ./...` 通过，无循环引用

## 3. CI/CD 流水线

- [x] 3.1 创建 `.github/workflows/ci.yml`，配置 Go 编译、检查、测试
- [x] 3.2 在 CI 中添加 Docker 镜像构建验证步骤
- [x] 3.3 在 CI 中使用 `requirements.lock` 安装 Python 依赖

## 4. K8s 部署清单

- [x] 4.1 创建 `deploy/k8s/api-deployment.yaml`（Go API Deployment + Service + 探针）
- [x] 4.2 创建 `deploy/k8s/ai-worker-deployment.yaml`（Python Worker Deployment + Service + gRPC 探针）
- [x] 4.3 创建 `deploy/k8s/configmap.yaml`（非敏感配置项）
- [x] 4.4 创建 `deploy/k8s/README.md`（使用说明 + 生产就绪审查标注）

## 5. LLM 供应商解耦

- [x] 5.1 创建 `services/ai/llm_provider.py`，定义 `LLMProvider` 抽象基类 + `OpenAIProvider` 实现 + `FallbackProvider` 实现 + 工厂函数
- [x] 5.2 修改 `hub_ai/_server.py`，将现有 `_openai_sql` 和 `_rag_answer` 逻辑迁移至 `OpenAIProvider`
- [x] 5.3 修复 SQL 生成 prompt 中的 PostgreSQL 引用为 MySQL

## 6. Python 依赖锁定

- [x] 6.1 使用 `pip-compile` 从 `services/ai/requirements.txt` 生成 `services/ai/requirements.lock`
- [x] 6.2 验证 lockfile 已纳入版本管理

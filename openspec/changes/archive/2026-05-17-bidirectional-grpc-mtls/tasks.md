## 1. 证书生成与配置

- [x] 1.1 编写 `scripts/gen-certs.sh` 生成 CA、Go 服务端/客户端、Python 服务端/客户端证书（有效期 3650 天）
- [x] 1.2 创建 `certs/` 目录并将 `.gitignore` 加入其中
- [x] 1.3 更新 `deploy/compose/docker-compose.yml`：新增 :9090 端口、证书 volume 挂载、`HUB_GO_GRPC_TARGET` 环境变量
- [x] 1.4 验证证书生成脚本可重复执行且 `.gitignore` 生效

## 2. Proto 扩展与代码生成

- [x] 2.1 在 `api/proto/nl2sql/v1/nl2sql.proto` 中新增 `HubInternalService`（4 个 RPC：`TaskCallback`、`RunStepCallback`、`InternalNL2SQL`、`KnowledgeDocCallback`）
- [x] 2.2 执行 `make gen-go` 生成 Go 桩代码
- [x] 2.3 执行 `make gen-py` 生成 Python 桩代码

## 3. Go 侧：gRPC 服务端

- [x] 3.1 创建 `internal/grpcserver/server.go`：实现 `HubInternalServiceServer` 的 4 个 RPC 方法
- [x] 3.2 在 `cmd/api/main.go` 中新增 gRPC 服务端启动逻辑（:9090，mTLS，优雅关闭）
- [x] 3.3 从 HTTP handler 中抽离 TaskCallback 核心逻辑为公共方法（已直接删除，由 gRPC server 替代）
- [x] 3.4 从 HTTP handler 中抽离 RunStepCallback 核心逻辑为公共方法（已直接删除，由 gRPC server 替代）
- [x] 3.5 从 HTTP handler 中抽离 InternalNL2SQL 核心逻辑为公共方法（已直接删除，由 gRPC server 替代）
- [x] 3.6 从 HTTP handler 中抽离 KnowledgeDocCallback 核心逻辑为公共方法（已直接删除，由 gRPC server 替代）

## 4. Go 侧：mTLS 升级现有 gRPC 客户端

- [x] 4.1 修改 `internal/worker/nl2sql.go`：从 `insecure` 切换为 mTLS，加载客户端证书和服务端 CA

## 5. Python 侧：gRPC 客户端 + mTLS 升级

- [x] 5.1 创建 `services/ai/hub_ai/internal_client.py`：`HubInternalClient` 封装 4 个 gRPC 调用
- [x] 5.2 修改 `services/ai/hub_ai/__main__.py`：Python gRPC 服务端升级为 mTLS
- [x] 5.3 修改 `services/ai/orchestrator/consumer.py`：HTTP 回调替换为 gRPC 调用
- [x] 5.4 修改 `services/ai/orchestrator/knowledge_consumer.py`：HTTP 回调替换为 gRPC 调用
- [x] 5.5 修改 `services/ai/orchestrator/graph.py`：HTTP 调用替换为 gRPC 调用
- [x] 5.6 修改 `services/ai/orchestrator/tracing.py`：HTTP 调用替换为 gRPC 调用

## 6. 删除 HMAC 相关代码

- [x] 6.1 删除 `internal/middleware/middleware.go` 中的 `InternalHMACAuth` 函数
- [x] 6.2 删除 `internal/crypto/hmac.go` 整个文件
- [x] 6.3 删除 `services/ai/hub_ai/shared.py` 中的 `sign_body` 和 `make_headers` 函数
- [x] 6.4 删除 `internal/handlers/handlers.go` 中 `/internal` 路由组的 `InternalHMACAuth` 中间件挂载
- [x] 6.5 删除 `internal/handlers/handlers.go` 和 `internal/handlers/tasks.go` 中不再需要的 `hubcrypto` 导入
- [x] 6.6 从 `.env.example` 中移除 `HUB_INTERNAL_HMAC_SECRET` 配置项

## 7. 集成测试与验证

- [ ] 7.1 启动全栈 Docker Compose，确认 Go 和 Python 启动正常（需要 Docker Desktop 运行）
- [ ] 7.2 验证 Go→Python 的 `GenerateSQL` gRPC 调用在 mTLS 下正常工作（需要 Docker）
- [ ] 7.3 触发一次异步 agent pipeline，验证 Python→Go 的 gRPC 回调完整流程（需要 Docker）
- [ ] 7.4 触发知识文档索引，验证 KnowledgeDocCallback 回调（需要 Docker）
- [ ] 7.5 验证无效证书连接被拒绝（需要 Docker）
- [ ] 7.6 确认前端 SSE 推送正常，运行状态和步骤信息实时更新（需要 Docker）

## Why

当前 Go API 和 Python Worker 之间的服务间通信存在协议和认证方式不一致的问题：Go→Python 使用 gRPC (insecure)，而 Python→Go 使用 HTTP + HMAC 签名。这种双协议、双认证的架构增加了维护成本和认知负担，也不利于后续服务拆分。

本次变更加入反向 gRPC，并将现有 insecure 升级为 mTLS，统一为全双工 gRPC + mTLS 通信。

## What Changes

- **新增** Python→Go 方向的反向 gRPC，替代现有的 4 个 HTTP+HMAC 回调端点
- **升级** 现有 Go→Python gRPC 从 insecure 到 mTLS
- **新增** Go 侧 gRPC 服务端（:9090）
- **删除** `InternalHMACAuth` 中间件及所有 HMAC 相关代码
- **删除** Python 侧 `make_headers`、`sign_body` 等 HMAC 工具函数
- **新增** 自签证书体系（CA + 服务端 + 客户端证书）
- **新增** 证书生成脚本和 docker-compose 挂载配置

## Capabilities

### New Capabilities
- `service-mesh-communication`: 双工 gRPC + mTLS 服务间通信能力，覆盖 Go↔Python 两个方向的全部内部调用

### Modified Capabilities
<!-- 无现有 spec 需要修改 -->

## Impact

- `api/proto/nl2sql/v1/nl2sql.proto` — 新增 `HubInternalService` 定义
- `internal/gen/` — 重新生成 Go 桩代码
- `services/ai/gen/` — 重新生成 Python 桩代码
- `internal/handlers/tasks.go` — TaskCallback 核心逻辑抽离，HTTP handler 改为调用公共方法
- `internal/handlers/handlers.go` — InternalNL2SQL 核心逻辑抽离
- `internal/handlers/knowledge.go` — KnowledgeDocCallback 核心逻辑抽离
- `internal/middleware/middleware.go` — 删除 `InternalHMACAuth`
- `internal/crypto/hmac.go` — 删除整个文件
- `services/ai/hub_ai/shared.py` — 删除 HMAC 函数
- `services/ai/orchestrator/consumer.py` — HTTP→gRPC
- `services/ai/orchestrator/knowledge_consumer.py` — HTTP→gRPC
- `services/ai/orchestrator/tracing.py` — HTTP→gRPC
- `services/ai/orchestrator/graph.py` — HTTP→gRPC
- `internal/worker/nl2sql.go` — 升级到 mTLS
- `services/ai/hub_ai/__main__.py` — gRPC 服务端升级到 mTLS
- `cmd/api/main.go` — 新增 gRPC 服务端启动逻辑
- `deploy/compose/docker-compose.yml` — 新增 :9090 端口、证书 volume
- `Makefile` — 证书生成目标（可选）

## ADDED Requirements

### Requirement: 双工 gRPC 通信

Go API 和 Python Worker 之间 SHALL 通过双向 gRPC 进行全部服务间通信，并使用 mTLS 进行双向证书验证。

#### Scenario: Go 通过 gRPC 调用 Python GenerateSQL

- **WHEN** Go API 收到用户的 NL2SQL 请求
- **THEN** Go 通过 gRPC (mTLS) 调用 Python Worker 的 `GenerateSQL` RPC
- **THEN** Python Worker 返回生成的 SQL 和元数据

#### Scenario: Go 通过 gRPC 调用 Python RunAgentPipeline

- **WHEN** Go API 收到用户的 agent pipeline 请求
- **THEN** Go 通过 gRPC (mTLS) 调用 Python Worker 的 `RunAgentPipeline` RPC
- **THEN** Python Worker 返回 pipeline 执行结果

#### Scenario: Python 通过 gRPC 回调 Go TaskCallback

- **WHEN** Python Worker 完成异步任务处理
- **THEN** Python 通过 gRPC (mTLS) 调用 Go 的 `TaskCallback` RPC
- **THEN** Go 更新 `async_tasks` 表状态并推送 SSE 给前端

#### Scenario: Python 通过 gRPC 回调 Go RunStepCallback

- **WHEN** Python Worker 的 LangGraph 节点执行完毕
- **THEN** Python 通过 gRPC (mTLS) 调用 Go 的 `RunStepCallback` RPC
- **THEN** Go 插入 `agent_run_steps` 记录并推送 SSE 给前端

#### Scenario: Python 通过 gRPC 调用 Go InternalNL2SQL

- **WHEN** Python Worker 的 LangGraph nl2sql_node 需要执行 SQL
- **THEN** Python 通过 gRPC (mTLS) 调用 Go 的 `InternalNL2SQL` RPC
- **THEN** Go 调用 Python Worker 的 `GenerateSQL` gRPC 并执行返回的 SQL
- **THEN** Go 返回 SQL 执行结果给 Python

#### Scenario: Python 通过 gRPC 回调 Go KnowledgeDocCallback

- **WHEN** Python Worker 完成知识文档索引
- **THEN** Python 通过 gRPC (mTLS) 调用 Go 的 `KnowledgeDocCallback` RPC
- **THEN** Go 更新 `knowledge_docs` 表状态

### Requirement: 双向 mTLS 认证

Python Worker 和 Go API 之间的全部 gRPC 连接 SHALL 使用 mTLS 进行双向证书验证。

#### Scenario: Go 连接 Python Worker 时验证服务端证书

- **WHEN** Go API 向 Python Worker 发起 gRPC 连接
- **THEN** Go 验证 Python Worker 的服务端证书由内部 CA 签发
- **THEN** Python Worker 验证 Go 的客户端证书由同一内部 CA 签发

#### Scenario: Python 连接 Go 时验证服务端证书

- **WHEN** Python Worker 向 Go API 发起 gRPC 连接
- **THEN** Python 验证 Go 的服务端证书由内部 CA 签发
- **THEN** Go 验证 Python 的客户端证书由同一内部 CA 签发

#### Scenario: 无效证书拒绝连接

- **WHEN** 客户端使用未经 CA 签名的证书发起 gRPC 连接
- **THEN** 服务端拒绝连接，返回证书验证失败错误

### Requirement: HMAC 认证删除

服务间通信中 SHALL NOT 再使用 HMAC-SHA256 签名认证，全部 HMAC 相关代码 SHALL 被删除。

#### Scenario: HMAC 中间件不再存在

- **WHEN** 检查 `internal/middleware/` 代码
- **THEN** `InternalHMACAuth` 中间件 SHALL 已被删除

#### Scenario: HMAC 工具函数不再存在

- **WHEN** 检查 `internal/crypto/hmac.go`
- **THEN** 整个文件 SHALL 已被删除

#### Scenario: Python HMAC 函数不再存在

- **WHEN** 检查 `services/ai/hub_ai/shared.py`
- **THEN** `sign_body` 和 `make_headers` 函数 SHALL 已被删除

#### Scenario: Python 消费者不再使用 HTTP 回调

- **WHEN** 检查 `consumer.py`、`knowledge_consumer.py`、`tracing.py`、`graph.py`
- **THEN** 其中的 `httpx` HTTP 调用 SHALL 已被替换为 gRPC 调用
- **THEN** `make_headers` 导入 SHALL 已被删除

## ADDED Requirements

### Requirement: Knowledge consumer 随 ai-worker 自动启动

`services/ai/hub_ai/__main__.py` SHALL 在启动 gRPC server 的同时，以 daemon 线程方式启动 `orchestrator.knowledge_consumer.run_knowledge_consumer()`。

#### Scenario: ai-worker 正常启动时 knowledge consumer 自动运行

- **WHEN** ai-worker 容器启动，`python -m hub_ai` 被执行
- **THEN** gRPC server 在 `0.0.0.0:50051` 监听
- **AND** agent pipeline consumer 线程已启动，监听 NATS `hub.tasks.agent_pipeline`
- **AND** knowledge consumer 线程已启动，监听 NATS `hub.tasks.knowledge_index`

#### Scenario: 用户上传知识文档后被正确消费

- **WHEN** 用户通过 `POST /v1/workspaces/{id}/knowledge/docs` 上传文档
- **AND** Go handler 将任务写入 DB 并发布到 NATS `hub.tasks.knowledge_index`
- **THEN** knowledge consumer 接收到消息，调用 `KnowledgeBase.add_document()` 进行分块和嵌入
- **AND** 完成后回调 `PATCH /internal/knowledge-docs/{id}/status` 更新状态为 `completed`
- **AND** 用户通过 `GET /v1/workspaces/{id}/knowledge/docs` 看到文档状态为 `completed`

#### Scenario: Knowledge consumer 线程崩溃时记录错误日志

- **WHEN** knowledge consumer 线程因未捕获异常而崩溃
- **THEN** 错误信息被记录到 stderr（`logging.error`）
- **AND** agent consumer 线程不受影响，继续正常消费

### Requirement: Consumer 线程命名语义清晰

`__main__.py` 中的 consumer 启动函数 SHALL 使用明确区分 agent 和 knowledge 的命名。

#### Scenario: 代码审查时线程用途一目了然

- **WHEN** 开发者阅读 `__main__.py` 的 consumer 启动部分
- **THEN** `start_agent_consumer` 函数名明确指出启动的是 agent pipeline consumer
- **AND** `start_knowledge_consumer` 函数名明确指出启动的是 knowledge indexing consumer

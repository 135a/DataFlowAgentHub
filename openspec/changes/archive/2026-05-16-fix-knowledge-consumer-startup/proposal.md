## Why

当前 `services/ai/hub_ai/__main__.py` 启动时仅启动了 agent pipeline 的 NATS consumer（`orchestrator.consumer.run_consumer`），并未启动知识库索引的 NATS consumer（`orchestrator.knowledge_consumer.run_knowledge_consumer`）。这导致用户上传知识文档后，Go 侧已将任务写入 DB 并发布到 NATS `hub.tasks.knowledge_index`，但无人消费，文档状态永远停留在 `pending`，整个 RAG 索引流程断裂。

## What Changes

- 在 `__main__.py` 中新增 `start_knowledge_consumer` 线程，随 gRPC server 一同启动
- 将现有的 `start_consumer` 重命名为 `start_agent_consumer`，语义更明确
- Consumer 线程失败时记录日志，daemon 线程随主进程退出

## Capabilities

### New Capabilities

- `knowledge-consumer-startup`: 知识库索引 NATS consumer 随 ai-worker 服务自动启动，用户上传文档后能被正确消费、分块、嵌入并回调状态

### Modified Capabilities

（无现有 capability 需要修改）

## Impact

- 修改文件：`services/ai/hub_ai/__main__.py`（约 10 行改动）
- 新增文件：无
- 依赖新增：无
- 部署影响：需重新构建 `ai-worker` 镜像

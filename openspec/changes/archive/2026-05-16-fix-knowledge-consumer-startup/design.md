## Context

当前 `__main__.py` 的 NATS consumer 启动逻辑（第 194-206 行）仅启动了一个 consumer 线程：

```python
def start_consumer():
    from orchestrator.consumer import run_consumer  # agent pipeline
    asyncio.run(run_consumer())

consumer_thread = threading.Thread(target=start_consumer, daemon=True)
consumer_thread.start()
```

`orchestrator/knowledge_consumer.py` 已完整实现了 `run_knowledge_consumer()` 函数，监听 NATS `hub.tasks.knowledge_index` 主题，消费后调用 `KnowledgeBase.add_document()` 进行分块和嵌入，最后回调 Go API 更新文档状态。但此函数从未被调用或导入。

## Goals / Non-Goals

**Goals:**
- 知识库索引 consumer 随 ai-worker 服务自动启动，无需手动干预
- Consumer 线程崩溃时记录错误日志，便于排查
- 语义清晰：区分 agent consumer 和 knowledge consumer

**Non-Goals:**
- 不改动 `knowledge_consumer.py` 的消费逻辑（它已经完整可用）
- 不引入进程管理器或 supervisor（保持 daemon 线程模式，与现有架构一致）
- 不处理 consumer 崩溃后的自动重启（daemon 线程模式不做重启，依赖 Docker Compose 的 `restart: unless-stopped` 做进程级恢复）

## Decisions

### Decision 1: 使用独立的 daemon 线程而非在同一 asyncio event loop 中运行

**选择**：为 knowledge consumer 创建独立线程，与 agent consumer 线程模式完全一致。

**替代方案**：在同一个 asyncio event loop 中 `gather` 两个 consumer。

**理由**：
- 与现有 `start_consumer` 模式保持一致，降低认知差异
- 两个 consumer 的崩溃互相隔离：agent consumer 崩溃不影响 knowledge consumer，反之亦然
- 改动最小，仅新增约 8 行代码

### Decision 2: 重命名现有函数以提升语义

**选择**：将 `start_consumer` 重命名为 `start_agent_consumer`。

**理由**：不再只有一个 consumer，含糊的命名会造成理解障碍。

## Risks / Trade-offs

- [风险] 两个 daemon 线程共享同一进程，若 ai-worker 崩溃则所有 consumer 同时停止
  → 缓解：Docker Compose `restart: unless-stopped` 确保进程级恢复；NATS JetStream 持久化保证消息不丢失
- [风险] `threading.Thread` + `asyncio.run()` 混用模式在 Python 3.12 中被警告
  → 缓解：MVP 阶段保持与现有 agent consumer 一致，后续统一迁移到纯 asyncio 方案
- [权衡] daemon 线程不捕获 `asyncio.CancelledError`，优雅关闭时 consumer 可能被强制终止
  → 接受：当前 agent consumer 有同样行为，且 NATS 消息有 ack/nak 机制保证不丢

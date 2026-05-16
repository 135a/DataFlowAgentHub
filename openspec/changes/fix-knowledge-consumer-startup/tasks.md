## 1. 修改 ai-worker 启动逻辑

- [ ] 1.1 将 `__main__.py` 第 195 行的 `start_consumer` 函数重命名为 `start_agent_consumer`
- [ ] 1.2 新增 `start_knowledge_consumer` 函数，内部调用 `orchestrator.knowledge_consumer.run_knowledge_consumer()`
- [ ] 1.3 创建 `knowledge_thread` daemon 线程，target 指向 `start_knowledge_consumer`
- [ ] 1.4 更新日志信息：`"Started NATS consumer thread"` → `"Started NATS consumer threads (agent + knowledge)"`

## 2. 验证

- [ ] 2.1 启动全栈服务：`docker compose -f deploy/compose/docker-compose.yml up -d --build`
- [ ] 2.2 检查 ai-worker 日志，确认同时出现 agent consumer 和 knowledge consumer 的启动日志
- [ ] 2.3 通过 API 上传知识文档，验证文档状态在 5 秒内从 `pending` 变为 `completed`
- [ ] 2.4 验证 agent pipeline consumer 不受影响：发送含"分析"关键词的消息，确认异步任务正常完成

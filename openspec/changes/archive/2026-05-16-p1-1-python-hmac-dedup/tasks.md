## 1. 新建共享模块

- [x] 1.1 创建 `services/ai/hub_ai/shared.py`，实现 `sign_body(secret, body)` 和 `make_headers(secret, body_bytes)` 两个函数，仅依赖标准库 `hmac` 和 `hashlib`

## 2. 修改 orchestrator 模块 —— 替换导入

- [x] 2.1 修改 `services/ai/orchestrator/consumer.py`：移除本地 `sign_body()` 和 `make_headers()` 定义（含 `import hmac`/`import hashlib`），添加 `from hub_ai.shared import sign_body, make_headers`
- [x] 2.2 修改 `services/ai/orchestrator/knowledge_consumer.py`：同上
- [x] 2.3 修改 `services/ai/orchestrator/graph.py`：将 inline HMAC + headers 构造替换为 `make_headers()` 调用，移除 `import hmac`/`import hashlib`，添加 `from hub_ai.shared import make_headers`
- [x] 2.4 修改 `services/ai/orchestrator/tracing.py`：移除本地 `sign_body()` 定义和内联 headers 构造，替换为 `make_headers()` 调用，移除 `import hmac`/`import hashlib`，添加 `from hub_ai.shared import sign_body, make_headers`

## 3. 验证

- [x] 3.1 确认 4 个修改文件中不再包含 `import hmac` 和 `import hashlib`（除 shared.py 自身外）
- [ ] 3.2 重启 ai-worker 容器，确认 NATS 消费者正常启动且回调 Go API 无签名校验错误（需 Docker 环境手动验证）

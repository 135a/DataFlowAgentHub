# 冒烟清单

1. **健康**：`GET /health` 返回 `postgres` / `redis` 为 `ok`。
2. **登录**：`POST /v1/auth/login` 使用种子账号拿到 `access_token`。
3. **会话**：`POST /v1/sessions` 创建会话；`GET /v1/sessions` 列表可见。
4. **消息（流式）**：另开终端 `curl -N -H "Authorization: Bearer <token>" http://127.0.0.1:8080/v1/sessions/<sid>/stream`；再 `POST /v1/sessions/<sid>/messages` 发送自然语言（非 `export`），应看到 `sql_generated` / `result` 等事件。
5. **审批**：发送包含 **`export`** 的消息，应返回 `awaiting_approval`；`GET /v1/approvals` 可见 pending；`POST /v1/approvals/<id>/decide` 批准或驳回。

6. **Schema 发现**：创建一个关联了 hub 自身数据库的会话（或默认无关联的会话），发送自然语言查询（如 "how many rows in demo_sales"），确认返回结果正确。可检查 Python worker 日志，确认 prompt 中 `Tables:` 字段包含 `demo_sales` 表的列信息（`id (integer), region (text), amount_cents (integer), sold_on (date)`）而非空 `{}`。再用 `docker compose exec redis redis-cli KEYS "schema:*"` 验证 Redis 缓存 key 存在。

7. **知识索引**：使用 operator/admin 账号调用 `POST /v1/workspaces/{workspaceID}/knowledge/docs` 上传一篇 Markdown 文档（`{"title": "测试文档", "content": "# Hello\n\n这是一段测试内容。"}`），确认返回 202 及 `task_id`。等待 2-5 秒后调用 `GET /v1/workspaces/{workspaceID}/knowledge/docs` 查看文档列表，确认上传的文档 `status` 变为 `completed`。检查 Python `knowledge_consumer` 日志确认 NATS 消息已被消费。

8. **SSE 实时推送**：使用 `POST /v1/sessions/{sessionID}/sse-token` 获取短期 SSE Token。用浏览器或 `curl` 连接 `GET /v1/sessions/{sessionID}/stream?token=<sse_token>`，确认连接成功（text/event-stream）。另开终端发送消息 `POST /v1/sessions/{sessionID}/messages`，确认 SSE 连接收到 `result`、`agent_step` 等事件。断开连接后确认 3 秒内自动重连。

（可选）配置 `OPENAI_API_KEY` 后重复步骤 4，验证 LLM 生成 SQL。

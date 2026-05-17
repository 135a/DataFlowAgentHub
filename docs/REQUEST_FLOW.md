# 用户请求传递流程

> 用户发送一条消息后，它在各个文件之间的完整传递路径

---

## 一、总览

```
                                      用户消息
                                         │
                                         ▼
                                  ┌─────────────┐
                                  │  Go API      │
                                  │  :8080       │
                                  └──┬──────┬────┘
                         gRPC :50051 │      │ NATS
                                    │      │
                            ┌───────▼┐  ┌──▼──────────┐
                            │ Python  │  │ Python      │
                            │ gRPC    │  │ NATS        │
                            │ 服务端  │  │ 消费者      │
                            └────────┘  └─────────────┘
```

---

## 二、同步路径（简单查询）

用户发普通问题（不含 analyze/report 等关键词），走同步快速通道。

```
浏览器                          Go API                           Python AI                    DB
  │                              │                                │                         │
  │ HTTP POST                     │                                │                         │
  │ /v1/sessions/{sid}/messages   │                                │                         │
  │══════════════════════════════>│                                │                         │
  │                              │                                │                         │
  │                              ├── handlers/handlers.go ────────┤                         │
  │                              │   PostMessage() [第272行]      │                         │
  │                              │                                │                         │
  │                              │ ① ratelimit 限流检查           │                         │
  │                              │ ② 解析 JSON 请求体             │                         │
  │                              │ ③ 验证 Session 归属权          │                         │
  │                              │ ④ 查数据源配置                 │                         │
  │                              │ ⑤ 存用户消息到 messages 表 ────┼───────────>             │
  │                              │ ⑥ SSE: "user_message"         │                         │
  │                              │ ⑦ 检查 export 关键词           │                         │
  │                              │ ⑧ Schema 发现 + 缓存          │                         │
  │                              │ ⑨ 创建 run 记录 ──────────────┼───────────>             │
  │                              │ ⑩ SSE: "run_started"          │                         │
  │                              │                                │                         │
  │                              ├── nl2sqlexec/executor.go ──────┤                         │
  │                              │   Execute() [第56行]           │                         │
  │                              │                                │                         │
  │                              ├── worker/nl2sql.go ────────────┤                         │
  │                              │   GenerateSQL() [第37行]       │                         │
  │                              │   ═══════════════════════════> │                         │
  │                              │   gRPC GenerateSQL             │                         │
  │                              │                                ├── hub_ai/__main__.py ───┤
  │                              │                                │   Servicer.GenerateSQL()│
  │                              │                                │   [第57行]               │
  │                              │                                │                         │
  │                              │                                │   有 API Key?           │
  │                              │                                │     ├── 是 → OpenAI API │
  │                              │                                │     └── 否 → 正则兜底   │
  │                              │                                │                         │
  │                              │  ◀══════════════════════════════ 返回 SQL               │
  │                              │                                │                         │
  │                              ├── sqlrun/run.go ──────────────┤                         │
  │                              │   QueryRows() [第30行]        │                         │
  │                              │   ① 只读关键字二次检查         │                         │
  │                              │   ② 包装 SQL + LIMIT          │                         │
  │                              │   ③ 执行查询 ─────────────────┼───────────>             │
  │                              │   ④ 扫描结果 → []map[string]any                         │
  │                              │                                │                         │
  │                              ├── 存 assistant 消息到 DB ─────┼───────────>             │
  │                              │   SSE: "sql_generated"        │                         │
  │                              │   SSE: "result"               │                         │
  │                              │                                │                         │
  │ HTTP 200 {run_id, sql, rows} │                                │                         │
  │◀══════════════════════════════                                │                         │
```

### 涉及文件一览（同步路径）

| 步骤 | 文件 | 函数 | 行号 | 做什么 |
|------|------|------|------|--------|
| 入口 | `internal/handlers/handlers.go` | `PostMessage()` | 272 | 接收 HTTP 请求，限流/验证/路由 |
| 执行 | `internal/nl2sqlexec/executor.go` | `Execute()` | 56 | 编排 gRPC 调用 + SQL 执行 |
| gRPC | `internal/worker/nl2sql.go` | `GenerateSQL()` | 37 | 向 Python 发 gRPC 请求 |
| Python | `services/ai/hub_ai/__main__.py` | `Servicer.GenerateSQL()` | 57 | 调用 LLM 生成 SQL |
| SQL | `internal/sqlrun/run.go` | `QueryRows()` | 30 | 执行 SQL（只读保护） |

---

## 三、异步路径（深度分析）

用户问题含 analyze/report/图表 等关键词，或显式指定 `workflow: "agent_pipeline"`。

```
浏览器        Go API                         Python NATS 消费者              LangGraph 图          Go 内部接口
  │            │                                │                            │                    │
  │ HTTP POST  │                                │                            │                    │
  │═══════════>│                                │                            │                    │
  │            │  handlers/handlers.go          │                            │                    │
  │            │  PostMessage 判断走异步路径     │                            │                    │
  │            │                                │                            │                    │
  │            ├── async/task.go ───────────────┤                            │                    │
  │            │   EnqueueTask() [第35行]       │                            │                    │
  │            │   ① INSERT async_tasks 表      │                            │                    │
  │            │   ② NATS Publish              │                            │                    │
  │            │      hub.tasks.agent_pipeline  │                            │                    │
  │            │                                │                            │                    │
  │ ◄══════════ 202 {run_id, task_id,           │                            │                    │
  │               status:"pending_async"}       │                            │                    │
  │            │                                │                            │                    │
  │            │                           ┌────┴────┐                      │                    │
  │            │                           │ NATS     │                      │                    │
  │            │                           │ 消息     │                      │                    │
  │            │                           └────┬────┘                      │                    │
  │            │                                │                            │                    │
  │            │                     ┌──────────▼──────────┐                 │                    │
  │            │                     │ consumer.py          │                 │                    │
  │            │                     │ process_message()    │                 │                    │
  │            │                     │ [第15行]             │                 │                    │
  │            │                     │                      │                 │                    │
  │            │                     │ asyncio.to_thread(   │                 │                    │
  │            │                     │   graph.invoke)      │                 │                    │
  │            │                     └──────────┬───────────┘                 │                    │
  │            │                                │                            │                    │
  │            │                     ┌──────────▼───────────┐                │                    │
  │            │                     │ graph.py LangGraph   │                │                    │
  │            │                     │                      │                │                    │
  │            │                     │  NL2SQL_NODE         │                │                    │
  │            │                     │  ────────────────    │                │                    │
  │            │                     │  HTTP POST           │                │                    │
  │            │                     │  /internal/nl2sql    │══════════════> │                    │
  │            │                     │  (HMAC签名)          │                │                    │
  │            │                     │                      │                │  handlers/handlers. │
  │            │                     │                      │                │  go InternalNL2SQL  │
  │            │                     │                      │                │  [第586行]          │
  │            │                     │                      │                │   → executor.Execute│
  │            │                     │                      │                │   → gRPC GenerateSQL│
  │            │                     │                      │                │   → sqlrun.QueryRows│
  │            │                     │                      │                ◄════ 返回 {sql,rows}│
  │            │                     │                      │                │                    │
  │            │                     │  ANALYSIS_NODE       │                │                    │
  │            │                     │  ────────────────    │                │                    │
  │            │                     │  data_analysis_      │                │                    │
  │            │                     │  agent.py            │                │                    │
  │            │                     │  pandas 统计分析     │                │                    │
  │            │                     │  + LLM 业务摘要      │                │                    │
  │            │                     │                      │                │                    │
  │            │                     │  CHART_NODE          │                │                    │
  │            │                     │  ────────────────    │                │                    │
  │            │                     │  chart_agent.py      │                │                    │
  │            │                     │  matplotlib 图表     │                │                    │
  │            │                     │                      │                │                    │
  │            │                     │  REPORT_NODE         │                │                    │
  │            │                     │  ────────────────    │                │                    │
  │            │                     │  report_generation_  │                │                    │
  │            │                     │  agent.py            │                │                    │
  │            │                     │  Markdown + Excel    │                │                    │
  │            │                     │                      │                │                    │
  │            │                     │  每个节点执行时:      │                │                    │
  │            │                     │  tracing.py          │══════════════> │                    │
  │            │                     │  POST /internal/     │                │  tasks.go           │
  │            │                     │  runs/{id}/steps     │                │  RunStepCallback    │
  │            │                     │  (HMAC签名)          │                │  [第117行]          │
  │            │  ◄ SSE "agent_step"  │                    │                │                    │
  │            │                     │                      │                │                    │
  │            │                     └──────────┬───────────┘                │                    │
  │            │                                │                            │                    │
  │            │                     ┌──────────▼───────────┐                │                    │
  │            │                     │ consumer.py          │                │                    │
  │            │                     │ HTTP POST            │                │                    │
  │            │                     │ /internal/tasks/     │══════════════> │                    │
  │            │                     │ {id}/callback        │                │                    │
  │            │                     │ (HMAC签名)           │                │  tasks.go           │
  │            │                     │                      │                │  TaskCallback()     │
  │            │                     │                      │                │  [第54行]           │
  │            │                     │                      │                │  ① 更新 async_tasks │
  │            │                     │                      │                │  ② 更新 runs        │
  │            │                     │                      │                │  ③ 存 assistant消息 │
  │            │  ◄ SSE "result"     │                      │                │  ④ SSE: "result"    │
```

### 涉及文件一览（异步路径）

| 步骤 | 文件 | 函数 | 行号 | 做什么 |
|------|------|------|------|--------|
| 入口 | `internal/handlers/handlers.go` | `PostMessage()` | 272 | 判断走异步，调 EnqueueTask |
| 任务创建 | `internal/async/task.go` | `EnqueueTask()` | 35 | DB 写入 + NATS 发布 |
| 消费者 | `services/ai/orchestrator/consumer.py` | `process_message()` | 15 | NATS 消费，启动 LangGraph |
| 图编排 | `services/ai/orchestrator/graph.py` | `build_graph()` | 113 | 4 节点 LangGraph 工作流 |
| NL2SQL | `services/ai/orchestrator/graph.py` | `nl2sql_node()` | 22 | 回调 Go 生成+执行 SQL |
| 分析 | `services/ai/agents/data_analysis_agent.py` | `data_analysis_node()` | 17 | pandas 统计 + LLM 摘要 |
| 图表 | `services/ai/agents/chart_agent.py` | `chart_agent_node()` | 195 | matplotlib 生成 PNG |
| 报告 | `services/ai/agents/report_generation_agent.py` | `report_generation_node()` | 11 | Markdown + Excel |
| 追踪 | `services/ai/orchestrator/tracing.py` | `report_run_step()` | 10 | 步骤进度回调 Go |
| 回调 | `internal/handlers/tasks.go` | `TaskCallback()` | 54 | 更新状态 + SSE 推送 |

---

## 四、SSE 事件推送

所有 SSE 事件通过 `handlers/handlers.go` 的 `SessionStream()` 函数（第 444 行）发送，前端通过 `GET /v1/sessions/{sessionID}/stream` 接收。

| 事件类型 | 触发时机 | 发送者 |
|---------|---------|--------|
| `user_message` | 用户消息已保存 | `PostMessage()` line 311 |
| `run_started` | Run 记录已创建 | `PostMessage()` line 364 |
| `sql_generated` | SQL 已生成（仅同步） | `PostMessage()` line 416 |
| `agent_step` | Agent 节点执行进度（仅异步） | `tasks.go` `RunStepCallback()` line 155 |
| `result` | 最终结果就绪 | `PostMessage()` line 421 或 `TaskCallback()` line 108 |
| `error` | 执行失败 | `finishRunFailed()` line 440 |
| `approval_*` | 审批流程状态变更 | `DecideApproval()` line 563 |

---

## 五、关键文件索引

### Go 端

| 文件 | 核心职责 |
|------|---------|
| `cmd/api/main.go` | 启动入口，组装所有依赖 |
| `internal/handlers/handlers.go` | **核心**：21 个路由，PostMessage、SessionStream、InternalNL2SQL |
| `internal/handlers/tasks.go` | 异步任务回调、步骤追踪、任务状态查询 |
| `internal/handlers/datasources.go` | 数据源管理（CRUD + 测试连接） |
| `internal/middleware/middleware.go` | 认证、RBAC、TraceID、HMAC 中间件 |
| `internal/worker/nl2sql.go` | gRPC 客户端，调 Python GenerateSQL |
| `internal/nl2sqlexec/executor.go` | NL2SQL 执行编排器 |
| `internal/sqlrun/run.go` | 只读 SQL 执行器 |
| `internal/async/task.go` | 异步任务队列（DB + NATS） |
| `internal/schema/discovery.go` | 数据库 Schema 发现 |
| `internal/ssebus/bus.go` | 内存 SSE 发布/订阅总线 |

### Python 端

| 文件 | 核心职责 |
|------|---------|
| `services/ai/hub_ai/__main__.py` | gRPC 服务端 + 后台线程启动 |
| `services/ai/orchestrator/graph.py` | LangGraph 4 节点工作流 |
| `services/ai/orchestrator/consumer.py` | NATS 消费者 |
| `services/ai/orchestrator/tracing.py` | 步骤进度回调 |
| `services/ai/agents/data_analysis_agent.py` | 数据分析 Agent |
| `services/ai/agents/chart_agent.py` | 图表生成 Agent |
| `services/ai/agents/report_generation_agent.py` | 报告生成 Agent |
| `services/ai/rag/knowledge_base.py` | RAG 知识库（ChromaDB） |

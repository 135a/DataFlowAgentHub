## 1. 项目依赖与环境准备

- [x] 1.1 在 `services/ai/` 下更新 `pyproject.toml` / `requirements.txt`，添加 `langgraph`、`langchain`、`chromadb`、`pandas`、`numpy`、`openpyxl`、`weasyprint`（PDF 导出）等依赖，并锁定版本
- [x] 1.2 在 `deploy/compose/docker-compose.yml` 中新增 `chroma`（向量数据库）与 `rocketmq`（或 `nats`）服务容器；更新 `.env.example` 补充新增环境变量（CHROMA_HOST、MQ_BROKER_ADDR 等）
- [x] 1.3 在 Go 端的 `go.mod` 中引入 RocketMQ（或 NATS）客户端库依赖

## 2. 数据库 Schema 扩展

- [x] 2.1 新增迁移脚本，创建 `async_tasks` 表（task_id、session_id、status、payload、result、created_at、updated_at）
- [x] 2.2 新增迁移脚本，创建 `knowledge_docs` 表（doc_id、workspace_id、title、content_hash、vector_id、created_at）
- [x] 2.3 新增迁移脚本，创建 `agent_run_steps` 表（step_id、run_id、agent_name、input_snapshot、output_snapshot、status、created_at），用于记录多 Agent 流转的中间链路追踪
- [x] 2.4 更新 `docs/MIGRATIONS.md` 补充新增三张表的说明与回滚策略

## 3. RAG 知识库模块（rag-knowledge-base）

- [x] 3.1 在 `services/ai/` 下新建 `rag/` 目录，实现 `KnowledgeBase` 类，封装向量化（Embedding）与存储（Chroma Collection）逻辑
- [x] 3.2 实现文档入库接口：支持接收纯文本/Markdown 内容，分块（chunking）后向量化并存储至 Chroma
- [x] 3.3 实现语义检索接口：根据用户问题进行相关度查询，返回 Top-K 候选知识片段
- [x] 3.4 在 Go 控制面新增知识库管理 API：`POST /api/v1/workspaces/{id}/knowledge/docs`（上传）、`GET /api/v1/workspaces/{id}/knowledge/docs`（列表），并校验 operator 及以上权限
- [x] 3.5 为知识库接口编写冒烟测试：上传一个业务指标定义文档，验证检索接口能返回相关片段

## 4. 数据分析 Agent（data-analysis-agent）

- [x] 4.1 在 `services/ai/` 下新建 `agents/data_analysis_agent.py`，实现 `DataAnalysisAgent` 类，暴露标准的 LangGraph 节点函数签名
- [x] 4.2 实现核心分析能力：接收 State 中的 DataFrame，计算同比/环比（MoM/YoY）、滚动均值、及数值异常（超出 N 倍标准差）识别
- [x] 4.3 实现结论文本生成：调用大模型将计算结果转换为人类可读的业务结论摘要，写回 State 的 `analysis_summary` 字段
- [x] 4.4 添加数据截断保护：当 DataFrame 行数超过阈值（默认 500 行）时，自动截断并在日志中记录告警，防止上下文超限

## 5. 报表生成 Agent（report-generation-agent）

- [x] 5.1 在 `services/ai/` 下新建 `agents/report_generation_agent.py`，实现 `ReportGenerationAgent` 类，作为 LangGraph 节点
- [x] 5.2 实现 Markdown 报告结构化生成：读取 State 中的 `nl2sql_result`、`analysis_summary`，输出结构化的 Markdown 格式简报
- [x] 5.3 实现 Excel 导出工具：使用 `openpyxl` 将 DataFrame 数据写入 `.xlsx` 文件，保存至对象存储或本地挂载目录
- [x] 5.4 实现 PDF 导出工具（可选 MVP）：将 Markdown 报告通过 `weasyprint` 转换为 PDF 文件
- [x] 5.5 在 Go 控制面新增报表文件下载 API：`GET /api/v1/runs/{run_id}/report`，根据 run 的状态返回文件流或 404

## 6. LangGraph 多 Agent 编排器（langgraph-orchestrator）

- [x] 6.1 在 `services/ai/` 下新建 `orchestrator/graph.py`，使用 LangGraph 的 `StateGraph` 定义全局共享状态字典 (`AgentState`)，包含字段：`user_input`、`schema_context`、`rag_context`、`sql`、`nl2sql_result`、`analysis_summary`、`final_report`、`error`
- [x] 6.2 将各 Agent 注册为图的节点（Node）：`rag_node`、`nl2sql_node`（复用现有 NL2SQL Worker）、`analysis_node`、`report_node`
- [x] 6.3 定义带条件判断的有向边（Edge）：根据 State 中的任务意图（如是否包含"分析"、"生成报告"关键词）动态决定激活哪些下游节点
- [x] 6.4 接入 LangGraph 的 `MemorySaver` / `SqliteSaver` Checkpointer，实现 AgentState 的中间状态持久化，支持任务恢复
- [x] 6.5 实现从 Go 底座接收任务 payload 的 gRPC 接口，将请求转化为 LangGraph 的初始 State 并触发 `graph.invoke()`
- [x] 6.6 确保每个节点执行完毕后，将节点名称、输入/输出摘要写入 `agent_run_steps` 表以供追踪

## 7. 异步任务调度器 Go 端（async-task-scheduler）

- [x] 7.1 在 `internal/` 下新建 `async/` 包，定义 `Task` 结构体与 `TaskStore` 接口（支持 Postgres 实现），封装任务创建、状态更新、查询等基础操作
- [x] 7.2 实现 MQ 生产者（Producer）：将任务 payload 序列化后投递至 RocketMQ/NATS Topic，并在 DB 中将任务状态更新为 `queued`
- [x] 7.3 实现任务提交接口：对需要异步执行的 Agent 请求立即返回 HTTP 202 + `task_id`，由后台 goroutine 触发投递
- [x] 7.4 实现 Python Worker 回调接口：`POST /internal/tasks/{task_id}/callback`，接收执行结果并更新任务终态（`succeeded` / `failed`）；接口需 HMAC 签名校验（复用已有内部鉴权机制）
- [x] 7.5 新增任务状态查询 API：`GET /api/v1/tasks/{task_id}`，返回任务当前状态与结果摘要（需 JWT/API Key 鉴权）
- [x] 7.6 实现 MQ 消费者（Consumer）：在 Python 端订阅对应 Topic，拉取任务后触发 LangGraph 编排器，执行完毕后调用回调接口
- [x] 7.7 实现任务 TTL 过期机制：通过定时任务（Go ticker / cron）扫描长时间停留在 `queued`/`running` 状态的任务，标记为 `expired`

## 8. 会话与中间步骤增强（conversation-session）

- [x] 8.1 扩展消息历史 API 响应结构，新增 `run_steps` 字段，返回 `agent_run_steps` 表中该会话关联 run 的所有中间步骤记录
- [x] 8.2 更新 SSE 流式推送协议，新增 `agent_step` 事件类型（`event: agent_step`），实时推送当前激活的 Agent 名称与摘要信息
- [x] 8.3 在 `docs/` 中更新 SSE 事件类型说明文档

## 9. Go 底座编排增强（agent-orchestration）

- [x] 9.1 扩展 `internal/orchestration` 模块，增加对异步 run 的状态支持：`pending_async` → `running` → `succeeded` / `failed`
- [x] 9.2 调整任务分发逻辑：当任务类型被识别为复杂 Agent 流水线时，走异步 MQ 通道；简单单次 NL2SQL 查询保留同步 gRPC 通道
- [x] 9.3 在 `deploy/compose/docker-compose.yml` 中更新 `ai-worker` 服务，使其同时启动 LangGraph HTTP 服务与 MQ Consumer

## 10. 前端更新（web）

- [x] 10.1 在前端对话页中渲染 `agent_step` SSE 事件：以时间线或折叠卡片形式展示"RAG 检索中 → SQL 生成中 → 数据分析中 → 报告生成中"的实时进度
- [x] 10.2 新增任务状态轮询逻辑：对于 HTTP 202 异步任务响应，前端每 5 秒轮询 `/api/v1/tasks/{task_id}` 直到终态，并在界面上展示最终结果
- [x] 10.3 新增报表下载按钮：当 run 状态为 `succeeded` 且 report 文件存在时，展示"下载 Excel"与"下载 PDF"操作按钮

## 11. 可观测性补充（observability）

- [x] 11.1 在 Python `ai-worker` 中集成 `opentelemetry`，追踪 LangGraph 每次 Node 调用的耗时与输入输出长度
- [x] 11.2 将 `agent_pipeline` 日志格式对齐现有的 JSON Log 规范，确保 trace_id 一致性跨 Go → MQ → Python Worker 链路完整传播
- [x] 11.3 在 Prometheus metrics 中新增指标：`agent_node_duration_seconds`（各 Agent 节点耗时）、`async_task_queue_length`（MQ 积压长度）
- [x] 11.4 补充冒烟测试脚本（`docs/SMOKE_CHECKLIST.md`）：覆盖"提交复杂异步任务 → 轮询状态 → 下载报表"完整链路

## 12. 文档与演示数据

- [x] 12.1 准备测试数据集（包含用于 RAG 的《业务口径白皮书.md》和用于 NL2SQL 的 `sales_data` 测试表结构）
- [x] 12.2 在 `docs/MultiAgent.md` 中补充 LangGraph 架构图及交互时序图知识库文档批量导入的操作步骤与新增环境变量说明

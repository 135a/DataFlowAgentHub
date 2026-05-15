## Why

随着项目第一阶段（Go底座与基础 Python NL2SQL Worker）的完成，系统已经具备了单任务的数据查询和初步的大模型交互能力。然而，距离完整的“一体化数据智能体平台”仍有差距。在真实的复杂企业数据场景中，我们迫切需要：
1. 应对包含多个步骤的复杂任务（例如：先查询数据，再进行同比分析，最后生成报告）。
2. 让不同专长的 Agent 相互协作（如 NL2SQL Agent、数据分析 Agent 与报表 Agent）。
3. 解决领域数据知识的精准问答问题（如业务指标口径的 RAG 知识库问答）。
4. 对长耗时的任务（如大规模报表生成）进行健壮的异步调度与状态跟踪。

本变更旨在引入多 Agent 编排框架（LangGraph）、向量检索（RAG）、高级分析 Agent 及异步任务调度机制，从而全面实现架构提纲中的“3.3 多 Agent 协作功能”与“3.4 核心 AI 数据功能”。

## What Changes

- **NEW** 引入 LangGraph 作为核心多 Agent 协作编排引擎，支持复杂任务的合理拆解与串/并行流转。
- **NEW** 构建独立的数据分析 Agent，处理深度的数学分析（如同环比计算、趋势分析、业务异常识别）。
- **NEW** 构建自动报表生成 Agent，能够将多步分析的结果汇总为结构化报告（并支持 Excel/PDF 导出）。
- **NEW** 集成 RAG 数据问答模块（基于 Chroma 或 FAISS），支持基于指标口径与数据字典的智能问答。
- **NEW** 在 Go 底座侧引入异步任务调度（结合 Go-Workflow 与 RocketMQ），支持报表生成等长耗时异步任务的削峰排队与可靠投递。
- **NEW** 增强上下文缓存与记忆管理，实现 Agent 池化与多角色间的 Shared Context。

## Capabilities

### New Capabilities
- `langgraph-orchestrator`: 基于 LangGraph 的多 Agent 协作编排模块。负责接收复杂指令，将其拆解为多步子任务，协调各专业 Agent 流水线作业并管理共享上下文状态。
- `data-analysis-agent`: 智能数据分析模块。作为专用 Worker 节点，基于已有查询结果执行定制的统计计算与趋势分析。
- `report-generation-agent`: 自动报表生成模块。负责聚合分析结论与原始数据，生成企业级图文简报及可导出的 Excel/PDF 报表格式。
- `rag-knowledge-base`: 知识库与检索问答模块。提供向量库维护与检索能力，专门针对“指标口径是什么”、“表结构定义”等领域元数据进行精确答复。
- `async-task-scheduler`: Go 底座端的异步长任务调度器。负责高延迟任务（如离线报表导出）的状态机流转、排队控制及完成回调。

### Modified Capabilities
- `agent-orchestration`: 增强现有的编排核心，使其能够与 Python 侧的 LangGraph 编排器进行更复杂的双向协作流转，支持挂起与异步结果回调。
- `conversation-session`: 增强会话上下文存储结构，以完整记录和回放多 Agent 多轮流转的中间步骤（Trace）与思考过程。

## Impact

- **代码与依赖**：在 Python 端引入 `langgraph`、向量数据库驱动（如 `chromadb`）以及数据科学库（`pandas` 等）；Go 端引入 RocketMQ 客户端及任务调度中间件抽象。
- **API 扩展**：对外新增用于知识库文档上传/同步的 API、异步任务状态查询 API 以及报表文件下载 API。
- **数据结构变更**：需扩展数据库 Schema 以支持 `async_tasks`（异步任务队列）、`knowledge_docs`（知识库元数据）以及 `agent_traces`（复杂协作链路追踪）。
- **基础设施**：部署架构中需要新增 RocketMQ（或替代消息队列）与向量数据库，增加了一定的运维复杂度与硬件资源开销。

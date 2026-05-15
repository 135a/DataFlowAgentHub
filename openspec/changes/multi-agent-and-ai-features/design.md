## Context

目前，DataFlowAgentHub 已经通过 `bootstrap-enterprise-data-agent` 建立了具备高可用支撑的 Go 语言底层网关与基础的 Python 单节点 NL2SQL Worker，能够满足单轮简单数据查询的闭环需求。然而，面对真实企业场景，单一 Agent 往往无法胜任复杂的多步骤业务流（如跨表检索、趋势分析、生成总结报告）。为此，我们亟需将平台升级为基于多 Agent 协作（Multi-Agent Collaboration）的复杂智能处理平台，并辅以知识库增强（RAG）和健壮的长耗时异步任务调度机制。

## Goals / Non-Goals

**Goals:**
- 将单体 AI 逻辑重构为基于 LangGraph 的多 Agent 协作管线，实现任务的分发与串行/并行调度。
- 引入三个专门的核心 Agent：RAG 问答 Agent、高级数据分析 Agent、报表生成 Agent。
- 在 Go 底座实现基于消息队列（如 RocketMQ）的长耗时异步任务调度机制，防止复杂分析任务导致 HTTP/gRPC 网关超时。
- 完善多角色 Agent 之间的共享记忆（Shared Context）与中间思考路径（Trace）的回放保存机制。

**Non-Goals:**
- 暂不开发可视化前端拖拽构建 Agent 流的工作流画板（目前仅在代码层与 API 层实现编排定义）。
- 暂不涉及包含图片、语音等多模态大模型的直接分析，目前仅专注纯文本与结构化关系型数据的智能处理。
- 暂不引入重量级的分布式向量数据库集群（如 Milvus Cluster），首期使用轻量级向量检索方案（如 Chroma / FAISS）即可。

## Decisions

1. **核心多 Agent 编排框架：采用 LangGraph**
   - **Rationale**: 相比于直接手写状态机、或使用传统 LangChain AgentExecutor，LangGraph 原生支持图结构的循环与流转。这使我们能清晰定义每个专职 Agent 的边界与依赖关系，并利用其 Checkpointer 特性对中间状态进行原生持久化存储。

2. **异步调度模型：Go-Workflow + RocketMQ 架构**
   - **Rationale**: 报表生成和深度分析往往耗时超过 30 秒。我们在 Go 层采用异步下发策略，接收请求后立即向客户端返回 `TaskID`；底层任务推入 RocketMQ，Python Agent 异步消费执行，执行完成后再回调 Go 接口写入结果状态。此方案可有效削峰并提供极致的高并发稳定性。

3. **向量检索方案选型：Chroma**
   - **Rationale**: 在初期 MVP 阶段，由于企业的数据字典、指标口径数据量相对有限，轻量级、无独立服务依赖的 Chroma 或 FAISS 能快速启动并在隔离环境中良好工作，未来如有需要可轻松迁移。

4. **上下文与状态共享结构设计（State Object）**
   - **Rationale**: LangGraph 的流转核心在于共享的 State。我们将定义标准的全局状态字典（包括 User_Input, NL2SQL_Result, RAG_Context, Analysis_DF, Final_Report），从而确保处于下游流水线的报表 Agent 能够精准地继承和解析上游分析 Agent 的结构化结果。

## Risks / Trade-offs

- **系统追踪复杂度成倍增加（Trade-off）**：跨语言的长链路异步执行使得排查错误变得困难。必须依靠前序引入的 OpenTelemetry 基础设施进行统一的 trace_id 透传，方可确保任务的可观测性。
- **LLM Token 消耗失控与上下文超限（Risk）**：多 Agent 流转过程中若直接在上下文中传递过大的全量 DataFrame，将迅速耗尽上下文窗口。必须在 State 更新前进行强制数据截断或生成缩略摘要（Summary）。
- **异步链路的重试与幂等性（Risk）**：长任务容易因网络波动而失败。需要在 RocketMQ 的死信队列机制与 Go 调度层面增加重试容错与状态回滚补偿，同时保证 Python Worker 的执行严格满足幂等性。

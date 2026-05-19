## Context

当前项目存在三个独立的技术债务：

1. **AI Worker（Python）**：`services/ai/hub_ai/` 的 gRPC 服务端处于桩代码阶段，未实现实际 servicer。当前 NL2SQL 工作通过 Go API → gRPC 客户端 → Python 端 `_client.py` → 内部 HTTP 回调 Go API 的方式运行，形成一个低效的"自我调用"回路。

2. **前端 App.tsx**：480+ 行的巨型组件，集成了会话管理、消息展示、进度跟踪、模式选择、数据源选择、SSE 订阅等全部逻辑，状态全部通过顶层 `useState` 管理，难以维护和扩展。

3. **知识库 RAG**：`internal/handlers/handlers.go:635` 的 `postMessageToKnowledge` 中存在显式 `// TODO: 集成 ChromaDB 检索`, 当前回退到直接 LLM 问答，ChromaDB 向量检索未接入。

## Goals / Non-Goals

**Goals:**
- 实现 Python AI Worker 完整的 gRPC servicer（GenerateSQL / RunAgentPipeline / Health）
- 移除前端 App.tsx 的自调用回路，走正确 gRPC → Python 路径
- 将 App.tsx 拆分为多个独立子组件，引入 React Context 管理共享状态
- 完成 ChromaDB 向量检索集成，实现完整 RAG 流水线
- 保持所有现有 API 契约不变

**Non-Goals:**
- 不改变 Go API 的 HTTP 路由结构
- 不引入新的前端框架（保持 React + Vite）
- 不修改现有的知识点上传/存储逻辑
- 不做多租户 ChromaDB 隔离改造（当前每个 workspace 一个 collection 的设计不变）

## Decisions

### 1. AI Worker：用 gRPC async servicer 替代内部 HTTP 回调

**现状问题**：`_client.py` 中 `internal_nl2sql_sync` 通过 HTTP POST 到 Go API 的 `/internal/tasks/.../callback`，形成 Go→Python→Go 的回环。

**决策**：在 `hub_ai/__main__.py` 中实现真正的 gRPC async servicer：
- `GenerateSQL`：接收 `GenerateSQLRequest`，调用 LLM 生成 SQL，返回 `GenerateSQLResponse`
- `RunAgentPipeline`：接收 `RunAgentPipelineRequest`，启动 LangGraph 图，流式返回步骤结果
- `Health`：返回服务健康状态

**替代方案考虑**：
- 保持 HTTP 回调不变：放弃了，因为增加延迟和复杂度，且无法发挥 gRPC 流式返回的优势
- 使用同步 gRPC：放弃了，Python asyncio 更适合与 LangGraph 和 ChromaDB 集成

### 2. 前端拆分：按功能域拆分为独立组件 + Context

**策略**：
- 将 App.tsx 拆解为：
  - `SessionProvider` (Context)：管理会话列表、当前会话、消息加载
  - `QueryProvider` (Context)：管理查询模式（quick/deep）、数据源选择
  - `ProgressProvider` (Context)：管理步骤进度、计时器
  - `ChatInput`：输入框 + 发送按钮
  - `MessageList`：消息列表展示
  - `KnowledgeQueryStatus`：知识库查询状态提示
- App.tsx 降级为编排层，仅组装 Provider 和布局

**替代方案考虑**：
- Redux/Zustand：放弃了，MVP 阶段 Context 足够，避免额外依赖
- 保持现状：放弃了，480+ 行组件已难以维护

### 3. RAG 集成：Python 侧实现检索，Go 侧通过 gRPC 调用

**策略**：
- ChromaDB 检索逻辑已在 `services/ai/rag/knowledge_base.py` 中实现（`KnowledgeBase.search`）
- handlers.go 中的 `postMessageToKnowledge` 改为通过 gRPC 调用 Python 的 `RAGSearch` 方法
- 如果 AI Worker 不可用，回退到当前直接 LLM 问答

**替代方案考虑**：
- Go 侧直接调 ChromaDB：放弃了，Python 生态的 embedding 和 ChromaDB 集成更成熟
- 完全在 Go 侧做 RAG：放弃了，会增加 embedding 模型部署的复杂度

## Risks / Trade-offs

- **[风险] AI Worker 实现后，依赖 gRPC 的稳定性** → 缓解：保留 fallback 逻辑，gRPC 不可用时回退到直接 LLM
- **[风险] 前端拆分可能引入回归** → 缓解：逐步拆分，每次拆完运行现有测试 `npm test`
- **[风险] ChromaDB 性能问题** → 缓解：当前 collection 规模较小，搜索 top_k=3，延迟可控
- **[风险] 三个改进并行可能互相干扰** → 缓解：三个方向代码域独立（Python/前端/Go），可并行开发

## Open Questions

- AI Worker 的 gRPC servicer 是否需要支持流式返回（server streaming）？当前设计先实现 unary，后续可按需升级
- 前端 Context 拆分后，localStorage 持久化逻辑归属？先放在对应的 Provider 中

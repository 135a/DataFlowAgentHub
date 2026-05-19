## Why

当前项目有三个关键短板阻碍 MVP 完成度：Python AI Worker 的 gRPC 服务端仍是桩代码（未实现实际 servicer），前端 `App.tsx` 是 480+ 行的巨型组件难以维护，知识库 RAG 的 ChromaDB 向量检索集成处于 TODO 状态（当前是直接 LLM 问答回退）。这三个问题直接影响产品的核心 AI 能力交付、前端可维护性和知识库功能的实际可用性。

## What Changes

- **AI Worker gRPC 服务端完成**：实现 `hub_ai/__main__.py` 中 gRPC servicer 的 `GenerateSQL`、`RunAgentPipeline`、`Health` 等 RPC 方法，结束桩代码阶段
- **前端 `App.tsx` 组件拆分**：将巨型组件拆分为多个独立子组件，引入 Context 或轻量状态管理，提升可维护性
- **ChromaDB RAG 集成完成**：移除 handlers.go 中的 TODO，连接 ChromaDB 向量检索，实现文档分块 → 嵌入 → 语义搜索 → LLM 问答的完整流水线

## Capabilities

### New Capabilities
- `ai-worker-impl`: 实现 Python AI Worker 的 gRPC 服务端完整 servicer，支持 GenerateSQL（NL2SQL）、RunAgentPipeline（Multi-Agent 编排）、Health（健康检查）
- `frontend-architecture`: 拆分巨型 App.tsx 组件，引入分层状态管理，提升前端代码可维护性
- `rag-integration`: 完成 ChromaDB 向量检索集成，实现从文档上传、分块、嵌入到语义搜索的完整知识库 RAG 流水线

### Modified Capabilities

（无现有 spec 需求变更）

## Impact

- **后端**：Go API handlers 中的 `postMessageToKnowledge` 需要对接 ChromaDB 而非直接 LLM
- **Python Worker**：`hub_ai/` 包需实现完整 gRPC servicer，与 Go 侧 `worker/nl2sql.go` 客户端对接
- **前端**：`web/src/App.tsx` 拆分为多个子组件，可能引入 React Context，需确保与现有路由和会话逻辑兼容
- **基础设施**：Docker Compose 中 worker 镜像可能需要调整以加载完整服务
- **现有知识库文档上传流程不变**，仅后端检索逻辑变更

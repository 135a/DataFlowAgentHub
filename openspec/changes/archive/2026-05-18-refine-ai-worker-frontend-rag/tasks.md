## 1. AI Worker gRPC 服务端实现

- [x] 1.1 在 `hub_ai/_server.py` 中实现 gRPC servicer 类，继承自动生成的 `HubAIServicer`
- [x] 1.2 实现 `GenerateSQL` 方法：接收请求 → 调用 LLM → 返回 SQL
- [x] 1.3 实现 `RunAgentPipeline` 方法：接收请求 → 启动 LangGraph StateGraph → 返回结果
- [x] 1.4 实现 `Health` 方法：检查服务依赖 → 返回 SERVING/NOT_SERVING
- [x] 1.5 在 `hub_ai/__main__.py` 中启动 gRPC async server，注册 servicer
- [x] 1.6 更新 Go 侧 `worker/nl2sql.go` 直接调用 gRPC，移除内部 HTTP 回调回路
- [x] 1.7 验证 gRPC 方法通过 `go test` 和 `pytest` 测试

## 2. 前端 App.tsx 组件拆分

- [x] 2.1 创建 `SessionContext.tsx`（SessionProvider）：提取会话列表、消息加载、SSE token 逻辑
- [x] 2.2 创建 `QueryContext.tsx`（QueryProvider）：提取查询模式、数据源选择、localStorage 持久化
- [x] 2.3 创建 `ProgressContext.tsx`（ProgressProvider）：提取步骤进度、计时器、耗时估算逻辑
- [x] 2.4 创建 `ChatInput.tsx` 独立组件：输入框 + 发送按钮 + 禁用状态
- [x] 2.5 创建 `MessageList.tsx` 独立组件：消息列表渲染
- [x] 2.6 创建 `KnowledgeQueryStatus.tsx` 独立组件：知识库查询状态提示
- [x] 2.7 重构 `App.tsx`：移除业务逻辑，降级为 Provider + 布局编排层
- [x] 2.8 运行 `npm test` 验证现有测试通过

## 3. ChromaDB RAG 集成

- [x] 3.1 在 AI Worker 中实现 `RAGSearch` gRPC 方法：ChromaDB 检索 + LLM 问答
- [x] 3.2 在 AI Worker 中实现 `IndexDocument` gRPC 方法：文档分块 → embedding → ChromaDB 存储
- [x] 3.3 更新 proto 定义，添加 `RAGSearch` 和 `IndexDocument` RPC
- [x] 3.4 重新生成 Go 和 Python gRPC 桩代码（`make gen-go` + `make gen-py`）
- [x] 3.5 修改 Go API `postMessageToKnowledge`：通过 gRPC 调用 Worker RAGSearch，添加 fallback 逻辑
- [x] 3.6 修改 Go API 知识库文档上传处理：上传成功后调用 Worker IndexDocument 做向量化
- [x] 3.7 验证 ChromaDB 检索流程通过 `pytest` 测试

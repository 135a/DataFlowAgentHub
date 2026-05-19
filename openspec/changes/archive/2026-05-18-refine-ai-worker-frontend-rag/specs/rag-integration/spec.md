## ADDED Requirements

### Requirement: Go API 通过 gRPC 调用 Python Worker 执行 RAG 检索

`postMessageToKnowledge` SHALL 通过 gRPC（而非直接 LLM 调用）将用户问题发送到 Python AI Worker 执行 ChromaDB 检索增强生成。

#### Scenario: 启用 Worker 时走 RAG 路径
- **WHEN** AI Worker gRPC 可用且收到知识库查询请求
- **THEN** 调用 Worker 的 `RAGSearch` 方法，传入用户问题和 workspace_id，返回检索到的文档片段列表和 LLM 生成的答案

#### Scenario: Worker 不可用时回退到直接 LLM
- **WHEN** AI Worker gRPC 不可用（连接失败或超时）
- **THEN** `postMessageToKnowledge` 回退到当前直接 LLM 问答模式，不抛出错误

### Requirement: Python Worker 实现 RAGSearch gRPC 方法

Python AI Worker SHALL 实现 `RAGSearch` gRPC 方法，执行 ChromaDB 向量检索 + LLM 生成回答的完整 RAG 流水线。

#### Scenario: 成功检索并生成回答
- **WHEN** `RAGSearch` 收到有效的 `RAGSearchRequest`（含 question 和 workspace_id）
- **THEN** 调用 `KnowledgeBase.search` 检索 top_k 相关文档片段，将片段作为 context 发送给 LLM 生成回答，返回包含 answer 和 sources 的 `RAGSearchResponse`

#### Scenario: ChromaDB 中没有相关文档
- **WHEN** 知识库中未找到与问题相关的文档片段
- **THEN** LLM 仅基于自身知识生成回答，返回的 `sources` 列表为空，`answer` 中提示"未在知识库中找到直接相关的文档"

#### Scenario: ChromaDB 连接失败
- **WHEN** ChromaDB 不可用（连接失败或超时）
- **THEN** 返回 `RAGSearchResponse` 其中 `ok=false`、`error_message` 描述 ChromaDB 连接异常

### Requirement: 知识库文档上传后自动触发 ChromaDB 嵌入

当通过知识库管理页面上传文档时，系统 SHALL 自动将文档分块、生成 embedding 并存入 ChromaDB。

#### Scenario: 上传文档后自动向量化
- **WHEN** 用户上传新的知识库文档（通过 `UploadKnowledgeDoc` 或 `UploadKnowledgeDocFromFile`）
- **THEN** Go API 在保存文件后通过 gRPC 调用 Worker 的 `IndexDocument` 方法，将文档内容分块、生成 embedding 并存入 ChromaDB collection

#### Scenario: 文档向量化失败不影响上传
- **WHEN** 文档保存成功但 ChromaDB 索引失败
- **THEN** 文件仍保存在磁盘，上传接口返回 200，但响应中包含 `"rag_indexed": false` 和 `"rag_error"` 描述索引失败原因

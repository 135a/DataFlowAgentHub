## ADDED Requirements

### Requirement: Knowledge base Q&A
The system SHALL support natural language question answering using RAG (ChromaDB retrieval + LLM generation).

#### Scenario: Successful knowledge query
- **WHEN** user sends a message in knowledge base mode
- **THEN** system searches ChromaDB for relevant document chunks (top_k=3)
- **THEN** system constructs a prompt with retrieved context
- **THEN** system calls LLM to generate an answer
- **THEN** system returns `{ answer: "...", sources: [...] }`

#### Scenario: No relevant documents found
- **WHEN** ChromaDB returns no relevant results for the query
- **THEN** system includes note "未找到相关文档，回答可能不准确"
- **THEN** system still calls LLM to generate a best-effort answer

#### Scenario: Knowledge query timeout
- **WHEN** the total RAG + LLM operation exceeds 15 seconds
- **THEN** system returns error "知识库查询超时，请重试"

## ADDED Requirements

### Requirement: 知识库向量化及精准检索
集成向量数据库（如 Chroma/FAISS），将业务词典及指标定义持久化，并在需要时通过 RAG 技术召回补充业务上下文。

#### Scenario: 垂直领域口径查询增强
- **WHEN** 用户的提问中包含系统不确定的专用指标或表字段名（如“按核心有效活跃用户口径统计”）。
- **THEN** RAG Agent 优先从向量知识库中进行相关度检索，获取该业务名词的准确 SQL 定义解释，并将该信息附带在 prompt 上下文中供 NL2SQL Agent 生成最终查询语句时使用。

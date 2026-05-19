## ADDED Requirements

### Requirement: LLMProvider 接口定义

系统 SHALL 定义一个 `LLMProvider` 接口，抽象所有 LLM 调用，使得更换供应商无需修改业务逻辑代码。

#### Scenario: 接口包含核心方法
- **WHEN** 系统使用 LLMProvider 接口
- **THEN** 接口包含 `GenerateSQL(ctx, request) → (sql, notes, error)` 方法
- **AND** 接口包含 `Ask(ctx, question, context) → (answer, error)` 方法（用于 RAG）

#### Scenario: 接口定义在独立包中
- **WHEN** 开发者在 Python 代码中引用 LLM 接口
- **THEN** 接口定义在 `services/ai/llm_provider.py` 文件中
- **AND** 不依赖任何特定供应商的 SDK

### Requirement: OpenAIProvider 实现

系统 SHALL 提供 `OpenAIProvider` 作为 `LLMProvider` 的默认实现，将现有 `_openai_sql` 和 `_rag_answer` 逻辑迁移至其下。

#### Scenario: OpenAIProvider 实现接口
- **WHEN** 系统使用 `OpenAIProvider`
- **THEN** 它实现 `LLMProvider` 接口
- **AND** 使用 `AsyncOpenAI` 客户端调用 OpenAI API
- **AND** 通过 `OPENAI_API_KEY`、`OPENAI_BASE_URL`、`OPENAI_MODEL` 环境变量配置

#### Scenario: 工厂函数按需创建 Provider
- **WHEN** 系统启动时
- **THEN** 通过工厂函数根据环境变量创建对应 Provider
- **AND** `OPENAI_API_KEY` 存在时创建 `OpenAIProvider`
- **AND** 无有效配置时创建 `FallbackProvider`

### Requirement: 修复 SQL 生成 prompt 中的 PostgreSQL 引用

系统 SHALL 将 `_openai_sql` 中硬编码的 `"You are a Postgres SQL generator"` 修改为 `"You are a MySQL SQL generator"`，与实际的数据库方言一致。

#### Scenario: Prompt 使用 MySQL 方言
- **WHEN** `OpenAIProvider` 生成 SQL
- **THEN** system prompt 指示模型生成 MySQL 方言 SQL
- **AND** 不再引用 PostgreSQL

### Requirement: FallbackProvider 实现

系统 SHALL 提供一个降级 Provider，当无 LLM 供应商配置时返回固定响应而非空或假数据。

#### Scenario: 无 API Key 时使用 Fallback
- **WHEN** `OPENAI_API_KEY` 未设置
- **THEN** 系统使用 `FallbackProvider`
- **AND** `GenerateSQL` 返回硬编码的 `SELECT 1 AS ok`
- **AND** `Ask` 返回 "No LLM provider configured"

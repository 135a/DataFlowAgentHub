# prompt-documentation 规格说明

## ADDED Requirements

### Requirement: NL2SQL Prompt 文档

系统 SHALL 在 `docs/PROMPTS.md` 中记录 NL2SQL Prompt 的完整内容、设计理念和参数配置。

#### Scenario: Prompt 内容可查

- **WHEN** 面试官或开发者阅读 `docs/PROMPTS.md`
- **THEN** 可找到 NL2SQL Prompt 的完整文本、temperature=0.1 理由（需要确定性 SQL 输出）、规则说明（仅 SELECT、无 DDL/DML、public schema 默认）

#### Scenario: 设计理念可展开

- **WHEN** 面试中问到 "为什么这样设计 NL2SQL prompt"
- **THEN** 文档包含足够的上下文供面试者展开：schema 注入策略、zero-shot vs few-shot 决策、安全规则设计

### Requirement: 数据分析 Prompt 文档

系统 SHALL 在 `docs/PROMPTS.md` 中记录数据分析 Agent 的 Prompt 内容、模型配置和设计理念。

#### Scenario: Analysis Prompt 文档化

- **WHEN** 阅读 `docs/PROMPTS.md` 的数据分析部分
- **THEN** 可找到 system prompt 文本、temperature=0.3 理由（平衡创造性与准确性）、user message 结构（用户意图 + 统计摘要）

### Requirement: 报告生成模板文档

系统 SHALL 在 `docs/PROMPTS.md` 中记录报告生成的 Markdown 模板结构和数据映射关系。

#### Scenario: 报告模板可理解

- **WHEN** 阅读 `docs/PROMPTS.md` 的报告部分
- **THEN** 可理解报告包含的章节（标题、用户请求、分析摘要、数据表格、文件输出）及不含 LLM 调用的设计决策

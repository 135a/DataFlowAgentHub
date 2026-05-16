## Why

当前项目的 LLM Prompt 分散在 Python 代码中（`__main__.py` 的 NL2SQL prompt、`data_analysis_agent.py` 的 analysis prompt），缺乏集中文档。面试时需要能展开讲解 Prompt 设计理念、演进历史和调优策略。

## What Changes

- 创建 `docs/PROMPTS.md`：记录所有 LLM Prompt 的完整内容、设计理念、参数配置
- 文档化以下 Prompt：
  - **NL2SQL Prompt**（`hub_ai/__main__.py`）：Postgres SQL 生成 prompt
  - **Data Analysis Prompt**（`agents/data_analysis_agent.py`）：数据分析摘要 prompt
  - **Report Generation**（`agents/report_generation_agent.py`）：报告生成逻辑（非 LLM，但记录模板结构）
- 记录每个 Prompt 的模型配置、temperature、token 策略

## Capabilities

### New Capabilities

- `prompt-documentation`: 系统所有 LLM Prompt 的集中文档，包含设计理念和参数配置

### Modified Capabilities

<!-- 纯文档新增 -->

## Impact

- 新增 `docs/PROMPTS.md`
- 无代码变更

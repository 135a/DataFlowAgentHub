## Context

项目当前有 2 个 LLM 调用点，使用 OpenAI 兼容 API：

| 位置 | 角色 | 模型 | Temperature |
|---|---|---|---|
| `hub_ai/__main__.py:_openai_sql()` | NL2SQL 生成 | `gpt-4o-mini` | 0.1 |
| `agents/data_analysis_agent.py` | 数据分析摘要 | `gpt-4o-mini` | 0.3 |

`report_generation_agent.py` 不使用 LLM，纯模板拼接生成 Markdown。

## Goals / Non-Goals

**Goals:**
- 创建 `docs/PROMPTS.md`，包含所有 Prompt 完整内容
- 记录设计理念（为什么这样写 prompt）
- 记录参数选择理由（temperature、model）

**Non-Goals:**
- 不修改任何 Prompt 内容
- 不新增 Prompt 版本管理机制

## Decisions

### 1. 文档结构

```
docs/PROMPTS.md
├── NL2SQL Prompt
│   ├── 位置、触发条件
│   ├── 完整 Prompt 文本
│   ├── 设计理念（zero-shot、schema 注入、防注入规则）
│   └── 参数配置
├── 数据分析 Prompt
│   ├── 位置、触发条件
│   ├── 完整 Prompt 文本
│   ├── 设计理念（system role、统计摘要上下文）
│   └── 参数配置
└── 报告生成模板
    ├── 位置、触发条件
    └── 模板结构说明
```

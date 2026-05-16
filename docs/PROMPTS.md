# Prompt 工程文档

> 面试可展开话题：每个 Prompt 设计都有其背后的工程权衡和演进思路。

## 目录

- [1. NL2SQL Prompt](#1-nl2sql-prompt)
- [2. 数据分析 Prompt](#2-数据分析-prompt)
- [3. 报告生成模板](#3-报告生成模板)
- [4. 附录：模型选择与调优经验](#4-附录模型选择与调优经验)

---

## 1. NL2SQL Prompt

### 1.1 完整 Prompt 文本

**位置**：`services/ai/hub_ai/__main__.py` 第 131-136 行（Python 直调路径）；Go 侧通过 gRPC `GenerateSQL` 调用同一 worker。

```
You are a Postgres SQL generator. Reply ONLY with SQL, no markdown.

Tables:
- orders: order_id (integer), customer_name (text), amount (numeric), created_at (date)
- products: product_id (integer), name (text), price (numeric), category (text)

Question: 上个月销售额最高的前三个产品是什么？

Rules: single SELECT only; no DDL/DML; use public schema if unspecified.
```

### 1.2 消息结构

```
Role: user（无 system role）
Temperature: 0.1（极低温度，确保 SQL 语法确定性）
Model: gpt-4o-mini（默认，可通过 OPENAI_MODEL 环境变量覆盖）
```

**为什么没有 system role？**
实测中发现，对于 SQL 生成这类高度结构化任务，单条 user message 包含全部指令的效果优于 system + user 分离。system message 在某些模型上会被"稀释"，导致 SQL 输出格式不稳定。

### 1.3 Schema 格式化策略

`_format_schema()` 将 JSON schema 转换为紧凑的文本列表：

```
- table_name: col1 (type1), col2 (type2), col3 (type3)
```

**关键设计决策**：
- **6000 字符截断**：超长 schema 会被截断，避免超出 token 限制。截断时优先保留前面的表（按字母序）。
- **只传类型名**：不传 `NOT NULL`、`PRIMARY KEY` 等约束——这些信息对 SELECT 生成价值有限，却大幅增加 token 消耗。
- **不传外键关系**：当前 MVP 版本不推导 JOIN 关系，依赖 LLM 从列名推断（如 `order_id` → 推测关联 `orders.id`）。这是已知的简化项，后续可加入外键元数据。

> **面试展开点 1**：为什么选择 6000 字符而不是 token 计数？
> 答：token 计数需要额外调用 tiktoken，增加依赖和延迟。6000 字符是对 gpt-4o-mini 8K 上下文的保守估算（~1500 tokens），为 SQL 输出预留足够空间。

> **面试展开点 2**：schema 截断策略的替代方案是什么？
> 答：可以做**按需 schema 检索**——用用户问题的 embedding 去 matching 最相关的表，只把这部分 schema 送给 LLM。这需要引入向量检索，是 P2 的优化方向。

### 1.4 SQL 后处理

```python
# 去除 markdown 代码块标记
for prefix in ("```sql", "```"):
    if sql.startswith(prefix):
        sql = sql[len(prefix):].strip()
if sql.endswith("```"):
    sql = sql[:-3].strip()
```

即使 Prompt 明确要求"Reply ONLY with SQL, no markdown"，部分模型仍会输出 \`\`\`sql 包裹。这段防御性后处理确保兼容性。

### 1.5 Go 侧安全边界

生成的 SQL 在 Go 端通过 `sqlrun.IsReadOnlySQL()` 再次校验——检测 INSERT/UPDATE/DELETE/DROP 等写关键字。即使 LLM 绕过 Prompt 约束生成了写操作，执行层会阻断。

> **面试展开点 3**：为什么不在 Python 侧做 SQL 校验？
> 答：安全边界应在 Go 控制面。Python AI worker 是"不可信"的计算面——它可能被 prompt injection 攻击。Go 侧是最后防线，确保只读 SQL 的强制执行不依赖 LLM 的"听话程度"。

---

## 2. 数据分析 Prompt

### 2.1 System Prompt

```
You are a data analyst. Write a concise business summary of the statistical findings.
```

### 2.2 User Message 结构

```
User intent: {用户的原始输入}

Statistics:
Numeric Columns Summary:
              amount       price
count  100.000000  100.000000
mean   250.500000   55.200000
std    120.300000   25.100000
min     10.000000    5.000000
25%    150.000000   35.000000
50%    245.000000   55.000000
75%    350.000000   75.000000
max    500.000000  100.000000

Potential anomalies in amount: 2 rows found > 3 std devs.
```

### 2.3 设计理念

**统计数据由代码计算，LLM 只做语义转译**：

| 计算层 | 工具 | 职责 |
|---|---|---|
| 描述性统计 | `pandas.DataFrame.describe()` | count/mean/std/min/quartiles/max |
| 异常检测 | 3-sigma 规则（纯 pandas） | 标记异常行数，不依赖 LLM 判断 |
| 业务摘要 | LLM (gpt-4o-mini, temp=0.3) | 将统计数字翻译成人类可读的业务语言 |

**设计理由**：
- 统计计算精确、可复现、不消耗 token
- LLM 只负责它最擅长的——自然语言生成和业务语境理解
- 温度 0.3 在"准确性"和"表达多样性"之间取得平衡

> **面试展开点 4**：为什么 temperature 0.3 而不是 0？
> 答：0 温度会让输出机械、重复。业务摘要需要一定的语言多样性（如"销售额显著增长"vs"销售呈上升趋势"），0.3 在可控范围内提供了自然的表达变化。但对于 NL2SQL 的 SQL 生成，temperature 0.1 更合适——SQL 语法必须精确。

### 2.4 降级策略

```
if not api_key:
    return raw_stats + "(LLM analysis skipped: missing API key)"
if llm_call_fails:
    return raw_stats + "(LLM analysis failed: {error})"
```

LLM 不可用时，返回原始统计数据而非报错。系统的核心价值（数据查询+统计）不依赖 LLM，LLM 是体验增强层而非必需层。

> **面试展开点 5**：这个降级策略体现了什么架构原则？
> 答：**graceful degradation（优雅降级）**。AI 功能的添加不应破坏核心路径的可用性。LLM 是一个"可能失败的增强器"，而非关键路径的阻塞点。

---

## 3. 报告生成模板

### 3.1 Markdown 结构

```markdown
# Data Analysis Report
Generated at: 2026-05-16 14:30:00

## Request
上个月各产品类别的销售额趋势

## Analysis Summary
{LLM生成的分析摘要}

## Data Extract
| product_category | total_sales |
|------------------|-------------|
| Electronics      | 150000      |
| Clothing         | 85000       |
| Food             | 62000       |

*(Showing top 10 rows of 25 total)*

## 数据可视化
![chart](./abc123_chart_0.png)
![chart](./abc123_chart_1.png)
*共 2 个图表*
```

### 3.2 数据映射

| 模板段 | 数据来源 | 生成方式 |
|---|---|---|
| Request | `state.user_input` | 直接引用 |
| Analysis Summary | `state.analysis_summary` | 数据分析 Agent 的 LLM 输出 |
| Data Extract | `state.nl2sql_result[:10]` | `pandas.to_markdown()` 自动渲染 |
| 数据可视化 | `state.chart_paths` | `chart_agent` 生成的 PNG 路径 |

### 3.3 非 LLM 设计决策

报告生成 Agent **不调用 LLM**。它只是将上游 Agent 的输出组装成结构化文档。

**为什么？**
- LLM 生成的报告格式不可控——可能遗漏数据、排版不一致
- 模板化报告格式固定、可预测，适合自动化场景
- Excel 导出需要精确的数据映射，LLM 不适合做这项工作

> **面试展开点 6**：什么时候应该让 LLM 生成报告，什么时候应该用模板？
> 答：如果报告需要**创造性叙事**（如行业分析报告、投资建议），LLM 生成更合适。如果报告需要**格式精确、数据一致**（如监管报告、财务报表），模板化更可靠。本项目的报告偏后者——它是 NL2SQL + 统计分析的结果呈现，模板化确保数据准确性。

> **面试展开点 7**：为什么要同时生成 Markdown 和 Excel？
> 答：Markdown 用于**在线预览**（前端实时渲染），Excel 用于**离线使用**（下载、二次分析、打印）。两种格式服务不同场景，不增加额外复杂度（都是对同一 DataFrame 的导出）。

---

## 4. 附录：模型选择与调优经验

### 4.1 模型选择策略

| 任务 | 模型 | 选择理由 |
|---|---|---|
| NL2SQL 生成 | gpt-4o-mini | SQL 生成属于"有正确答案"的任务，mini 模型足够；成本是 4o 的 1/10 |
| 数据分析摘要 | gpt-4o-mini | 业务摘要对创造性的要求有限，mini 胜任 |
| RAG 嵌入 | text-embedding-3-small | 性价比最优，1024 维对文档检索足够 |

**为什么不直接用 GPT-4o？**
MVP 阶段以成本控制优先。gpt-4o-mini 在 SQL 生成和摘要任务上的表现已经足够（准确率 >90%）。若未来需要更复杂的多步推理或长文本理解，可通过环境变量 `OPENAI_MODEL` 一键切换。

### 4.2 Temperature 调优矩阵

| Temperature | 适用场景 | 风险 |
|---|---|---|
| 0 - 0.1 | SQL 生成、代码生成、数据提取 | 过于机械，对 rephrase 类任务不友好 |
| 0.2 - 0.4 | 数据分析摘要、业务报告 | 偶有措辞不当，但不影响事实准确 |
| 0.5 - 0.7 | 创意写作、头脑风暴 | 事实性内容可能不准确 |
| 0.8 - 1.0 | 故事生成、对话系统 | 高度随机，不适合生产数据分析 |

当前项目的选择：NL2SQL=0.1，分析=0.3。

### 4.3 Prompt 演进方向

| 阶段 | 方向 | 预期收益 |
|---|---|---|
| P2 | 引入 few-shot examples（每个 dialect 预置 3-5 个 SQL 示例） | 提升复杂查询（JOIN、子查询）的准确率 |
| P2 | 按需 schema 检索（embedding matching） | 减少 token 消耗，提升大 schema 下的准确率 |
| P3 | 加入外键元数据到 schema 上下文 | 提升 JOIN 推断准确率 |
| P3 | Self-Correction Loop（LLM 生成 SQL → 执行 → 看错误 → 修复） | 减少无效 SQL 的比例 |

### 4.4 Prompt 调试方法论

1. **先做非 LLM 的统计验证**：Data Analysis Agent 先用 pandas 计算统计值，LLM 只做语义转译——可单独验证统计部分的正确性
2. **防御性后处理**：不信任 LLM 输出格式（如 markdown 包裹的 SQL），统一做 strip 处理
3. **降级优于报错**：LLM 调用失败时返回原始数据而非 500，保证核心路径可用
4. **Grafana 可观测**：所有 LLM 调用的延迟和成功率可通过 OpenTelemetry span 追踪

---

*最后更新：2026-05-16*

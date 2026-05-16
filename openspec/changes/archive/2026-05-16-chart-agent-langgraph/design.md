## Context

当前 LangGraph 拓扑为 3 节点线性管道：`START → nl2sql_node → (条件路由) → analysis_node → report_node → END`。新增 `chart_node` 后变为 4 节点图：

```
START
  |
  v
nl2sql_node
  |
  v
route_next() ──→ __end__ (simple)
  |
  v
route_analysis() ──→ analysis_node ──→ report_node → END
  |
  v
chart_node ────────────┘
```

关键设计：`chart_node` 与 `analysis_node` 并行分支，两者都完成后汇聚到 `report_node`。

## Goals / Non-Goals

**Goals:**
- 新增 `chart_agent_node(state)` 函数，从 `nl2sql_result` 生成图表
- 自动选择图表类型：数值列 ≥ 2 → 柱状图/折线图，单列分类 → 饼图
- 修改 `graph.py` 的节点注册和路由逻辑，新增 `chart_node`
- `report_node` 在 Markdown 报告中嵌入图表路径引用
- 图表保存为 PNG 至 `/tmp/reports/`

**Non-Goals:**
- 不实现交互式图表（如 Plotly HTML）
- 不修改 gRPC proto 定义
- 不修改 Go 端代码
- 不引入外部图表服务（如 QuickChart.io）

## Decisions

### 1. 图表库选择：matplotlib

**选择**：使用 `matplotlib` 生成 PNG 图表。

**理由**：
- Python 生态最成熟的绑图库，无外部服务依赖
- 直接输出 PNG 文件，适合嵌入 Markdown 报告
- 中文支持通过 `matplotlib` 的 `font_manager` 配置中文字体

**替代方案考虑**：
- **Plotly**：交互式图表更美观，但输出 HTML 不适合嵌入纯 Markdown 报告
- **Seaborn**：基于 matplotlib 的封装，引入额外依赖但价值有限

### 2. 图表类型自动选择策略

**选择**：基于 NL2SQL 结果的数据列类型自动选择：

| 条件 | 图表类型 |
|---|---|
| 1 个数值列 + 1 个文本列 | 柱状图 (`bar`) |
| 2+ 个数值列 + 1 个文本列 | 分组柱状图 |
| 1 个数值列 + 1 个时间列 | 折线图 (`line`) |
| 1 个文本列 + 1 个数值列且类别 ≤ 6 | 饼图 (`pie`) |
| 无法判断 | 柱状图（默认） |

**理由**：覆盖最常见的分析场景，无需用户手动指定图表类型。

### 3. 并行执行策略

**选择**：`chart_node` 和 `analysis_node` 在同一路由层级分支，LangGraph 会串行执行（当前版本不支持真正并行节点），但两者独立不互相依赖。

**理由**：LangGraph StateGraph 的边模型是串行的，但通过合理的边定义可以实现 `nl2sql_node → chart_node → report_node` 和 `nl2sql_node → analysis_node → report_node` 两条并行路径汇聚。

实际拓扑（修订后）：
```
nl2sql_node 后的条件路由:
  - "simple" 或无关键字 → __end__
  - 包含 chart/图表 关键字 → chart_node → report_node → END
  - 包含 analyze/分析 关键字 → analysis_node → report_node → END
  - agent_pipeline → analysis_node → chart_node → report_node → END
```

### 4. chart_node 执行顺序

在 `agent_pipeline` 模式下：`analysis_node` → `chart_node` → `report_node`（串行链路），chart 使用 analysis 的统计结果辅助图表标注。

### 5. 状态扩展

`AgentState` 新增字段：
- `chart_paths: list[str]` — 生成的图表 PNG 文件路径列表

## Risks / Trade-offs

- **matplotlib 中文乱码**：Docker 镜像可能缺少中文字体 → `chart_agent` 内配置回退字体，优先使用 `SimHei`/`Noto Sans CJK`，否则使用英文标签
- **大数据集图表性能**：NL2SQL 结果可能超过 500 行 → `chart_agent` 自动采样至最多 50 个数据点
- **图表生成失败不应阻塞流程**：chart_node 异常时设置 `chart_paths = []` 并记录警告，允许 report_node 继续执行

## Why

当前 LangGraph 只有 3 个节点（NL2SQL → Analysis → Report），缺少数据可视化能力。用户查询结果只能以表格形式呈现，无法自动生成图表。需要新增 `chart_agent` 节点，在分析阶段并行生成统计图表，丰富最终报告的可视化内容。

## What Changes

- 新增 `services/ai/agents/chart_agent.py`，实现 `chart_agent_node(state)` 函数
- 图表类型自动选择：根据数据列类型智能选择柱状图/折线图/饼图
- 修改 `orchestrator/graph.py`：新增 `chart_node`，修改路由逻辑，Agent 从 3 节点变为 4 节点
- 修改 `orchestrator/state.py`：新增 `chart_paths` 状态字段
- 修改 `agents/report_generation_agent.py`：在报告中嵌入图表引用
- 图表输出为 PNG 文件，存储至 `/tmp/reports/{run_id}_chart_{n}.png`

## Capabilities

### New Capabilities

- `chart-agent`: 提供自动化数据图表生成能力，根据 NL2SQL 查询结果智能选择图表类型并生成 PNG 图表

### Modified Capabilities

<!-- 新增 Agent 节点，不修改现有 spec 需求 -->

## Impact

- 新增 `services/ai/agents/chart_agent.py`
- 修改 `services/ai/orchestrator/graph.py`（新增 chart_node + 路由改造）
- 修改 `services/ai/orchestrator/state.py`（新增 chart_paths 字段）
- 修改 `services/ai/agents/report_generation_agent.py`（嵌入图表）
- 新增 Python 依赖 `matplotlib`
- LangGraph 拓扑从 3 节点变为 4 节点

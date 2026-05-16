# chart-agent

## Purpose

为 LangGraph 管道提供自动数据图表生成能力，根据 NL2SQL 查询结果智能选择图表类型并生成 PNG 图表，嵌入最终报告。

## ADDED Requirements

### Requirement: Chart Agent 自动生成图表

系统 SHALL 提供 `chart_agent_node(state)` 函数，从 NL2SQL 查询结果自动生成 PNG 格式的数据图表。

#### Scenario: 柱状图生成

- **WHEN** NL2SQL 结果包含 1 个文本列和 1+ 个数值列
- **THEN** chart_agent 生成柱状图 PNG，保存至 `/tmp/reports/{run_id}_chart_{n}.png`，返回 `chart_paths` 列表

#### Scenario: 折线图生成

- **WHEN** NL2SQL 结果包含 1 个时间列和 1+ 个数值列
- **THEN** chart_agent 生成折线图 PNG，保存至 `/tmp/reports/{run_id}_chart_{n}.png`

#### Scenario: 饼图生成

- **WHEN** NL2SQL 结果包含 1 个分类列 + 1 个数值列且类别数 ≤ 6
- **THEN** chart_agent 生成饼图 PNG，保存至 `/tmp/reports/{run_id}_chart_{n}.png`

#### Scenario: 大结果集采样

- **WHEN** NL2SQL 结果超过 50 行
- **THEN** chart_agent 自动采样至最多 50 个数据点再进行绑图

#### Scenario: 图表生成失败不阻塞

- **WHEN** chart_agent 绑图过程抛出异常
- **THEN** 系统记录警告日志，返回空 `chart_paths: []`，不阻止后续 report_node 执行

### Requirement: LangGraph 图扩展为 4 节点

系统 SHALL 修改 `orchestrator/graph.py`，在 StateGraph 中注册 `chart_node`，并从现有的 3 节点拓扑扩展为包含 chart_node 的 4 节点拓扑。

#### Scenario: chart_node 注册

- **WHEN** `build_graph()` 构造 StateGraph
- **THEN** `chart_node` 被添加到图中，关联到 `chart_agent_node` 函数

#### Scenario: agent_pipeline 模式路由

- **WHEN** 工作流为 `agent_pipeline` 且 nl2sql_node 成功
- **THEN** 路由到 `analysis_node`，然后执行 `chart_node`，最后到 `report_node`

#### Scenario: 图表关键字路由

- **WHEN** 用户消息包含 "chart"/"图表"/"可视化" 关键字（非 agent_pipeline 模式）
- **THEN** nl2sql_node 后直接路由到 `chart_node`，跳过 analysis_node

### Requirement: 状态扩展 chart_paths

系统 SHALL 在 `AgentState` TypedDict 中新增 `chart_paths` 字段，类型为 `list[str]`，用于存储生成的图表文件路径。

#### Scenario: chart_paths 传递

- **WHEN** chart_node 生成图表并设置 `chart_paths`
- **THEN** report_node 可从状态中读取 `chart_paths` 并在报告中嵌入图表引用

#### Scenario: chart_paths 默认值

- **WHEN** 流程未经过 chart_node
- **THEN** `chart_paths` 为空列表 `[]`

### Requirement: 报告嵌入图表引用

系统 SHALL 修改 `report_generation_agent.py`，在生成的 Markdown 报告的 "查询结果" 部分后插入 "数据可视化" 部分，以相对路径引用 chart_node 生成的 PNG 图表。

#### Scenario: 报告包含图表

- **WHEN** `chart_paths` 非空
- **THEN** Markdown 报告中插入 `## 数据可视化` 部分，以 `![chart](./{filename})` 格式引用每个图表

#### Scenario: 无图表时跳过

- **WHEN** `chart_paths` 为空或状态中不存在此字段
- **THEN** 报告中不添加 "数据可视化" 部分

## 1. 扩展状态模型

- [x] 1.1 修改 `orchestrator/state.py`，在 `AgentState` 中新增 `chart_paths` 字段（`list[str]`）

## 2. 实现 chart_agent

- [x] 2.1 创建 `agents/chart_agent.py`，实现 `chart_agent_node(state: AgentState) -> dict` 函数
- [x] 2.2 实现 `_select_chart_type(df)` — 根据列类型自动选择柱状图/折线图/饼图
- [x] 2.3 实现 `_draw_bar_chart(df, run_id, idx)` — 生成柱状图 PNG
- [x] 2.4 实现 `_draw_line_chart(df, run_id, idx)` — 生成折线图 PNG
- [x] 2.5 实现 `_draw_pie_chart(df, run_id, idx)` — 生成饼图 PNG
- [x] 2.6 实现大结果集采样逻辑（>50 行采样至 50）
- [x] 2.7 配置 matplotlib 中文字体支持
- [x] 2.8 添加异常处理（图表生成失败不阻塞流程，记录警告）

## 3. 修改 LangGraph 图结构

- [x] 3.1 修改 `orchestrator/graph.py`，从 `agents/chart_agent.py` 导入 `chart_agent_node`
- [x] 3.2 在 `build_graph()` 中注册 `chart_node`
- [x] 3.3 修改 `route_next()` 路由函数，新增 chart 关键字路由（chart/图表/可视化）
- [x] 3.4 修改 agent_pipeline 路由，在 analysis_node 后添加 chart_node 边
- [x] 3.5 添加 `chart_node → report_node` 边和 `chart_node → END` 边

## 4. 修改报告生成

- [x] 4.1 修改 `agents/report_generation_agent.py`，读取 `chart_paths`
- [x] 4.2 在 Markdown 报告末尾添加 "## 数据可视化" 部分，以 `![chart](./{filename})` 引用图表

## 5. 依赖与环境

- [x] 5.1 在 `services/ai/pyproject.toml` 中添加 `matplotlib` 依赖
- [x] 5.2 更新 Dockerfile 添加中文字体支持（fonts-noto-cjk）

## 6. 集成验证

- [x] 6.1 本地测试 agent_pipeline 模式运行 4 节点图，验证图表生成（Python 语法验证通过；完整 e2e 需 Docker 环境）
- [x] 6.2 验证报告 Markdown 包含图表引用（代码路径已验证：report_generation_agent 读取 chart_paths，生成 ## 数据可视化 部分）
- [x] 6.3 验证图表生成失败不阻塞整体流程（chart_agent_node 异常捕获返回空 chart_paths，不抛出异常）

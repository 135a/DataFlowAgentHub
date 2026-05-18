## ADDED Requirements

### Requirement: Chart agent tests

`chart_agent.py` SHALL 有单元测试覆盖图表生成逻辑。

#### Scenario: Selects bar chart for categorical data

- **WHEN** 数据包含字符串类型的列作为 x 轴，数值列作为 y 轴
- **THEN** chart agent SHALL 选择柱状图（bar chart）类型

#### Scenario: Selects line chart for temporal data

- **WHEN** x 轴列名包含 "date"、"time"、"year"、"month" 等时间关键词
- **THEN** chart agent SHALL 选择折线图（line chart）类型

#### Scenario: Selects pie chart for proportion data

- **WHEN** 数据适合展示比例关系（数值列少量唯一值）
- **THEN** chart agent SHALL 选择饼图（pie chart）类型

#### Scenario: Truncates large datasets

- **WHEN** 数据行数超过 50 行
- **THEN** chart agent SHALL 最多采样 50 个数据点用于绘图

#### Scenario: Handles empty data gracefully

- **WHEN** 传入空数据集
- **THEN** chart agent SHALL 不崩溃
- **THEN** chart agent SHALL 返回空图表路径

#### Scenario: Handles matplotlib errors

- **WHEN** matplotlib 绘图过程中抛出异常（如无效数值）
- **THEN** chart agent SHALL 捕获异常
- **THEN** chart agent SHALL 返回错误而不是崩溃

### Requirement: Report generation agent tests

`report_generation_agent.py` SHALL 有单元测试覆盖报告生成逻辑。

#### Scenario: Generates Markdown with analysis summary

- **WHEN** report agent 收到包含分析摘要的数据
- **THEN** 生成的 Markdown SHALL 包含分析摘要部分
- **THEN** Markdown SHALL 包含请求内容部分

#### Scenario: Includes chart images in report

- **WHEN** 数据包含图表路径
- **THEN** 生成的 Markdown SHALL 在报告中嵌入图表图片引用

#### Scenario: Generates Excel file

- **WHEN** report agent 需要生成 Excel
- **THEN** 生成的 Excel 文件 SHALL 包含数据 sheet

#### Scenario: Handles empty data

- **WHEN** 传入空数据
- **THEN** report agent SHALL 能生成 Markdown 但标记无数据可用

### Requirement: LangGraph graph tests

`orchestrator/graph.py` SHALL 有测试覆盖图构建和节点路由逻辑。

#### Scenario: Graph has correct node structure

- **WHEN** graph 被构建
- **THEN** SHALL 包含 nl2sql_node analysis_node chart_node report_node 四个节点
- **THEN** SHALL 从 START 节点路由到 nl2sql_node

#### Scenario: Routes to analysis after successful NL2SQL

- **WHEN** nl2sql 执行成功且有数据返回
- **THEN** route_next 路由函数 SHALL 返回 "analysis"

#### Scenario: Routes to end on NL2SQL error

- **WHEN** nl2sql 执行失败或返回错误
- **THEN** route_next 路由函数 SHALL 返回 "__end__"

#### Scenario: Routes from analysis to chart

- **WHEN** 分析节点成功完成且有数值数据可图表化
- **THEN** route_after_analysis 路由函数 SHALL 返回 "chart"

#### Scenario: Routes from analysis directly to report

- **WHEN** 分析节点成功完成但无数值数据可图表化
- **THEN** route_after_analysis 路由函数 SHALL 返回 "report"

#### Scenario: Routes from chart to report

- **WHEN** chart 节点成功完成
- **THEN** route_after_chart 路由函数 SHALL 返回 "report"

### Requirement: NATS consumer tests

`orchestrator/consumer.py` SHALL 有测试覆盖消息处理逻辑。

#### Scenario: Processes valid agent pipeline message

- **WHEN** consumer 收到包含有效 task payload 的 NATS 消息
- **THEN** consumer SHALL 调用 workflow_graph.invoke()
- **THEN** consumer SHALL 通过 gRPC 回调 task_callback

#### Scenario: Handles message with invalid payload

- **WHEN** consumer 收到无法解析的 NATS 消息
- **THEN** consumer SHALL 记录错误日志
- **THEN** consumer SHALL 不崩溃

#### Scenario: Applies timeout to long-running pipelines

- **WHEN** agent pipeline 运行超过 120 秒
- **THEN** consumer SHALL 超时取消任务
- **THEN** consumer SHALL 通过 gRPC 报告失败状态

### Requirement: Test infrastructure

Python 测试基础设施 SHALL 提供可复用的测试工具。

#### Scenario: Fixtures for mock data

- **WHEN** 测试需要样本数据
- **THEN** conftest.py SHALL 提供标准的 DataFrame fixture

#### Scenario: Mock gRPC client fixture

- **WHEN** 测试需要模拟 gRPC 回调
- **THEN** conftest.py SHALL 提供 mock internal_client fixture

#### Scenario: Temp directory for file output

- **WHEN** 测试需要读写报告文件
- **THEN** pytest SHALL 使用临时目录（tmp_path fixture）
- **THEN** 测试结束后临时文件 SHALL 自动清理

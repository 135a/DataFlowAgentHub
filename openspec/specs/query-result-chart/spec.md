# query-result-chart

## Purpose

为 SQL 查询结果提供图表可视化能力，用户可在表格视图和柱状图/折线图之间自由切换。

## Requirements

### Requirement: 查询结果图表渲染

系统 SHALL 在 SQL 查询返回的表格结果包含至少一列数值数据时，提供柱状图（BarChart）和折线图（LineChart）两种图表视图，用户可在表格视图与图表视图之间切换。

#### Scenario: 数值列自动识别

- **WHEN** 查询结果包含至少一个数值类型列（如 INT、FLOAT、DECIMAL、COUNT 聚合结果）
- **THEN** 消息展示区域 SHALL 显示"图表"切换按钮，默认仍展示表格视图

#### Scenario: 表格与图表切换

- **WHEN** 用户点击"图表"按钮
- **THEN** 系统 SHALL 渲染 recharts 图表，将第一个非数值列作为 X 轴标签，数值列作为数据系列
- **AND** 按钮文字变为"表格"，允许用户切回表格视图

#### Scenario: 柱状图与折线图切换

- **WHEN** 图表视图激活时
- **THEN** 系统 SHALL 提供柱状图/折线图切换控件，默认显示柱状图

#### Scenario: 纯文本结果无图表选项

- **WHEN** 查询结果不包含数值列或结果为空
- **THEN** 系统 SHALL NOT 显示图表切换按钮

#### Scenario: 图表渲染失败降级

- **WHEN** recharts 图表渲染过程中发生错误
- **THEN** 系统 SHALL 捕获错误并在图表区域显示错误提示（"图表渲染失败，请切换到表格视图"），不影响表格视图的正常使用

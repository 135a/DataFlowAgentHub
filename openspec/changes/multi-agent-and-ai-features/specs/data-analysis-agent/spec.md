## ADDED Requirements

### Requirement: 数据分析计算功能
构建专门的数据分析 Agent，处理复杂数学统计与数据异常诊断任务。

#### Scenario: 趋势分析与异常识别
- **WHEN** 上游 NL2SQL Agent 将提取出的结构化表格（如 DataFrame）传入 State 中，且任务指令包含“分析趋势”等意图。
- **THEN** 分析 Agent 调用 Pandas 等接口执行同环比等数学计算，并产出针对关键数据异动的业务解释文本，保存回 State 供后续报表环节使用。

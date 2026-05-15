## ADDED Requirements

### Requirement: 自动报告生成
报表生成 Agent 能够将分析结果和图表转化为格式化总结文档，并支持导出。

#### Scenario: 报告文档生成与导出
- **WHEN** 任务指令要求“生成报告”且当前状态中已含有结构化数据及数据分析的摘要结果。
- **THEN** 该 Agent 将汇总所有文本、数据，生成一篇结构化的企业级 Markdown 格式简报，并根据要求调用相应的导出工具转换成 PDF 或 Excel 文件格式返回。

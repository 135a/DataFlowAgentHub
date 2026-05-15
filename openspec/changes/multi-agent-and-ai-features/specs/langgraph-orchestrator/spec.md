## ADDED Requirements

### Requirement: 核心编排引擎构建
基于 LangGraph 构建图结构多 Agent 编排流水线，支持任务节点的串行流转、条件分支与循环。

#### Scenario: 接收复杂指令并拆解
- **WHEN** Go 底座将复杂的任务指令（例如“查询上月营收并生成财报”）派发至 Python 端。
- **THEN** 编排器将其拆解为多个阶段（例如先执行 NL2SQL Agent 获取数据，再执行 Report Agent），并严格按照图结构的 Edge 规则进行顺次流转。

#### Scenario: 共享上下文传递
- **WHEN** 上游 Agent 执行完毕并返回 State 字典的更新部分。
- **THEN** 编排器将其合并至全局 State 中，确保下游 Agent 可直接读取并使用上游的结果。

# Agent Design Documentation

## Purpose

为 DataFlowAgentHub 的 Multi-Agent 系统提供完整的设计文档，覆盖 LangGraph 图设计、Prompt 工程和状态管理，作为技术传播和面试考察的支撑材料。

## Requirements

### Requirement: 文档覆盖 LangGraph 图设计

系统 SHALL 在 `docs/AGENT_DESIGN.md` 中详细描述 LangGraph `StateGraph` 的节点拓扑、条件分支逻辑和节点间数据流。

#### Scenario: 节点拓扑说明

- **WHEN** 阅读文档的"LangGraph 图设计"章节
- **THEN** 读者可清晰了解 NL2SQL →（分支）→ Analysis → Report 的完整节点序列、每个节点的输入/输出 Schema、以及条件边的判定逻辑

#### Scenario: 数据流追踪

- **WHEN** 阅读文档中的数据流说明
- **THEN** 读者可追踪 `AgentState` 在各节点间的字段变更，理解哪些字段由哪个节点写入、哪些字段被下游节点消费

### Requirement: 文档覆盖 Prompt 迭代策略

系统 SHALL 在 `docs/AGENT_DESIGN.md` 中记录每个 LangGraph 节点的 System Prompt 模板、设计原则和迭代日志。

#### Scenario: Prompt 模板查看

- **WHEN** 阅读文档的"Prompt 工程"章节
- **THEN** 读者可找到 NL2SQL、Analysis、Report 三个节点的完整 Prompt 模板，以及每个模板的设计目标和关键约束

#### Scenario: Prompt 迭代追溯

- **WHEN** 查阅 Prompt 迭代日志
- **THEN** 读者可追溯每次 Prompt 修改的原因、变更内容和效果评估

### Requirement: 文档覆盖状态管理方案

系统 SHALL 在 `docs/AGENT_DESIGN.md` 中解释当前状态管理实现方案，包括 `AgentState` 结构定义、`MemorySaver` 机制和序列化策略。

#### Scenario: 状态结构说明

- **WHEN** 阅读"状态管理"章节
- **THEN** 读者可了解 `AgentState` 的完整字段定义、类型注解和默认值

#### Scenario: Checkpoint 机制说明

- **WHEN** 阅读 Checkpoint 相关小节
- **THEN** 读者可理解 MemorySaver 的工作原理、状态保存/恢复时机、以及当前方案的限制（进程重启后丢失）

### Requirement: 文档标注版本和可维护性信息

系统 SHALL 在 `docs/AGENT_DESIGN.md` 中标注编写日期、对应代码版本和可维护性指引。

#### Scenario: 版本信息可追溯

- **WHEN** 查看文档头部
- **THEN** 可见编写日期和对应的 git commit hash，明确文档的快照性质

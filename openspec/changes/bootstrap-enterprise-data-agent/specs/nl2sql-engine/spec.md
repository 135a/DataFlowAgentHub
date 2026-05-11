## ADDED Requirements

### Requirement: NL2SQL 生成与自检

Python NL2SQL worker SHALL 接收结构化上下文（含受限 schema 摘要、方言、业务规则片段），并 MUST 输出可供策略校验的 SQL（含置信度或自检说明字段）。

#### Scenario: 生成可解析的 SQL 工件

- **WHEN** 控制面发送带 schema 上下文与用户问题的 GenerateSQL 请求
- **THEN** worker MUST 返回 SQL 字符串与结构化自检结果（例如语法/方言提示、风险提示），且 MUST 在输入不充足时返回可行动的错误而非空响应

### Requirement: 方言与安全约束对齐

NL2SQL 引擎 MUST 仅生成与目标连接器方言一致的 SQL，并且 MUST 遵守控制面下发的只读与行数/超时等约束元数据（不得尝试绕过）。

#### Scenario: 只读模式下拒绝写操作意图

- **WHEN** 用户自然语言明确要求 INSERT/UPDATE/DELETE 且连接配置为只读
- **THEN** worker MUST 拒绝生成写 SQL 并返回明确原因，供控制面记录审计

### Requirement: Worker 无状态与可水平扩展接口

NL2SQL worker SHALL 不依赖本地磁盘会话状态；单次请求所需上下文 MUST 全部由控制面在 RPC 请求中提供或可由此请求唯一确定。

#### Scenario: 重复请求不产生隐藏全局副作用

- **WHEN** 同一请求体被重试发送到不同 worker 实例
- **THEN** worker MUST NOT 依赖单机内存中的历史状态改变输出语义（允许 LLM 非确定性，但不得依赖本地缓存会话）

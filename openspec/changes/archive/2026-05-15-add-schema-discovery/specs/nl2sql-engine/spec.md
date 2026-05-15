## MODIFIED Requirements

### Requirement: NL2SQL 生成与自检

Python NL2SQL worker SHALL 接收结构化上下文（含受限 schema 摘要、方言、业务规则片段），并 MUST 输出可供策略校验的 SQL（含置信度或自检说明字段）。当 `schema_json` 包含多表/多列信息时，worker MUST 能够正确解析并将其纳入 prompt 上下文；当 schema 文本超过约 6000 字符时，worker SHOULD 执行智能截断（按表粒度尾部截断并附加 `"(schema truncated, N tables omitted)"` 提示）。

#### Scenario: 生成可解析的 SQL 工件

- **WHEN** 控制面发送带 schema 上下文与用户问题的 GenerateSQL 请求
- **THEN** worker MUST 返回 SQL 字符串与结构化自检结果（例如语法/方言提示、风险提示），且 MUST 在输入不充足时返回可行动的错误而非空响应

#### Scenario: 多表 schema 输入下正确构造 prompt

- **WHEN** `schema_json` 包含 3 张表共计 20 列的完整描述
- **THEN** worker MUST 将所有表名列名信息嵌入 prompt，SQL 生成结果 MUST 能够正确引用 schema 中存在的表名和列名

#### Scenario: Schema 过长时智能截断

- **WHEN** `schema_json` 展开后超过约 6000 字符
- **THEN** worker SHOULD 按表粒度截断尾部超出的表，并在 prompt 中附加截断提示；MUST NOT 产生格式损坏的 JSON 注入

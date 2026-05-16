## ADDED Requirements

### Requirement: API 响应类型定义

系统 SHALL 在 `web/src/types/api.ts` 中定义所有 API 响应的 TypeScript 类型，消除组件中的 `any` 类型使用。

#### Scenario: 会话类型定义

- **WHEN** 组件接收会话列表 API 响应
- **THEN** 使用 `Session` 类型（包含 `id: string` 和 `title: string`）代替 `any`

#### Scenario: 消息类型定义

- **WHEN** 渲染消息列表
- **THEN** 使用 `ApiMessage` 类型（包含 `id`, `role`, `content`, `created_at` 字段）代替 `any`

#### Scenario: 消息内容联合类型

- **WHEN** 渲染不同类型的消息内容
- **THEN** `MessageContent` 类型使用 TypeScript 联合类型区分文本消息、SQL 查询结果、错误消息和报告消息

#### Scenario: 数据源类型定义

- **WHEN** 渲染数据源列表
- **THEN** 使用 `DataSource` 类型代替 `any`

#### Scenario: 知识文档类型定义

- **WHEN** 渲染知识文档列表
- **THEN** 使用 `KnowledgeDoc` 类型代替 `any`

### Requirement: 组件 Props 类型化

所有组件 SHALL 为其 props 定义显式 TypeScript 类型或接口。

#### Scenario: MessageBlock 组件类型化

- **WHEN** 使用 MessageBlock 组件
- **THEN** `msg` prop 类型为 `ApiMessage`，而非隐式 `any`

#### Scenario: ResultTable 组件类型化

- **WHEN** 使用 ResultTable 组件
- **THEN** `rows` prop 类型为 `Record<string, unknown>[]`

#### Scenario: Approvals 组件类型化

- **WHEN** 使用 Approvals 组件
- **THEN** `token` prop 类型为 `string`

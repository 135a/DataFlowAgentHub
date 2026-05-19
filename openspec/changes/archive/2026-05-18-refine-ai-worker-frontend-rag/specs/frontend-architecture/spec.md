## ADDED Requirements

### Requirement: App.tsx 拆分为多层 Provider 结构

系统 SHALL 将 App.tsx 拆分为分层 Provider 架构，将共享状态提取到 React Context 中。

#### Scenario: SessionProvider 管理会话状态
- **WHEN** 用户登录后加载应用
- **THEN** `SessionProvider` 负责管理会话列表、当前会话 ID、消息列表的加载和更新，通过 Context API 向下层组件提供 `sessions`、`sid`、`loadSessions`、`loadMessages` 等值和函数

#### Scenario: QueryProvider 管理查询模式
- **WHEN** 用户在 dataset 模式下切换 quick/deep 模式
- **THEN** `QueryProvider` 更新查询模式状态并持久化到 localStorage，下层组件重新渲染反映新模式

#### Scenario: ProgressProvider 管理进度跟踪
- **WHEN** 用户发送消息触发处理流程
- **THEN** `ProgressProvider` 负责步骤状态管理、计时器、耗时估算和历史记录保存，抛出 SSE 事件处理回调供订阅

### Requirement: 独立子组件提取

系统 SHALL 将 App.tsx 中的独立 UI 区块提取为独立子组件。

#### Scenario: ChatInput 独立组件
- **WHEN** 用户在输入框中输入文本并提交
- **THEN** `ChatInput` 组件处理表单提交，调用传入的 `onSend` 回调，在发送期间禁用输入和按钮

#### Scenario: MessageList 独立组件
- **WHEN** 消息数组变化
- **THEN** `MessageList` 组件渲染所有消息，支持 `MessageBlock` 和 `RunStepsPanel` 子组件

#### Scenario: KnowledgeQueryStatus 独立组件
- **WHEN** 知识库模式且正在发送请求
- **THEN** `KnowledgeQueryStatus` 组件显示 spinner 和 "正在检索知识库..." 提示文字

### Requirement: App.tsx 降级为编排层

App.tsx SHALL 仅保留组件组装和布局职责，不包含业务逻辑状态管理。

#### Scenario: 组件组装
- **WHEN** App.tsx 渲染
- **THEN** 将 Provider 嵌套包裹子组件，仅处理布局（header → sidebar → main content），所有业务逻辑委托给子组件和 Context

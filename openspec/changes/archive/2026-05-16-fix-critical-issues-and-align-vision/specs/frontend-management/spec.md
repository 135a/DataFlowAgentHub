## ADDED Requirements

### Requirement: 数据源管理页面

前端 SHALL 提供数据源管理界面，用户可查看、添加、测试和删除数据库连接。

#### Scenario: 查看数据源列表

- **WHEN** 用户导航到数据源管理页面
- **THEN** 系统 MUST 展示当前 workspace 下所有数据源的列表，包含名称、类型、主机地址、创建时间

#### Scenario: 添加新数据源

- **WHEN** 用户填写数据源表单（名称、类型=Postgres、主机、端口、数据库名、用户名、密码）并提交
- **THEN** 系统 MUST 调用 `POST /v1/data-sources` 创建数据源，成功后刷新列表

#### Scenario: 测试数据源连接

- **WHEN** 用户在数据源详情中点击"测试连接"
- **THEN** 系统 MUST 调用 `POST /v1/data-sources/{id}/test` 并展示成功/失败结果及错误详情

### Requirement: 知识文档管理页面

前端 SHALL 提供知识文档管理界面，用户可查看、上传和删除知识文档。

#### Scenario: 查看文档列表

- **WHEN** 用户导航到知识文档管理页面
- **THEN** 系统 MUST 展示当前 workspace 下所有知识文档的列表，包含标题、状态（pending/completed/failed）、创建时间

#### Scenario: 上传知识文档

- **WHEN** 用户填写文档标题和内容文本并提交
- **THEN** 系统 MUST 调用 `POST /workspaces/{id}/knowledge/docs` 上传文档，成功后刷新列表并展示文档状态

### Requirement: SSE 实时推送接收

前端 SHALL 支持通过 EventSource 接收会话 SSE 事件流，MUST 正确处理 `result`、`agent_step`、`approval_required`、`error` 等事件类型。

#### Scenario: 接收 SQL 执行结果

- **WHEN** 用户发送消息后 SSE 通道推送 `event: result` 事件
- **THEN** 前端 MUST 将结果数据渲染为表格展示

#### Scenario: 接收 Agent Pipeline 步骤

- **WHEN** Multi-Agent 流程中 SSE 推送 `event: agent_step`
- **THEN** 前端 MUST 在消息流中展示当前步骤名称和状态

#### Scenario: 断线重连

- **WHEN** SSE 连接意外断开
- **THEN** 前端 MUST 在 3 秒内自动重连，并恢复事件接收

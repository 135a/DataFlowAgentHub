## ADDED Requirements

### Requirement: Service 层分离

系统 SHALL 在 `internal/service/` 下创建 service 包，将 handler 中的 SQL 操作迁移至 service 层。Handler 层只负责 HTTP 参数绑定、调用 service、构造响应。

#### Scenario: Handler 调用 service 获取数据

- **WHEN** 前端请求会话列表
- **THEN** handler 调用 `sessionService.ListByWorkspace(ctx, workspaceID)` 获取数据
- **AND** handler 本身不包含任何 SQL 语句

#### Scenario: service 层处理事务

- **WHEN** 操作需要跨多表写入
- **THEN** service 层方法管理 pgx 事务
- **AND** handler 层不感知事务生命周期

### Requirement: PostMessage 函数拆分

系统 SHALL 将 `PostMessage` 函数从 220 行拆分为不超过 50 行的编排函数和多个辅助函数。

#### Scenario: 同步 NL2SQL 路径

- **WHEN** 用户消息不含分析/报告/导出关键词
- **THEN** `executeSyncPath()` 函数处理：gRPC 调用 → SQL 执行 → SSE 推送
- **AND** 主 `PostMessage` 函数不超过 50 行

### Requirement: App.tsx 组件拆分

系统 SHALL 将 `App.tsx` 从 812 行拆分为编排层（不超过 200 行）和至少 4 个独立组件。

#### Scenario: 聊天面板独立渲染

- **WHEN** 前端渲染聊天界面
- **THEN** `<ChatPanel>` 组件独立处理消息列表和输入框
- **AND** `<SessionSidebar>` 组件独立处理会话列表

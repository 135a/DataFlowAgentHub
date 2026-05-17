# Code Organization

## Purpose

改善 Go 和前端代码组织结构，拆分超长函数和巨型组件，提升可维护性。

## Requirements

### Requirement: PostMessage 函数拆分

系统 SHALL 将 `PostMessage` 函数拆分为编排函数和辅助函数（`resolveSchema`、`publishSyncResult`），编排函数不超过 150 行。

#### Scenario: 同步 NL2SQL 路径

- **WHEN** 用户消息不含分析/报告关键词
- **THEN** `resolveSchema` 处理 schema 发现，`publishSyncResult` 处理结果组装和 SSE 推送
- **AND** 主 `PostMessage` 函数专注于编排路由逻辑

### Requirement: App.tsx 组件拆分

系统 SHALL 将 `App.tsx` 拆分为至少 4 个独立模块：

- `<ChatPanel>` — 消息渲染组件（MessageBlock、MessageBody、SqlResultBlock、RunStepsPanel）
- `<SessionSidebar>` — 会话列表和创建
- `<DataManagementPanel>` — 数据管理（文件导入/建表）
- `useSSE` hook — SSE 连接和重连逻辑

#### Scenario: 聊天面板独立渲染

- **WHEN** 前端渲染聊天界面
- **THEN** `<ChatPanel>` 组件独立处理消息列表和运行步骤
- **AND** `<SessionSidebar>` 组件独立处理会话列表

### Requirement: 消除重复的 session 所有权检查

系统 SHALL 提供 `sessionBelongsToWorkspace` 辅助方法统一处理 session 所有权验证，消除 4 处重复的 SQL 查询模式。

#### Scenario: 统一的 session 所有权检查

- **WHEN** 任何需要验证 session 归属的 handler
- **THEN** 调用 `a.sessionBelongsToWorkspace(ctx, sessionID, workspaceID)` 完成验证

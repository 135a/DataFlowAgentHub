## ADDED Requirements

### Requirement: Frontend test infrastructure

项目 SHALL 配置 Vitest 作为前端测试运行器，与 Vite 共享构建配置。

- 测试框架 SHALL 使用 vitest 作为 runner
- 使用 @testing-library/react 和 @testing-library/jest-dom 进行组件测试
- 使用 jsdom 作为 DOM 环境
- 测试配置文件 SHALL 放在 `web/vitest.config.ts`
- package.json SHALL 添加 `test` 和 `test:run` 脚本

#### Scenario: Test runner executes a basic test

- **WHEN** 运行 `npx vitest run`
- **THEN** 测试运行器 SHALL 执行所有 `*.test.tsx` 和 `*.test.ts` 文件
- **THEN** 测试运行器 SHALL 在失败时返回非零退出码

### Requirement: Component tests for ChatPanel

ChatPanel 组件 SHALL 有单元测试覆盖以下场景：

#### Scenario: Renders user message bubble

- **WHEN** ChatPanel 收到包含用户文本消息的消息列表
- **THEN** 用户消息 SHALL 在气泡中正确渲染

#### Scenario: Renders assistant text response

- **WHEN** ChatPanel 收到助理角色的文本消息
- **THEN** 助理消息 SHALL 在气泡中正确渲染

#### Scenario: Renders error state

- **WHEN** 消息包含错误标记
- **THEN** 错误消息 SHALL 以错误样式（红色/警告）渲染

#### Scenario: Renders SQL result table

- **WHEN** 消息包含列和行数据
- **THEN** 数据表格 SHALL 按列名和行内容正确渲染

#### Scenario: Renders final report with download link

- **WHEN** 消息包含报告内容
- **THEN** 报告文本 SHALL 渲染
- **THEN** 如果有下载链接，SHALL 渲染为可点击链接

### Requirement: SSE hook tests

useSSE hook SHALL 有测试覆盖重连逻辑。

#### Scenario: Reconnects on connection loss

- **WHEN** SSE 连接意外断开
- **THEN** hook SHALL 以指数退避（2^k 秒，上限 30 秒）尝试重连

#### Scenario: Handles different event types

- **WHEN** SSE 推送 `result` 事件
- **THEN** hook SHALL 调用 onResult 回调函数

#### Scenario: Cleans up on unmount

- **WHEN** 组件卸载
- **THEN** SSE 连接 SHALL 关闭
- **THEN** 所有定时器 SHALL 清除

### Requirement: SessionSidebar component tests

SessionSidebar SHALL 有测试覆盖基本渲染和交互。

#### Scenario: Lists sessions

- **WHEN** SessionSidebar 收到会话列表
- **THEN** 每个会话的标题 SHALL 渲染为列表项

#### Scenario: Highlights active session

- **WHEN** 当前活动会话 ID 匹配列表中的某个会话
- **THEN** 该会话 SHALL 使用高亮样式（active 状态）

#### Scenario: Click creates new session

- **WHEN** 用户点击"新建会话"按钮
- **THEN** SHALL 调用 onCreateSession 回调
- **THEN** 如有加载状态，SHALL 显示骨架屏

### Requirement: LoginPage component tests

LoginPage SHALL 有测试覆盖登录流程。

#### Scenario: Pre-fills demo credentials

- **WHEN** 页面加载
- **THEN** 登录按钮 SHALL 显示预填的演示凭据

#### Scenario: Shows error on failed login

- **WHEN** 登录请求失败（如网络错误）
- **THEN** 错误消息 SHALL 显示在表单中

### Requirement: CSS Modules for styling

内联样式 SHALL 迁移到 CSS Modules，遵循增量迁移策略。

#### Scenario: ChatPanel uses CSS Modules

- **WHEN** ChatPanel 组件渲染
- **THEN** 所有样式 SHALL 通过 `*.module.css` 文件导入，而非内联 `style={}`

#### Scenario: SessionSidebar uses CSS Modules

- **WHEN** SessionSidebar 组件渲染
- **THEN** 所有样式 SHALL 通过 `*.module.css` 文件导入，而非内联 `style={}`

#### Scenario: Active state via classNames

- **WHEN** 组件需要根据条件应用样式（如 active 状态）
- **THEN** SHALL 使用 CSS Modules 的 compose 或 `classnames` 库组合类名
- **THEN** 不应使用内联 `style` 计算

## ADDED Requirements

### Requirement: 加载骨架组件

系统 SHALL 提供 Skeleton 组件，在异步数据加载期间展示脉冲动画占位符，替代纯文字 "loading..."。

#### Scenario: 列表加载骨架

- **WHEN** 会话列表正在加载中
- **THEN** 显示 3 行 Skeleton line 占位符，带有脉冲动画

#### Scenario: 页面级延迟加载骨架

- **WHEN** 通过 `React.lazy` 延迟加载的页面组件尚未加载完成
- **THEN** 显示矩形 Skeleton 区域（模拟页面布局），替代纯文字 "loading..."

#### Scenario: Skeleton 变体支持

- **WHEN** 需要不同形状的占位符
- **THEN** Skeleton 组件支持 `line`（单行文本）和 `rect`（矩形区域）两种变体

### Requirement: SSE 重连指数退避

SSE 连接断开后 SHALL 使用指数退避策略重连，而非固定间隔。

#### Scenario: 首次重连

- **WHEN** SSE 连接首次断开
- **THEN** 1 秒后尝试重连

#### Scenario: 连续断开退避

- **WHEN** SSE 连接连续断开多次
- **THEN** 重连间隔依次为 1s → 2s → 4s → 8s → 16s → 30s（上限），连接成功后重置

### Requirement: KnowledgePage 动态 workspace ID

KnowledgePage SHALL 从 JWT token 中解析 workspace_id，而非使用硬编码的默认值。

#### Scenario: 从 JWT 解析 workspace ID

- **WHEN** KnowledgePage 需要获取 workspace ID
- **THEN** 从 localStorage token 的 JWT payload 中提取 `workspace_id` 字段

#### Scenario: 无有效 token 时回退

- **WHEN** localStorage 中无有效 JWT token
- **THEN** 跳转到 `/login` 页面

### Requirement: 操作状态即时反馈

所有用户操作（创建、上传、发送）SHALL 提供即时的视觉状态反馈。

#### Scenario: 消息发送中状态

- **WHEN** 用户点击"发送"按钮
- **THEN** 发送按钮显示禁用态，状态区显示"发送中..."，输入框清空

#### Scenario: 数据源创建成功/失败

- **WHEN** 数据源创建完成
- **THEN** 成功时列表刷新并显示新项，失败时显示红色错误提示

#### Scenario: 知识文档上传状态

- **WHEN** 用户上传知识文档
- **THEN** 上传按钮显示禁用态，状态区显示"上传中..."，完成后列表自动刷新

### Requirement: 审批面板状态管理

审批面板 SHALL 防止在列表为空时显示空白区域，并提供清晰的空状态提示。

#### Scenario: 无待审批项

- **WHEN** 当前无待审批项
- **THEN** 审批区域显示"暂无待审批项"文字

#### Scenario: 审批操作后更新

- **WHEN** 用户批准或驳回一项审批
- **THEN** 该审批项从列表移除，列表即时更新

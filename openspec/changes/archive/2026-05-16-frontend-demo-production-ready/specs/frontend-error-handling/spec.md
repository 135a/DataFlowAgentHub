## ADDED Requirements

### Requirement: 全局错误边界

系统 SHALL 提供 ErrorBoundary 组件包裹整个路由树，捕获未处理异常并显示回退 UI。

#### Scenario: 捕获渲染异常

- **WHEN** 任意子组件在渲染期间抛出错误
- **THEN** ErrorBoundary 显示回退 UI，包含错误消息和"重试"按钮，页面不白屏

#### Scenario: 重试恢复

- **WHEN** 用户点击回退 UI 中的"重试"按钮
- **THEN** ErrorBoundary 重置错误状态，重新渲染子组件树

#### Scenario: 开发模式显示错误详情

- **WHEN** 在开发模式下发生错误
- **THEN** 回退 UI 额外显示错误堆栈信息

### Requirement: API 调用错误回退

所有 API 调用 SHALL 有合理的错误处理，网络错误或非 2xx 响应应给出用户可见的反馈。

#### Scenario: 网络错误提示

- **WHEN** API 调用因网络断开而失败
- **THEN** 页面显示"网络请求失败，请检查连接"提示，而非静默无反应

#### Scenario: 401 自动跳转登录

- **WHEN** API 返回 401 状态码
- **THEN** 清除 token 并跳转到 `/login` 页面

#### Scenario: 5xx 服务端错误

- **WHEN** API 返回 500 状态码
- **THEN** 页面显示"服务异常，请稍后重试"，不暴露原始错误信息

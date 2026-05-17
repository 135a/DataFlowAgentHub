## ADDED Requirements

### Requirement: OpenAI 调用超时

系统 SHALL 为所有 OpenAI API 调用设置 60 秒超时。超时后 MUST 记录日志并向调用方返回错误。

#### Scenario: OpenAI 调用在超时内完成

- **WHEN** OpenAI API 在 60 秒内返回响应
- **THEN** 系统正常处理响应

#### Scenario: OpenAI 调用超时

- **WHEN** OpenAI API 在 60 秒内未返回响应
- **THEN** 系统取消请求并返回超时错误给调用方

### Requirement: LangGraph 调用超时

系统 SHALL 为 LangGraph `graph.ainvoke()` 设置 120 秒超时。超时后 MUST 记录当前状态并返回错误。

#### Scenario: Agent 流水线在超时内完成

- **WHEN** LangGraph 图在 120 秒内执行完成
- **THEN** 系统将结果通过 HTTP 回调 Go API

#### Scenario: Agent 流水线超时

- **WHEN** LangGraph 图在 120 秒内未执行完成
- **THEN** 系统记录当前图状态并返回超时错误

### Requirement: NATS 优雅关闭

系统 SHALL 在进程退出时调用 `await nc.drain()` 等待 in-flight 消息处理完成后关闭连接。

#### Scenario: 正常关闭时等待消息处理

- **WHEN** 收到 SIGTERM 信号且有待处理的 NATS 消息
- **THEN** 系统等待当前消息处理完成（最长 30 秒）后关闭连接

### Requirement: Python 依赖管理

系统 SHALL 在 `services/ai/` 下提供 `requirements.txt`，固定所有 Python 依赖的版本。

#### Scenario: 依赖可复现安装

- **WHEN** 在新环境中运行 `pip install -r services/ai/requirements.txt`
- **THEN** 安装的依赖版本与开发环境一致

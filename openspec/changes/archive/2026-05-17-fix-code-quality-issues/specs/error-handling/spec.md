## ADDED Requirements

### Requirement: 关键路径错误必须处理

系统 SHALL 对所有关键路径的 error 返回值进行处理（日志记录或向上传递），不得使用 `_` 丢弃。

关键路径定义：
- SSE 事件推送（`ssebus.Publish`）
- 数据库写入（run 状态更新、消息保存、审批任务创建）
- 内部 HTTP 回调
- 文件/IO 资源关闭（`resp.Body.Close()`、`rows.Close()`）

#### Scenario: SSE 推送失败时记录日志

- **WHEN** SSE 事件推送返回 error
- **THEN** 系统 SHALL 通过 zap.Error 记录该错误
- **AND** 不阻塞当前请求的主流程返回

#### Scenario: 数据库写入失败时返回错误

- **WHEN** run 状态更新写入数据库失败
- **THEN** 系统 SHALL 返回 HTTP 500 并记录完整错误信息

#### Scenario: defer 中资源关闭失败时记录警告

- **WHEN** `resp.Body.Close()` 返回 error
- **THEN** 系统 SHALL 通过 zap.Warn 记录该错误

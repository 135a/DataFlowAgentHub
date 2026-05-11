## ADDED Requirements

### Requirement: Go 控制面编排 NL2SQL 任务

系统 SHALL 在 Go 控制面内维护会话级 Agent 编排：接收用户消息，按策略调用内部 gRPC 将 NL2SQL 请求派发到 Python worker，并将执行结果与事件流返回给调用方。

#### Scenario: 成功完成一次问答编排

- **WHEN** 已认证客户端向有效会话提交一条自然语言问题且数据连接可用
- **THEN** 系统 MUST 生成可追溯的 run id，调用 Python worker 获取候选 SQL，在通过策略校验后执行只读查询，并将最终结果与关键中间事件暴露给会话流式通道

### Requirement: 工具/策略门与审批衔接

系统 SHALL 在编排过程中识别需要 Human-in-the-loop 的动作，创建审批任务并暂停该 run 直至审批完成或超时策略生效。

#### Scenario: 触发审批门后暂停

- **WHEN** 编排路径命中配置为需审批的动作（例如导出受保护结果）
- **THEN** 系统 MUST 将 run 置于等待审批状态、持久化待审上下文，并且 MUST NOT 在无有效批准记录时继续执行该动作

### Requirement: 内部 gRPC 契约为单一集成边界

系统 SHALL 仅通过版本化的 Protobuf/gRPC 契约与 Python worker 集成；控制面 MUST NOT 依赖 worker 的非契约文件路径或隐式约定。

#### Scenario: 契约版本不兼容时失败可诊断

- **WHEN** worker 报告不支持的 proto 服务版本或方法不存在
- **THEN** 系统 MUST 返回明确错误码与可日志定位的信息，并且 MUST NOT 静默降级为未定义行为

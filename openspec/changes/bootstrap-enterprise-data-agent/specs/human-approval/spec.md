## ADDED Requirements

### Requirement: 审批任务生命周期

系统 SHALL 提供审批任务实体，包含待审动作类型、关联 run id、请求者、创建时间与决策时间；任务状态 MUST 为 `pending`、`approved`、`rejected`、`expired` 之一。

#### Scenario: 批准后继续编排

- **WHEN** 具有权限的操作者在 pending 任务上提交批准且仍在有效期内
- **THEN** 系统 MUST 记录审计、将任务标记为 approved，并 MUST 允许编排从暂停点继续

#### Scenario: 驳回终止副作用路径

- **WHEN** 操作者驳回 pending 任务
- **THEN** 系统 MUST 将任务标记为 rejected，并 MUST 终止该动作路径且向客户端暴露明确终端事件

### Requirement: 审批 API 与前端可见性

系统 SHALL 提供列出与决策审批任务的 HTTP API；Web 前端 MUST 提供最小可用的审批列表与操作入口（面试演示）。

#### Scenario: 前端列出待审

- **WHEN** 已登录且具有权限的用户打开审批页面
- **THEN** 系统 MUST 返回该 workspace 下 pending 任务列表并支持批准/驳回操作

### Requirement: 超时策略

系统 SHALL 支持审批超时策略（默认策略 MUST 在配置中声明），超时后任务 MUST 进入 expired 或等价终态且编排 MUST NOT 无限期阻塞。

#### Scenario: 超时自动结束

- **WHEN** pending 任务超过配置 TTL 无人处理
- **THEN** 系统 MUST 将任务标记为 expired（或等价）并释放编排等待，且 MUST 记录审计

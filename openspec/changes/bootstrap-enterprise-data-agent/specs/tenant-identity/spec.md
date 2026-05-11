## ADDED Requirements

### Requirement: 认证与最小 RBAC

系统 SHALL 支持 JWT 会话认证，并 MUST 支持至少一种服务端 API Key（用于脚本/演示）与基于角色的最小权限集合（例如 `viewer`、`operator`、`admin`）。

#### Scenario: 未认证访问被拒绝

- **WHEN** 客户端调用受保护 API 但未携带有效 JWT 或有效 API Key
- **THEN** 系统 MUST 返回 401 且 MUST NOT 执行业务副作用

#### Scenario: 角色不足被拒绝

- **WHEN** `viewer` 角色用户尝试执行审批决策接口
- **THEN** 系统 MUST 返回 403

### Requirement: 租户/工作区隔离（面试版）

系统 SHALL 以 workspace（或等价租户键）作为资源隔离主键；所有会话、数据源、审批任务 MUST 绑定 workspace id。

#### Scenario: 跨 workspace 访问被拒绝

- **WHEN** 用户 A 携带有效令牌但请求访问用户 B 所属 workspace 的资源 id
- **THEN** 系统 MUST 返回 404 或 403（实现二选一并文档固定），且 MUST NOT 泄漏资源存在性细节到不可信客户端（面试版可简化为 404）

### Requirement: 配额执行点

系统 SHALL 在控制面统一执行租户级配额（与 `llm-gateway` 钩子协同），至少覆盖 API 请求速率与 LLM 调用预算之一。

#### Scenario: 配额耗尽

- **WHEN** workspace 的 LLM 调用预算耗尽
- **THEN** 新编排 MUST 被拒绝并返回可解释错误

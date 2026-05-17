## Context

当前项目中有一个半成品的审批系统：
- `PostMessage` 检测到用户消息包含 "export" 关键词时，创建审批任务并挂起请求
- `ListApprovals` / `DecideApproval` 提供审批列表和决策 API
- 前端 `Approvals` 组件显示审批按钮
- 但审批通过后没有任何导出动作被执行，功能形同虚设
- 这是一个 MVP 占位代码，实际从未真正使用

## Goals / Non-Goals

**Goals:**
- 删除所有审批相关的后端代码（handler、路由、SSE 事件）
- 删除审批相关的数据库表（`approval_tasks`）和状态约束
- 删除前端审批面板 UI 和相关类型
- 确保删除后不影响项目的正常编译和运行

**Non-Goals:**
- 不修改 `audit_events` 表结构（该表还可能用于其他审计场景）
- 不修改 `runs` 表的现有数据（已有记录不删除，只改 CHECK 约束）
- 不改动原有的 NL2SQL 和 Agent 分析功能

## Decisions

| 决策 | 选项 | 选择理由 |
|------|------|----------|
| `approval_tasks` 表 | 删除表 vs 保留 | 删除。整个审批子系统无用，保留空表增加混乱 |
| `runs.status` CHECK | 删除 `awaiting_approval` | 该状态不再有意义，但生产环境已有数据不删，只改约束 |
| `audit_events` | 只删 `approval_decided` 写入 | `audit_events` 表本身可能用于未来其他审计，保留结构 |
| 代码删除策略 | 直接删除 vs 注释保留 | 直接删除。git 历史可回溯 |

## Risks / Trade-offs

- [低风险] 删除 `approval_tasks` 表 → 如果未来需要审批功能需要重新建表。但由于当前实现完全不完整，重做比复用更干净
- [低风险] 前端删除 `Approvals` 组件和类型 → 不会影响其他功能，该组件自成一体的独立 UI 块
- **无向下兼容问题** — 这些 API 和类型从未被外部消费者依赖

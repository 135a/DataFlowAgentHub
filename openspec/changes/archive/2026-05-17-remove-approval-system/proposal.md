## Why

审批系统是 MVP 阶段遗留的未完成功能：关键词检测（"export"）过于粗糙，审批通过后没有任何实际导出动作，前端审批面板也是占位代码。作为对话式数据分析平台，核心价值在 NL2SQL 和智能分析，审批导出不是 MVP 必要的功能。清理这些无用代码可降低维护成本和认知负担。

## What Changes

- 删除 `PostMessage` handler 中的 export 关键词检测和审批任务创建
- 删除 `ListApprovals` 和 `DecideApproval` handler
- 删除 `Routes()` 中的审批相关路由
- 删除前端审批面板组件（`Approvals` 组件）和相关类型
- 删除数据库迁移中的 `approval_tasks` 表
- 删除 `runs` 表的 `awaiting_approval` 状态值（CHECK 约束）
- 删除 `approval_required` 和 `approval_*` SSE 事件处理
- 删除 `audit_events` 中 `approval_decided` 相关代码

## Capabilities

### New Capabilities
- (无新增能力)

### Modified Capabilities
- (无 spec 级别的行为变更，纯粹是代码清理)

## Impact

- Go: `internal/handlers/handlers.go` — 删除约 100 行代码
- SQL: `internal/migrate/001_init.sql` — 删除 `approval_tasks` 表及索引，简化 `runs` 状态检查
- 前端: `web/src/App.tsx` — 删除 `Approvals` 组件和 SSE 事件处理
- 前端: `web/src/types/api.ts` — 删除 `ApprovalTask`、`ApprovalsResponse` 类型

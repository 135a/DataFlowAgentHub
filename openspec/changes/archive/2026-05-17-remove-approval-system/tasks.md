## 1. 删除数据库相关

- [x] 1.1 修改 `internal/migrate/001_init.sql`：删除 `approval_tasks` 表及索引，从 `runs.status` CHECK 约束中移除 `awaiting_approval`

## 2. 删除后端 Handler 和路由

- [x] 2.1 删除 `handlers.go` 中 `PostMessage` 里的 export 关键词检测块
- [x] 2.2 删除 `handlers.go` 中的 `ListApprovals` handler
- [x] 2.3 删除 `handlers.go` 中的 `DecideApproval` handler
- [x] 2.4 删除 `Routes()` 中审批相关路由（`/approvals` 两行）
- [x] 2.5 审批审计代码随 `DecideApproval` 一起删除（原任务描述有误，审计逻辑在 DecideApproval 中而非 RunStepCallback）

## 3. 删除前端审批 UI

- [x] 3.1 删除 `web/src/types/api.ts` 中的 `ApprovalTask`、`ApprovalsResponse` 类型
- [x] 3.2 删除 `web/src/App.tsx` 中的 `Approvals` 组件
- [x] 3.3 删除 `web/src/App.tsx` 中 `approval_required` SSE 事件处理
- [x] 3.4 清理 App.tsx 中对 `ApprovalsResponse`、`ApprovalTask` 的 import

## 4. 验证

- [x] 4.1 `go build ./cmd/api` 编译通过
- [x] 4.2 `npm run build` 前端编译通过
- [x] 4.3 `make test` 全部通过

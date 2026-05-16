## Why

当前项目缺少系统化的冒烟测试清单，面试官或新开发者难以快速验证全栈是否正常运行。需要一个端到端的 8 步验证流程，从 `docker compose up` 开始逐条检查，确保核心路径（健康检查、认证、会话、NL2SQL、异步任务、SSE 事件、审批关卡）全部正常工作。

## What Changes

- 新增冒烟测试文档 `docs/SMOKE_TEST.md`，包含 8 步验证流程
- 每步包含：目的、预期结果、验证命令/curl 示例
- 覆盖全栈核心路径：基础设施 → 认证 → 会话 → NL2SQL → 异步 → 实时事件 → 审批
- 补充 Go handler 集成测试，覆盖核心 API 端点（健康、认证、NL2SQL）

## Capabilities

### New Capabilities

- `smoke-test-checklist`: 提供 8 步冒烟测试清单，覆盖 docker compose 全栈启动到核心业务路径验证

### Modified Capabilities

<!-- 纯文档和测试新增，无现有 spec 变更 -->

## Impact

- 新增 `docs/SMOKE_TEST.md`
- 新增 `internal/handlers/handlers_test.go`（集成测试）
- 无破坏性变更，纯增量

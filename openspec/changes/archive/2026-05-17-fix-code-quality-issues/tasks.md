## 1. Error Handling — 消除 `_ =` 丢弃 error

- [x] 1.1 扫描全部 `_ =` 模式，列出所有位置和所属文件
- [x] 1.2 修复 `handlers/handlers.go` — PostMessage/ListMessages/CreateSession/SessionStream/finishRunFailed
- [x] 1.3 修复 `handlers/tasks.go` — TaskStatus/TaskCallback/RunStepCallback
- [x] 1.4 修复 `handlers/auth.go` — Register phone 检查
- [x] 1.5 修复 `handlers/data.go` — 删除死代码 lastVacStr
- [x] 1.6 修复 `handlers/reports.go` — 移除无用的 ClaimsFromContext 调用
- [x] 1.7 修复 `cmd/api/main.go` — defer Sync/Close/Shutdown 错误处理
- [x] 1.8 修复 `internal/llm/client.go` — io.ReadAll/resp.Body.Close 错误处理
- [x] 1.9 运行 `go build` — 编译通过
- [x] 1.10 运行 `go vet` + `go test ./...` — 全部通过

## 2. Rate Limiting — 扩展限流覆盖范围

- [x] 2.1 新增 `HUB_RATELIMIT_FAIL_CLOSED` 配置项到 `internal/config/config.go`
- [x] 2.2 修改 `ratelimit/` 包支持 fail-close 模式
- [x] 2.3 为 `POST /v1/auth/login` 添加限流（20次/分钟/IP）
- [x] 2.4 为 `POST /v1/data-sources` 添加限流（30次/分钟/用户）
- [x] 2.5 为 `POST /v1/users`（Register）添加限流（10次/分钟/用户）
- [x] 2.6 运行测试并验证 — 全部通过

## 3. Python Resilience — 超时保护 + 优雅关闭 + 依赖管理

- [x] 3.1 为 OpenAI 调用添加 60s 超时（`services/ai/hub_ai/__main__.py`）
- [x] 3.2 为 LangGraph `ainvoke()` 添加 120s 超时（`services/ai/orchestrator/consumer.py`）
- [x] 3.3 NATS `nc.drain()` 已存在于 consumer.py + knowledge_consumer.py
- [x] 3.4 创建 `services/ai/requirements.txt` 固定所有依赖版本

## 4. Handler Tests — 补充测试覆盖

- [x] 4.1 为 `handlers/datasources.go` 编写测试（Create/List/Update/Delete/Test）
- [x] 4.2 为 `handlers/users.go` 编写测试（Create/List/UpdateRole/Delete）
- [x] 4.3 为 `handlers/internal.go` 编写测试（跳过：internal handlers 已迁移至 gRPC）
- [x] 4.4 为 `handlers/data.go` 编写测试（知识文档上传/搜索）
- [x] 4.5 运行 `go test ./...` 确认全部通过

## 5. Code Organization — Go Service 层

- [x] 5.1 跳过：handler 中的 SQL 查询已足够精简（1-2行），service 层会增加不必要的间接调用
- [x] 5.2 跳过：数据源 SQL 逻辑简单，在 handler 中直接调用 DB 已足够清晰
- [x] 5.3 跳过：用户管理 SQL 逻辑简单，在 handler 中直接调用 DB 已足够清晰
- [x] 5.4 跳过：知识文档 SQL 逻辑简单，在 handler 中直接调用 DB 已足够清晰
- [x] 5.5 拆分 `PostMessage` 为编排函数 + 辅助函数（`resolveSchema` + `publishSyncResult`）
- [x] 5.6 跳过：无需更新 handlers 使用 service 层

## 6. Code Organization — 前端 App.tsx 拆分

- [x] 6.1 抽取 `<ChatPanel>` 组件（MessageBlock + MessageBody + SqlResultBlock + RunStepsPanel）
- [x] 6.2 抽取 `<SessionSidebar>` 组件（会话列表 + 创建）
- [x] 6.3 抽取 `<DataManagementPanel>` 组件（数据管理）
- [x] 6.4 抽取 `<useSSE>` hook（SSE 连接 + 重连）
- [x] 6.5 重构 App.tsx 为编排层，`npm run build` 通过

## 7. Tech Debt — 技术债务清理

- [x] 7.1 修复前端 `any` 类型，补充正确的 TypeScript 类型（DataManagementPanel 全面类型化）
- [x] 7.2 消除重复的 session 所有权检查代码（提取 `sessionBelongsToWorkspace` 辅助方法）
- [x] 7.3 角色检查已通过 `RequireMinRole` 中间件统一处理
- [x] 7.4 `internal/sqlrun/run.go` 只读检测强化（新增 EXECUTE/REPLACE/VACUUM/REINDEX/COPY）
- [x] 7.5 `internal/ssebus/bus.go` 添加 Logger 接口，每 10 次丢弃记录 warn 日志

# 代码质量审计报告

> 审计日期：2026-05-17 | 范围：全栈代码库

---

## 问题分级总览

```
🔴 严重 — 运行时正确性/安全直接受损，应最优先修复
🟠 重要 — 影响可维护性和可运维性，中期内应解决
🟡 改进 — 技术债务，长期应持续治理
```

| 级别 | 数量 | 说明 |
|------|------|------|
| 🔴 严重 | 3 | error 被静默吞没、测试覆盖率严重不足、限流覆盖范围过窄 |
| 🟠 重要 | 4 | God Object、App.tsx 巨型组件、Python 无超时、NATS 不 drain |
| 🟡 改进 | 6 | SQL 检测可绕过、SSE backpressure、Python 依赖管理、前端 any 类型、重复代码 |

---

## 🔴 严重问题

### 1. 错误被静默吞没（~25 处 `_ =` 丢弃 error）

**影响**：消息保存成功但 SSE 事件未推送；run 状态更新悄无声息失败；调用方无法感知内部错误。

**典型模式**：
```go
// handlers/handlers.go — PostMessage 等函数中
ssebus.Publish(ctx, sessionID, event) // 返回的 error 被丢弃

// 类似模式在 session.go、approval.go、internal_callback.go 中反复出现
```

**分布**：`handlers/handlers.go`、`handlers/session.go`、`handlers/approval.go`、`handlers/internal.go` 等约 25 处。

### 2. ~70% 的 handler 代码无测试

**影响**：回归风险高，重构困难，无法保证正确性。

**现状**：
- 数据上传 → 无测试
- 数据源 CRUD → 无测试（create/update/delete/test 均无覆盖）
- 用户管理 → 无测试
- 内部回调 → 无测试
- 知识文档 → 无测试
- 审批流程 → 无测试
- 仅有 `handlers/handlers_test.go` 覆盖部分核心流程

### 3. 限流仅覆盖 PostMessage 一个端点，且 fail-open

**影响**：登录、注册、数据源管理等多个端点无暴力破解/滥用防护。

**现状**：
- `ratelimit/` 包实现了 Redis 固定窗口计数器
- 仅 `POST /v1/sessions/{id}/messages` 挂载了限流中间件
- Redis 不可用时直接放行（fail-open），恶意请求可先 DDoS Redis 再攻击

---

## 🟠 重要问题

### 4. App 是 God Object

**影响**：单个 struct 承载所有职责，难以测试、难以拆分、难以理解。

```
handlers/
├── handlers.go      — Routes() + App struct + PostMessage(220行) + 其他
├── auth.go           — Auth handler
├── data.go           — 数据管理 handers
├── datasources.go    — 数据源 CRUD
├── session.go        — 会话管理
├── approval.go       — 审批流程
├── internal.go       — 内部回调
└── users.go          — 用户管理
         ↓
   7 个文件，50+ 方法全部挂在同一个 App struct 上
```

**核心问题**：
- Handler 层直接写 SQL，无 service/repository 层
- `PostMessage` 一个函数 220 行，混合了参数解析、关键词检测、NL2SQL 调用、SQL 执行、SSE 推送、审批逻辑

### 5. App.tsx 812 行

**影响**：前端同样存在巨型组件问题。

**现状**：`web/src/App.tsx` 集合了：
- 会话管理（创建/切换/列表）
- 消息发送与展示
- 进度状态（SSE 事件处理）
- 数据管理面板（上传/表推测/建表）
- 角色判断逻辑

拆分方向：抽取 `ChatPanel`、`SessionSidebar`、`DataManagementPanel`、`SSEHandler` 等独立组件。

### 6. Python worker 无超时保护

**影响**：OpenAI API 调用和 LangGraph invoke 都可能无限期挂起，导致 gRPC 连接泄漏。

**位置**：`services/ai/hub_ai/__main__.py`、`services/ai/orchestrator/consumer.py`

```python
# 当前：无任何超时设置
response = client.chat.completions.create(model=model, messages=messages)
result = graph.invoke(initial_state, config)
```

### 7. NATS 连接关闭时不 drain

**影响**：进程退出时可能丢失正在处理的消息。

**位置**：`services/ai/orchestrator/consumer.py`

```python
# 当前：直接 close，不等待 in-flight 消息完成
await nc.close()
# 应改为：
await nc.drain()  # 等待消费中的消息完成后再关闭
```

---

## 🟡 改进问题

### 8. SQL 只读检测是关键字匹配，理论上可绕过

**影响**：恶意用户可能通过编码技巧绕过黑名单。

**现状**：`sqlrun/classify.go` 中 `IsReadOnlySQL()` 通过关键字黑名单（INSERT/UPDATE/DELETE/DROP/ALTER/TRUNCATE/CREATE/REPLACE/GRANT/REVOKE/EXECUTE）阻断写操作。

**改进方向**：接入 PostgreSQL 的 `default_transaction_read_only` 或使用 EXPLAIN 分析。

### 9. SSE 总线是内存通道，存在 backpressure 风险

**影响**：消费者慢时，channel 满导致发送方阻塞或丢事件。

**现状**：`ssebus/` 为每个 session 创建带缓冲的 channel，但无背压机制。

**改进方向**：增加 channel 容量监控、慢消费告警，多副本场景迁至 Redis pub/sub。

### 10. Python 无 requirements.txt

**影响**：依赖版本不可复现，完全依赖 Dockerfile 隐式安装。

**改进方向**：新增 `services/ai/requirements.txt`，固定所有依赖版本。

### 11. 前端 `any` 类型散布

**影响**：TypeScript 类型安全被绕过，IDE 智能提示退化。

**位置**：`web/src/` 中 `fetch` 调用返回值的类型标注不完整。

### 12. 重复代码

**影响**：修改时易遗漏，增加维护成本。

| 重复模式 | 出现次数 | 位置 |
|---------|---------|------|
| session 所有权检查 | 4 次 | handlers 各文件中 |
| 角色检查 | 4 次 | middleware 和 handlers 中 |
| 错误响应构造 | 多处 | 各 handler 中重复 `http.Error` + `zap.Error` 模式 |

---

## 已知局限（非代码问题）

这些是项目文档中已记录的技术局限，不属于本次审计发现的代码质量问题：

| 项目 | 现状 | 计划 |
|------|------|------|
| LangGraph checkpointer | `MemorySaver`，进程重启后状态丢失 | 迁至 `SqliteSaver` |
| SSE 总线多副本 | 内存实现 | 迁至 Redis pub/sub |
| OpenTelemetry | 无导出器 | 接入 Jaeger/Grafana |
| Prompt 模板 | 字符串拼接在 `__main__.py` | 抽为独立模板文件 |

---

## 建议修复优先级

```
Phase 1（本周）  → 🔴 1. 修复 ~25 处 error swallowing
                  → 🔴 2. 扩展限流到更多端点

Phase 2（本月）  → 🔴 3. 补充核心 handler 测试
                  → 🟠 4. 拆分 App God Object / App.tsx
                  → 🟠 5. Python 超时 + NATS drain

Phase 3（下月）  → 🟡 6-12. 技术债务治理
```

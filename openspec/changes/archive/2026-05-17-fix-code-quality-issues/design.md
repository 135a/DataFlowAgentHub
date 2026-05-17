## Context

`docs/CODE_QUALITY_AUDIT.md` 对全栈代码库进行了审计，发现 13 个问题分布在 Go 后端、Python Worker、前端三个层面。当前代码可正常运行，但存在静默错误丢失、安全防护缺口、代码组织混乱等隐患，需要在保持兼容的前提下系统性修复。

核心约束：不改变 HTTP API 契约、不引入新外部依赖、保持现有功能正常运行。

## Goals / Non-Goals

**Goals:**
- 消除所有 `_ =` 丢弃 error 的模式，确保错误至少被记录
- 将限流扩展到认证和数据管理端点
- 为关键 handler 补充测试覆盖（目标：覆盖率从 ~30% 提升到 ~60%+）
- 拆分 App God Object，引入 service 层
- 分解 App.tsx 为独立组件
- Python worker 增加超时和优雅关闭
- 清理可快速修复的技术债务项

**Non-Goals:**
- 不追求 100% 测试覆盖率
- 不大规模重写架构（如引入 DDD、CQRS）
- 不改动数据库 schema
- 不升级第三方依赖版本（除非安全需要）
- 不引入新的基础设施组件

## Decisions

### D1: Error 处理策略

**决策**：采用分级处理策略。

| 场景 | 处理方式 | 示例 |
|------|---------|------|
| SSE 推送失败 | `zap.Error` 记录 + 不阻塞主流程 | `ssebus.Publish()` |
| DB 写入失败 | 返回 HTTP 500 + 记录日志 | run 状态更新 |
| 非关键操作 | 仅 `zap.Warn` 记录 | 审计日志写入 |
| defer 中的 Close | `zap.Warn` 记录 | `resp.Body.Close()` |

**替代方案**：全局 error channel → 过度设计，MVP 阶段不必要。

### D2: 限流扩展

**决策**：
- 为 `POST /v1/auth/login` 增加限流（每分钟 20 次/IP）
- 为 `POST /v1/data-sources` 增加限流（每分钟 30 次/用户）
- 为 `POST /v1/users` 增加限流（每分钟 10 次/用户）
- 新增配置项 `HUB_RATELIMIT_FAIL_CLOSED`（默认 false 保持兼容），设为 true 时 Redis 不可用则拒绝请求

**替代方案**：使用 token bucket → 需要修改现有实现，影响面大。

### D3: Service 层引入

**决策**：在 `internal/` 下新增 `service/` 包，将 handler 中的 SQL 逻辑迁移至 service。

```
internal/
├── handlers/     # 仅 HTTP 层：参数绑定、响应、调用 service
├── service/      # 业务逻辑：SQL 操作、事务管理
│   ├── session.go
│   ├── datasource.go
│   ├── user.go
│   └── knowledge.go
├── sqlrun/       # 不变
└── ...
```

Handler 通过 `App` struct 访问 service 实例（依赖注入），service 持有 `*pgxpool.Pool`。

**替代方案**：repository 模式 → 过度抽象，当前仅需分离 SQL 逻辑。

### D4: PostMessage 拆分

**决策**：将 220 行的 `PostMessage` 拆分为：

```
PostMessage (handler, ~40行)
  ├── parseAndValidateMessage()
  ├── detectPipeline()          // 关键词检测 → 同步/异步/审批
  ├── executeSyncPath()         // gRPC → SQL run → SSE
  ├── executeAsyncPath()        // NATS publish → SSE
  └── executeApprovalPath()     // 创建审批任务 → SSE
```

### D5: App.tsx 拆分

**决策**：提取以下独立组件：

```
web/src/
├── components/
│   ├── ChatPanel.tsx           # 消息列表 + 输入框（~150行）
│   ├── SessionSidebar.tsx      # 会话列表 + 创建（~120行）
│   ├── DataManagementPanel.tsx # 数据管理面板（~200行）
│   └── SSEHandler.tsx          # SSE 连接 + 重连逻辑（~80行）
├── App.tsx                     # 编排层（~200行 → 从812行缩减）
```

### D6: Python 超时保护

**决策**：
- OpenAI 调用：`httpx.Timeout(60.0)` 或 `openai.Timeout(60)`
- LangGraph `graph.ainvoke()`：使用 `asyncio.wait_for(..., timeout=120)`
- gRPC servicer：`context.deadline()` 检查

### D7: NATS 优雅关闭

**决策**：`consumer.py` 和 `__main__.py` 中 `nc.close()` 改为 `await nc.drain()`，等待 in-flight 消息完成后再关闭。

## Risks / Trade-offs

- [Risk] Service 层抽取可能引入 bug → 通过补充测试 + 逐步迁移降低风险
- [Risk] 限流 fail-close 可能导致 Redis 故障时拒绝所有请求 → 默认 fail-open，通过配置项逐步切换
- [Risk] Python 超时设得太短可能误杀正常请求 → 采用宽松超时（60s/120s），后续根据观测数据调整
- [Risk] App.tsx 拆分可能引入 prop drilling → 必要时引入 Context，但优先保持简单

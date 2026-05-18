# 项目不足汇总

> 基于 2026-05-18 全量代码审查整理，覆盖 Go 后端、Python AI Worker、React 前端三端。

---

## 严重程度说明

| 级别 | 含义 |
|---|---|
| ⛔ 致命 | 生产环境可能直接导致事故或安全漏洞 |
| 🔴 严重 | 明显影响可靠性、安全性或可运维性 |
| 🟡 中等 | 需要优化但当前不影响核心功能 |
| 🟢 轻微 | 最佳实践层面的改进空间 |

---

## 一、架构问题

| # | 问题 | 位置 | 级别 | 影响 |
|---|---|---|---|---|
| 1 | **SSE 总线为单进程内存方案**，进程重启丢失所有连接，无法水平扩展，32 事件缓冲易溢出 | `ssebus/bus.go:22-26` | ⛔ 致命 | 部署重启导致用户实时推送中断；多实例部署场景不支持 |
| 2 | **NATS 故障静默降级**，连接失败仅记录日志，异步任务发布静默失败，用户收到 202 后永远收不到结果 | `cmd/api/main.go:103-106`、`async/task.go:57-62` | ⛔ 致命 | 复杂分析任务无任何完成回调，用户无错误反馈 |
| 3 | **无 gRPC 熔断保护**，Python Worker 宕机时请求挂起直到 60 秒超时 | `worker/` | 🔴 严重 | 依赖下游服务的接口退化为不可用，且无法快速失败 |
| 4 | **Python 线程池耗尽风险**，gRPC 线程池 (8 workers) 与 `asyncio.to_thread` 双线程叠加 | `__main__.py:187`、`consumer.py:44` | 🔴 严重 | 高并发场景下可能线程死锁 |

---

## 二、安全问题

| # | 问题 | 位置 | 级别 | 影响 |
|---|---|---|---|---|
| 5 | **SQL 只读检测为关键字匹配**，攻击者可用 `/**/INSERT/**/`、换行符等方式绕过 | `sqlrun/classify.go` | ⛔ 致命 | 理论上可绕过只读限制执行写操作 |
| 6 | **RBAC 角色映射两份重复定义**，需手动同步，漏改可致权限绕过 | `middleware.go:96`、`permissions.go:9` | 🔴 严重 | 新增角色如只更新一处，则权限管控失效 |
| 7 | **Token 撤销机制失败时放行**，Redis 不可用时已撤销令牌仍可正常使用 | `auth/revoke.go:17-26` | 🟡 中等 | 降低了撤销机制可靠性，但属于文档注明的设计取舍 |
| 8 | **报告代理路径遍历风险**，`run_id` 直接嵌入文件路径 | `report_generation_agent.py:48-53` | 🟡 中等 | 内部生成 ID 风险较低，但为不安全实践 |

---

## 三、代码质量问题

| # | 问题 | 位置 | 级别 | 影响 |
|---|---|---|---|---|
| 9 | **中英文错误信息混用**，"仅 admin 可创建用户" 与 "kind must be postgres" 并存，无本地化层 | 各 `handlers/` 文件 | 🟡 中等 | 接口响应语言不一致 |
| 10 | **多处错误被静默吞没**：Go 端 `json.Marshal` 错误丢弃、`rows.Err()` 未检查；Python 端 gRPC 回调错误仅日志；前端大量 `catch { /* ignore */ }` | 各端多处 | 🔴 严重 | 错误无法被上层感知，调试困难，用户无反馈 |
| 11 | **Python `HubInternalClient` 全局单例反模式**，同一客户端在三处模块独立定义和初始化 | `graph.py:22`、`consumer.py:14`、`tracing.py:8` | 🟡 中等 | 代码重复，通道关闭时线程不安全 |
| 12 | **RAG 上下文悬空未使用**，`AgentState.rag_context` 已定义但所有代理节点均未读取 | `state.py:7` | 🟡 中等 | ChromaDB 知识库实际未接入 LangGraph 流程 |
| 13 | **前端 `useMemo` 中包含 `navigate()` 副作用**，违反 React 纯语义 | `useRole.ts:28`、`useWorkspaceId.ts:28` | 🟡 中等 | 可能触发意外渲染循环 |
| 14 | **前端 6 处 `any` 类型滥用**，绕过 `strict: true` 编译检查 | `DataSourcesPage.tsx:72,101`、`AdminUsersPage.tsx:54,68,78` | 🟢 轻微 | 类型安全性打折扣 |

---

## 四、可运维性问题

| # | 问题 | 位置 | 级别 | 影响 |
|---|---|---|---|---|
| 15 | **硬编码 Linux 路径**，`/tmp/reports` 和 `/data/langgraph/checkpoints.db` 在 Windows/macOS 上直接崩溃 | `chart_agent.py:19`、`graph.py:134` | 🔴 严重 | 非 Linux 开发环境无法运行 |
| 16 | **NATS 无断线重连逻辑**，消费者一次连接失败即退出 | `consumer.py:88`、`knowledge_consumer.py:84` | 🔴 严重 | 网络抖动导致消费者永久退出 |
| 17 | **配置验证缺失**，非必需项传入非法值时被 `mustInt32` 静默替换为默认值 | `internal/config/config.go` | 🟡 中等 | 配置拼写错误不易发现 |
| 18 | **单工作区硬编码**，多租户能力仅预留了 `workspace_id` 列，实际用固定 UUID | `seed/`、各处 SQL 查询 | 🟡 中等 | 多工作区部署需要大改 |

---

## 五、测试覆盖盲区

| # | 盲区 | 位置 | 级别 |
|---|---|---|---|
| 19 | middleware 鉴权中间件、ratelimit 限流器、crypto 加解密、async 异步任务、telemetry 指标、grpcserver 回调 — **6 个 Go 核心模块无任何测试** | 对应包目录 | 🔴 严重 |
| 20 | Python 服务入口 `__main__.py`、知识消费 NATS 消费者 `knowledge_consumer.py` 无测试 | 对应文件 | 🟡 中等 |
| 21 | 前端核心组件 `App.tsx`（398 行）、API 层 `api.ts`、`ChartView`、`ErrorBoundary` 无测试 | 前端各文件 | 🔴 严重 |
| 22 | Python 测试依赖 `sys.modules` 操作，顺序敏感，并行运行可能失败 | `test_chart_agent.py` | 🟡 中等 |

---

## 六、前端工程化问题

| # | 问题 | 位置 | 级别 |
|---|---|---|---|
| 23 | **约 60% 样式为内联 `style={...}`**，CSS Modules 仅占约 40%，不利于主题化和暗色模式 | `App.tsx`、`ModeSelector.tsx`、各管理页面 | 🟢 轻微 |
| 24 | **无集成/E2E 测试**，用户完整流程（登录→选择 Session→发送→查看进度→查结果）无自动化验证 | 整个前端 | 🟡 中等 |

---

## 优先级建议

### 第一优先（致命级，需立即修复）
- SSE 总线单进程瓶颈 → 考虑 Redis pub/sub 替代
- NATS 故障静默降级 → 添加超时检测和用户反馈
- SQL 关键字检测绕过 → 改用词法解析或 AST 分析

### 第二优先（严重级，需尽快修复）
- gRPC 熔断保护 → 添加健康检查和熔断器
- Python 线程池优化 → 改用 `grpc.aio` 异步客户端
- RBAC 角色映射统一 → 抽取公共常量
- 硬编码 Linux 路径 → 使用环境变量配置
- NATS 重连机制 → 添加重试和退避逻辑
- 核心模块无测试 → 优先补充 middleware、ratelimit、App.tsx 测试

### 第三优先（中低级，持续改进）
- 错误信息本地化统一
- `any` 类型消除
- 内联样式迁移到 CSS Modules
- E2E 测试体系建设

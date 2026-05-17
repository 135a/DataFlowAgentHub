## Why

代码质量审计（`docs/CODE_QUALITY_AUDIT.md`）发现 3 个严重问题、4 个重要问题、6 个改进项，涉及运行时正确性、安全性、可维护性三个维度。这些问题当前已在生产路径上，需要系统性修复以降低线上故障风险和技术债务积累。

## What Changes

- **修复 ~25 处 error swallowing**：将所有 `_ =` 丢弃 error 改为至少记录日志，关键路径（SSE 推送、状态更新）必须处理错误
- **扩展限流覆盖范围**：为登录、注册、数据源管理等端点增加限流中间件，限流不可用时应 fail-close 而非 fail-open
- **补充 handler 测试**：为核心 handler（数据源 CRUD、用户管理、内部回调、知识文档）添加测试覆盖
- **拆分 App God Object**：引入 service 层，将 SQL 逻辑从 handler 层剥离；`PostMessage` 拆分为多个小函数
- **分解 App.tsx**：抽取 ChatPanel、SessionSidebar、DataManagementPanel 等独立组件
- **Python worker 超时保护**：OpenAI 调用和 LangGraph invoke 加入超时机制
- **NATS 优雅关闭**：连接关闭前 drain in-flight 消息
- **技术债务清理**：SQL 只读检测强化、SSE backpressure 监控、Python requirements.txt、前端 any 类型修复、重复代码消除

## Capabilities

### New Capabilities

- `error-handling`: 消除所有 `_ =` 静默丢弃 error 的模式，关键路径 error 必须处理或记录
- `rate-limiting`: 将限流中间件扩展到登录、注册、数据源管理等敏感端点
- `handler-tests`: 为数据源 CRUD、用户管理、内部回调、知识文档等 handler 补充测试
- `code-organization`: 拆分 App God Object（引入 service 层）和 App.tsx 巨型组件
- `python-resilience`: Python worker 增加超时保护和 NATS drain 优雅关闭

### Modified Capabilities

<!-- 无现有 spec 需要修改 -->

## Impact

- **Go handlers/**：全部 7 个文件，error 处理、service 层抽取、测试补充
- **Go middleware/**：限流中间件挂载点扩展
- **Go ratelimit/**：fail-open 改为可配置或 fail-close
- **web/src/**：App.tsx 拆分、any 类型修复
- **services/ai/**：Python worker 超时、NATS drain、requirements.txt
- **internal/sqlrun/**：SQL 只读检测强化
- **internal/ssebus/**：backpressure 监控

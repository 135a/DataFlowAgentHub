## Context

当前项目（commit `0cbb6ac`）是一个"面试向 MVP"对话式数据分析平台，架构为 Go（chi 路由）+ Python（LangGraph/gRPC），通过 Docker Compose 部署。经过全量代码审查（详见 `docs/ISSUES.md`），识别出 41 个问题，其中 8 个为编译/运行时阻断级 Bug。本次设计旨在以最小风险修复阻断问题，补齐核心功能缺口，并对齐"Go 底座 + Python 智能层"的架构愿景。

关键约束：
- Go 和 Python 的职责边界必须清晰（Go 不写 AI 逻辑，Python 不承担高性能调度）
- Docker Compose 一键部署体验不能退化
- 不引入破坏性的技术栈变更（保持 chi/pgx/NATS，不迁移到 Gin/MySQL/RocketMQ）

## Goals / Non-Goals

**Goals:**
- 修复全部编译和运行时阻断级 Bug（ISSUES #1-#8）
- 补齐 Multi-Agent 编排器核心链路（nl2sql_node 从 Mock → 真实 gRPC 调用）
- 打通知识索引全链路（上传→NATS→ChromaDB→回调）
- 修复 SSE 浏览器端可用性
- 加密数据源密码存储
- 消除硬编码密钥，建立环境变量强制校验
- 为 Go 核心模块和 Python Agent 模块建立基础测试
- 完善数据库迁移追踪和执行机制
- 添加 OTel 链路导出

**Non-Goals:**
- 不将 chi 替换为 Gin（chi 已满足需求）
- 不将 PostgreSQL 替换为 MySQL
- 不将 NATS 替换为 RocketMQ
- 不引入 etcd 服务注册发现（当前 Docker Compose 单机部署不需要）
- 不添加 FastAPI 层（Python gRPC Server 已足够）
- 不实现多数据源类型（MySQL）的完整适配——仅预留扩展点
- 不添加 Kubernetes/Helm 部署方案
- 前端仅补全关键管理页面，不做完整 UI 重构

## Decisions

### D1: SSE 认证方案——Query Parameter Token

**背景**: 浏览器 `EventSource` API 不支持自定义请求头，而当前 SSE 端点使用 `Authorization: Bearer <jwt>` 认证。

**方案对比**:

| 方案 | 优点 | 缺点 |
|------|------|------|
| A. Query Param Token | 简单，EventSource 原生支持 | Token 暴露在 URL 中 |
| B. Cookie-based Session | 浏览器自动携带 | 需引入 session 管理，CSRF 风险 |
| C. WebSocket 替代 SSE | 支持自定义头 | 需重写前后端通信层 |

**决策**: 选择 **方案 A**——SSE 端点同时支持 Header 认证和 `?token=<jwt>` 查询参数认证。中间件层统一处理，优先检查 Header 中的 Bearer Token，其次检查查询参数。Token 在 URL 中的风险通过以下方式缓解：
- 使用短期 SSE Token（独立于普通 JWT，有效期 1 小时）
- SSE Token 仅含最小权限（仅允许订阅对应 session 的事件流）

### D2: 数据源密码加密——AES-256-GCM + 环境变量密钥

**背景**: `data_sources.password` 当前以明文 TEXT 存储。

**决策**: 使用 AES-256-GCM 对称加密，密钥来自环境变量 `HUB_DB_ENCRYPTION_KEY`：
- 写入时：Go 端使用 `crypto/aes` + `crypto/cipher` 进行 AES-GCM 加密，密文以 base64 存入数据库
- 读取时：Go 端解密后传递给 Python Worker（gRPC 通信本身需配合 TLS，见 D5）
- `password` 列从 `TEXT` 改为 `TEXT`（存储 base64 密文，长度增加）
- 新增配置字段 `HUB_DB_ENCRYPTION_KEY`（32 字节，hex 编码，必填）

### D3: 数据库迁移追踪——简单版本表

**背景**: 当前 `internal/migrate/migrate.go` 每次启动重新执行所有 SQL，依赖 `IF NOT EXISTS` 保证幂等，无法处理 ALTER/ADD COLUMN 操作。

**决策**: 引入 `schema_migrations` 版本追踪表：
```sql
CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```
- 执行迁移前查询 `schema_migrations` 获取已应用的版本集合
- 跳过已应用的迁移文件，仅执行新迁移
- 将外部 `migrations/` 目录下的 005-007 文件嵌入到 `internal/migrate/` 中，统一管理
- 重命名迁移文件使其连续（001, 002, 003, 004），消除编号缺口

### D4: nl2sql_node 真实化——复用已有 gRPC 调用

**背景**: `orchestrator/graph.py` 的 `nl2sql_node` 返回硬编码 Mock 数据。

**决策**: `nl2sql_node` 内部通过 gRPC 调用 Go API，再由 Go 调用 Python `GenerateSQL` RPC（避免 Python 直连数据库，保持安全边界）：
```
nl2sql_node → HTTP POST /internal/nl2sql (Go handler) → gRPC GenerateSQL (Python) → 返回 SQL → Go 执行 SQL → 返回结果 → nl2sql_node
```
选择 HTTP 回调而非直接 gRPC 调用，因为：
- Python Worker 不应直接访问 Go 控制面的数据库连接
- 保持 Go 的只读 SQL 执行器（`sqlrun`）作为唯一 SQL 执行入口
- 复用已有的认证和审计链路

### D5: Dockerfile 修复与 gRPC 桩代码同步

**问题**: `Dockerfile.ai` 仅拷贝 `hub_ai/` 目录；gRPC 桩代码缺少 `RunAgentPipeline`。

**决策**:
- `Dockerfile.ai` 拷贝整个 `services/ai/` 目录（含 `orchestrator/`、`agents/`、`rag/`）
- 重新运行 `make gen` 生成完整的 Go 和 Python 桩代码
- 在 `internal/worker/nl2sql.go` 中添加 `RunAgentPipeline` 客户端方法

### D6: 硬编码密钥消除——分层策略

**决策**:
| 密钥 | 策略 |
|------|------|
| `HUB_JWT_SECRET` | 保持已有强制校验（启动失败若未设置） |
| `HUB_INTERNAL_HMAC_SECRET` | 新增强制校验（启动失败若未设置），添加 config 字段 |
| `HUB_SEED_PASSWORD` | 新增复杂度校验（>=8 字符），弱密码仅打印警告不阻塞启动 |
| `docker-compose.yml` | 所有密钥改为 `${VAR}` 引用，`docker-compose.yml` 不再含硬编码值 |

### D7: 限流器算法升级——滑动窗口

**决策**: 将固定窗口限流改为滑动窗口（Sliding Window Log），使用 Redis Sorted Set 实现：
- 每个请求记录时间戳到 Sorted Set
- 检查时删除窗口外的旧记录，统计窗口中剩余记录数
- Redis 不可用时保持 fail-open 策略（当前行为不变）

### D8: 语言路由替换——显式参数

**背景**: 中文关键词 `"分析"`、`"报告"` 硬编码判断是否触发 Multi-Agent 流程。

**决策**: 在 API 请求中增加可选的 `workflow` 参数：
```json
{ "message": "查询本月数据", "workflow": "agent_pipeline" }
```
- `workflow=auto`（默认）：向后兼容，保留关键词检测
- `workflow=simple`：强制走同步 NL2SQL 路径
- `workflow=agent_pipeline`：强制走 Multi-Agent 编排路径
- 前端在发送按钮旁增加"深度分析"开关

## Risks / Trade-offs

- **[SSE Token 在 URL 中暴露]** → 使用短期专用 Token（1h），独立于主 JWT；后续可迁移到 WebSocket 方案
- **[加密密钥管理]** → `HUB_DB_ENCRYPTION_KEY` 泄露将导致所有数据源密码可被解密；部署文档需强调密钥安全；后续可接入 KMS
- **[nl2sql_node 额外 HTTP 跳转]** → 增加一次 HTTP 往返延迟（内网 <5ms）；可通过 gRPC 直连优化（需同步安全设计）
- **[迁移追踪表方案简单]** → 不支持分布式锁，多副本同时启动可能竞态；当前单实例部署无影响，后续可加 advisory lock
- **[OTel 导出可能影响启动]** → Collector 不可达时用 `OTEL_EXPORTER_OTLP_ENDPOINT` 的默认超时（10s），不阻塞 API 启动（后台重连）
- **[前端新页面增加包体积]** → 数据源管理和知识文档页面均为懒加载（React.lazy），不影响首屏加载

## Open Questions

1. TLS 全链路加密（gRPC/NATS/Redis）——MVP 阶段是否引入 mTLS？建议在非 Docker Compose 的生产部署方案中单独处理
2. MySQL 数据源的完整适配——当前仅预留接口扩展点，具体实现范围待后续变更
3. JWT 黑名单存储——使用 Redis 还是在 Postgres 中建表？倾向于 Redis（带 TTL 自动清理过期条目）

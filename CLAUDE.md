# CLAUDE.md

本文件为 Claude Code (claude.ai/code) 在此仓库中工作时提供指导,输出的相关文档都必须用中文输出,要保证代码的可读性

## 常用命令

```bash
# Docker Compose 一键启动（全栈：postgres、redis、chroma、nats、ai-worker、api）
docker compose -f deploy/compose/docker-compose.yml --env-file .env up -d --build

# Go 本地开发（不依赖 Docker）
$env:GOMODCACHE="$PWD\.gomodcache"
$env:GOSUMDB="off"
go mod tidy
go run ./cmd/api                                    # 启动 API 服务

# 运行全部 Go 测试
make test                                           # 等同于 go test ./...

# Proto 代码生成
make gen-go                                         # 生成 Go gRPC 桩代码 -> internal/gen/
make gen-py                                         # 生成 Python gRPC 桩代码 -> services/ai/gen/
make gen                                            # 同时生成 Go 和 Python

# 前端
cd web && npm install && npm run dev                # Vite 开发服务器，默认代理到 :8080
```

## 架构

这是一个 **Go 控制面 + Python AI 计算面** 的对话式数据分析平台（MVP）。用户用自然语言提问，系统生成 SQL、连接数据库执行查询并返回结果，同时可选配 Multi-Agent 分析（LangGraph）、RAG 知识检索（ChromaDB）和人工审批关卡。

### 服务拓扑（Docker Compose）

```
[Browser / React SPA] --> [Go API :8080 (chi 路由)] --> [Python ai-worker :50051 (gRPC)]
                              |         |                        |
                         [Postgres]  [Redis]              [ChromaDB (向量库)]
                              |         |
                         [NATS (JetStream)]
```

### Go 代码布局（`internal/`）

| 包 | 职责 |
|---|---|
| `handlers/` | HTTP 层。单一 `App` 结构体持有所有依赖（DB、Redis、gRPC 客户端、SSE 总线、配置），`Routes()` 通过 chi 挂载全部端点。 |
| `middleware/` | TraceID 注入/提取、JWT 认证、RBAC 辅助函数（`RequireMinRole`，支持 viewer/operator/admin）、结构化请求日志。 |
| `config/` | 从 `HUB_*` 前缀的环境变量加载全部配置，仅 `HUB_JWT_SECRET` 为必填项。 |
| `auth/` | JWT 签发/解析（HS256），Claims 包含 `UserID`、`WorkspaceID`、`Role`。 |
| `worker/` | 连接 Python ai-worker 的 gRPC 客户端（`GenerateSQL`、`RunAgentPipeline`、`Health`）。 |
| `ssebus/` | 服务端事件（SSE）的内存发布/订阅总线，每个 session ID 对应一个通道，用于向浏览器实时推送运行状态。 |
| `sqlrun/` | 只读 SQL 执行器。通过关键字检测（INSERT/UPDATE/DELETE 等）阻断写操作，强制限制返回行数和超时时间。 |
| `migrate/` | 通过 `embed.FS` 将 `*.sql` 文件打包进二进制，启动时按字母序自动执行。 |
| `seed/` | 首次启动时创建默认管理员用户。 |
| `ratelimit/` | 基于 Redis 的固定窗口限流器，Redis 不可用时放行（fail-open）。 |
| `telemetry/` | Prometheus 指标（`hub_http_requests_total`、耗时直方图）及 `/metrics` 端点。 |
| `otelsetup/` | OpenTelemetry TracerProvider 初始化（W3C 传播，MVP 阶段无导出器）。 |
| `llm/` | 最小化的 OpenAI 兼容聊天补全 HTTP 客户端，429/5xx 自动重试。 |
| `connector/` | PostgreSQL 连接池封装，用于检测存活和列出 public schema 表。 |
| `schema/` | 查询 `information_schema.columns` 发现表结构，支持 Redis 缓存（`CachedSchema`）。 |
| `async/` | 异步任务队列：写入 `async_tasks` 表 + 发布到 NATS `hub.tasks.agent_pipeline`，含后台 reaper goroutine 用于过期任务清理。 |

### 请求流程

1. **同步路径（简单 NL2SQL）**：用户发消息 → Go handler 通过 gRPC 调用 Python `GenerateSQL` → Go 执行返回的 SQL（只读）→ 结果内联返回。
2. **异步路径（Multi-Agent）**：消息含 "analyze"/"report" 关键词 → Go 插入 `async_tasks` 行 + 发布 NATS 消息 → 返回 202 及 `task_id` → Python NATS 消费者运行 LangGraph 图（NL2SQL → Analysis → Report）→ 通过 HTTP `POST /internal/tasks/{id}/callback` 回调 Go → 前端轮询或接收 SSE 事件。
3. **审批关卡**：含 "export" 关键词的消息创建 `approval_tasks` 记录，需 operator/admin 审批通过后才继续执行。

### Python AI Worker（`services/ai/`）

- `hub_ai/` — gRPC 服务端（当前仍为桩代码阶段，尚未完整实现 servicer）。
- `orchestrator/` — LangGraph `StateGraph`，节点依次为：NL2SQL →（分支）→ Analysis → Report。`consumer.py` 订阅 NATS `hub.tasks.agent_pipeline`，运行图后将结果通过 HTTP 回调 Go。
- `agents/` — `data_analysis_agent.py`（pandas 统计分析 + LLM 业务摘要）和 `report_generation_agent.py`（Markdown 报告 + Excel 导出）。
- `rag/` — 基于 ChromaDB 的知识库：文档分块、嵌入（`text-embedding-3-small`）、语义搜索，每个工作区一个 Collection。

### 数据库

核心 schema（嵌入在 `internal/migrate/001_init.sql` 中）：`workspaces`、`users`、`data_sources`、`sessions`、`messages`、`runs`、`approval_tasks`、`audit_events`。补充迁移在 `internal/migrate/`（002–004），新增 `async_tasks`、`knowledge_docs`、`agent_run_steps`。

### 关键设计决策

- **JWT 认证**：交互式用户使用 JWT（HS256，通过 `POST /v1/auth/login` 获取），Bcrypt 验证密码。
- **只读 SQL 强制**：`sqlrun.IsReadOnlySQL()` 在执行前阻断写关键字。
- **限流失效放行**：Redis 不可用时请求不受限流影响。
- **过期保护**：异步任务设有 `expires_at`，后台 reaper goroutine 将过期任务标记为已过期。
- **Go 共享包暂空**：`pkg/` 目前仅含 `.gitkeep` 占位。

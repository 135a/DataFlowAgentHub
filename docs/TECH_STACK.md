# DataFlowAgentHub 技术栈

> 最后更新：2026-05-16 | commit `0fdb7ca`

---

## 架构概览

```
┌─────────────────────────────────────────────────────────────────────┐
│                         前端 (React SPA)                             │
├─────────────────────────────────────────────────────────────────────┤
│  React 18 │ TypeScript 5.6 │ Vite 5 │ React Router 6 │ nginx        │
└──────────────────────────────┬──────────────────────────────────────┘
                               │ HTTP/SSE
┌──────────────────────────────┴──────────────────────────────────────┐
│                      控制面 (Go API :8080)                            │
├─────────────────────────────────────────────────────────────────────┤
│  chi 路由  │  pgx/v5  │  go-redis  │  nats.go  │  gRPC Client       │
│  JWT HS256 │  bcrypt  │  AES-256-GCM 加密                            │
│  Prometheus │  OpenTelemetry (W3C) │  zap 结构化日志                  │
│  Go embed 迁移  │  SSE 内存总线                                       │
└──────────────────────────────┬──────────────────────────────────────┘
                               │ gRPC           │ NATS (JetStream)
         ┌─────────────────────┘                │
         ▼                                      ▼
┌────────────────────────────┐  ┌────────────────────────────────────┐
│  Python gRPC Server :50051 │  │  Python NATS Consumers (daemon 线程)│
│  (ai-worker)               │  │  (ai-worker 同进程)                  │
├────────────────────────────┤  ├────────────────────────────────────┤
│  gRPC (grpcio)             │  │  nats-py │ httpx (HTTP 回调 Go)     │
│  OpenAI SDK (LLM 调用)     │  │  LangGraph (StateGraph)             │
│  prompt 拼接 → 生成 SQL    │  │   ├─ MemorySaver (checkpointer)     │
│  SQL 只读检测 (双重守卫)    │  │   ├─ nl2sql_node                    │
│                            │  │   ├─ data_analysis_agent            │
│                            │  │   └─ report_generation_agent        │
│                            │  │                                     │
│                            │  │  Knowledge Consumer (独立)           │
│                            │  │   ├─ RecursiveCharTextSplitter      │
│                            │  │   ├─ OpenAI text-embedding-3-small  │
│                            │  │   └─ ChromaDB Client (向量存储)     │
│                            │  │                                     │
│                            │  │  OpenTelemetry (Tracing)            │
└────────────────────────────┘  └────────────────────────────────────┘
```

---

## 按层详述

### 前端

| 技术 | 版本 | 用途 |
|------|------|------|
| React | ^18.3 | UI 框架 |
| TypeScript | ~5.6 | 类型安全 |
| Vite | ^5.4 | 开发服务器与构建 |
| React Router DOM | ^6.28 | 客户端路由 |
| nginx | - | 生产环境静态文件托管 |

### Go 控制面

| 技术 | 版本 | 用途 |
|------|------|------|
| Go | 1.25 | 运行时 |
| chi/v5 | v5.2 | HTTP 路由（轻量级） |
| pgx/v5 | v5.7 | PostgreSQL 驱动（连接池） |
| go-redis/v9 | v9.7 | Redis 客户端 |
| nats.go | v1.37 | NATS 客户端（异步任务发布） |
| google.golang.org/grpc | v1.80 | gRPC 客户端（调 Python） |
| golang-jwt/jwt/v5 | v5.2 | JWT HS256 签发与解析 |
| golang.org/x/crypto | v0.49 | bcrypt 密码哈希、AES-256-GCM |
| OpenTelemetry | v1.43 | W3C 分布式追踪 |
| Prometheus client | v1.21 | HTTP 指标暴露（`/metrics`） |
| zap | v1.27 | 结构化日志 |
| embed.FS | 标准库 | SQL 迁移文件打包 |
| google/uuid | v1.6 | UUID 生成 |

### Python AI 计算面

| 技术 | 版本 | 用途 |
|------|------|------|
| grpcio + tools | 1.68 | gRPC 服务端 |
| protobuf | 5.28 | Proto 序列化 |
| openai | 1.57 | LLM 对话补全 + Embeddings |
| langgraph | >=0.2 | Multi-Agent 图编排 |
| langchain | >=0.3 | RAG 工具链 |
| langchain-openai | >=0.2 | OpenAI Embeddings 集成 |
| chromadb | >=0.5 | 向量存储客户端 |
| pandas | >=2.2 | 数据分析 |
| numpy | >=1.26 | 数值计算 |
| openpyxl | >=3.1 | Excel 导出 |
| nats-py | >=2.7 | NATS 消费者 |
| httpx | >=0.27 | 异步 HTTP 回调 |
| opentelemetry-sdk | >=1.25 | 分布式追踪 |
| tabulate | >=0.9 | DataFrame → Markdown |

### 基础设施

| 组件 | 镜像/版本 | 用途 |
|------|----------|------|
| PostgreSQL | 16-alpine | 主数据库（workspaces、users、sessions、runs、approval_tasks、audit_events、knowledge_docs、async_tasks） |
| Redis | 7-alpine | 限流 + Schema 缓存 |
| ChromaDB | 0.5.20 | 向量数据库（RAG 知识库检索） |
| NATS | 2.10-alpine (JetStream) | 异步消息队列 |
| Docker Compose | - | 一键部署 |

---

## 协议与通信

```
前端 ←── HTTP/SSE ──→ Go API :8080
                        │
          ┌─────────────┼─────────────┐
          ▼             ▼             ▼
     Postgres        Redis        NATS (JetStream)
     (直连 pgx)    (go-redis)         │
                                      │ nats.go 发 / nats-py 收
                                      ▼
                             Python ai-worker :50051
                                      │
                                      │ httpx (HTTP 回调 + HMAC 签名)
                                      ▼
                                 Go API /internal/*
                                      │
                                      │ gRPC
                                      ▼
                             Python ai-worker (GenerateSQL)
```

### 请求路径

| 路径 | 协议 | 延迟 | 说明 |
|------|------|------|------|
| 同步 NL2SQL | 前端 → HTTP → Go → gRPC → Python → Go sqlrun → HTTP 响应 | ~1-2s | 不含分析/报告关键词时默认走此路径 |
| 异步 Agent | 前端 → HTTP → Go → NATS → Python Consumer → LangGraph → HTTP 回调 → SSE 推送 | ~5-15s | 含分析/报告关键词 |
| RAG 索引 | 前端 → HTTP → Go → NATS → Python Knowledge Consumer → ChromaDB → HTTP 回调 | ~5s | 知识文档上传 |

---

## 安全技术

| 机制 | 实现细节 |
|------|---------|
| 用户认证 | JWT HS256（`POST /v1/auth/login` 签发，Claims 含 UserID、WorkspaceID、Role） |
| 服务间认证 | `X-Hub-Signature`（HMAC-SHA256，内部回调专用） |
| 密码存储 | bcrypt |
| 数据源密码 | AES-256-GCM 加密后存 DB |
| SQL 注入防护 | 双重只读守卫：Go 侧 `sqlrun.IsReadOnlySQL()` + Python 侧 `_read_only_ok()`，均通过关键字黑名单阻断写操作 |
| 限流 | Redis 固定窗口计数器，Redis 不可用时 fail-open |
| RBAC | viewer / operator / admin 三级角色，`RequireMinRole` 中间件 |

---

## 已知局限

| 项目 | 现状 | 改进方向 |
|------|------|---------|
| LangGraph checkpointer | `MemorySaver`，进程重启后状态丢失 | 迁至 `SqliteSaver`（已计划） |
| SSE 总线 | 内存实现 | 多副本场景需迁至 Redis pub/sub |
| OpenTelemetry | MVP 阶段仅初始化 TracerProvider，无导出器 | 接入 Jaeger/Grafana |
| 前端 | Vite proxy 开发模式 | `Dockerfile.web` 已拆出，需补全 nginx 部署方案 |
| Prompt | 字符串拼接在 `__main__.py` 中 | 抽为独立模板文件（已计划） |

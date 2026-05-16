# 网关架构分析与演进方案

> 撰写日期: 2026-05-16 | 基于 commit `0cbb6ac` 之后的代码状态

## 一、事实纠正：不是"三个网关"，是"两层模型"

在最初的分析中，我们按通信路径将 Go API Server 的职责拆成了"三个网关"——前置网关（Browser→Go）、Worker 网关（Go→Python）、回调网关（Python→Go）。但这容易造成一个误解：**好像有三个独立的东西需要维护**。

看代码就知道，它们其实是同一个：

```go
// cmd/api/main.go
app := &handlers.App{...}        // ← 一个 App，持有所有依赖
base := handlers.Routes(app)     // ← 一个 Routes()，定义全部路由
srv.ListenAndServe()             // ← 一个 HTTP Server
```

```go
// handlers/handlers.go - Routes()
func Routes(a *App) http.Handler {
    r := chi.NewRouter()
    // 全局中间件：Recoverer, Timeout, TraceID, RequestLog, Prometheus

    r.Route("/v1", func(r chi.Router) {
        r.Use(middleware.Auth(...))       // JWT 认证
        ...
    })

    r.Route("/internal", func(r chi.Router) {
        r.Use(middleware.InternalHMACAuth(...))  // HMAC 认证
        ...
    })

    return r  // 一个 Handler
}
```

"三个网关"是一个有用的**分析框架**，帮助你理解三条通信路径。但物理上它们已经是一个整体。"能不能写成一个"的答案——**它们已经是一个了，但可以做更好的统一**。

真实的分裂不在进程边界，而在两个维度：

```
                    认证方式
                  JWT      HMAC
              ┌────────┬────────┐
调用    外向  │ /v1/*  │   —    │  Go 作为服务端
方向          ├────────┼────────┤
         内向  │   —    │ /int.* │  Python 作为客户端
              └────────┴────────┘
```

因此，更准确的架构术语应该是 **"两层"**：

---

## 二、正确的架构模型：入站层 + 出站层

```
┌──────────────────────────────────────────────────────────────┐
│                      Go API Server (:8080)                    │
│                                                               │
│  ┌─────────────────────────────────────────────────────┐     │
│  │               统一入站层 (Ingress)                    │     │
│  │                                                      │     │
│  │  全局中间件: TraceID → RequestLog → Prometheus       │     │
│  │                                                      │     │
│  │  统一认证: JWT | API Key | HMAC 任一通过即放行        │     │
│  │  统一限流: 按 user_id / source_ip 区分粒度           │     │
│  │  统一 RBAC: viewer < operator < admin               │     │
│  │                                                      │     │
│  │  /v1/*            ← Browser/用户 (JWT/API Key)       │     │
│  │  /internal/*      ← Python Worker (HMAC)             │     │
│  │  /metrics,/health ← 运维探活                          │     │
│  └─────────────────────────────────────────────────────┘     │
│                              │                                │
│  ┌───────────────────────────┴───────────────────────────┐   │
│  │               统一出站代理层 (Egress)                  │   │
│  │                                                        │   │
│  │  gRPC    → Python AI Worker (GenerateSQL/AgentPipeline)│   │
│  │  NATS    → hub.tasks.* (异步任务投递)                  │   │
│  │  Redis   → 缓存 / 限流 / JWT吊销                       │   │
│  │  Postgres→ 业务数据 + 迁移 + 审计                       │   │
│  └────────────────────────────────────────────────────────┘   │
│                                                               │
│                    ┌──────────────────┐                       │
│                    │  sqlrun 守门员    │  ← 最后一道防线      │
│                    │  只读SQL + 行限制 │                       │
│                    │  + 超时控制       │                       │
│                    └──────────────────┘                       │
└──────────────────────────────────────────────────────────────┘
```

### 2.1 入站层：统一认证设计

当前的 `Auth` 和 `InternalHMACAuth` 是两个独立中间件。可以合并为一个 `UnifiedAuth`：

```go
// UnifiedAuth 依次尝试以下认证方式，任一通过即注入 Claims 并放行：
//
//   1. X-Hub-Signature 头      → HMAC-SHA256 签名验证（内部回调）
//   2. X-Hub-Api-Key 头        → API Key 匹配（服务间调用）
//   3. Authorization: Bearer   → JWT 解析 + 吊销检查（用户会话）
//   4. ?token= 查询参数         → JWT 解析（SSE EventSource）
//
// 全部失败 → 401

func UnifiedAuth(cfg *config.Config, log *zap.Logger, rdb *redis.Client) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // 尝试 HMAC（内部回调）
            if claims := tryHMAC(r, cfg.InternalHMACSecret); claims != nil {
                ctx := context.WithValue(r.Context(), ctxClaims, claims)
                next.ServeHTTP(w, r.WithContext(ctx))
                return
            }

            // 尝试 API Key
            if cfg.GlobalAPIKey != "" && r.Header.Get("X-Hub-Api-Key") == cfg.GlobalAPIKey {
                c := &auth.Claims{UserID: seed.ServiceAPIUserID, ...}
                ctx := context.WithValue(r.Context(), ctxClaims, c)
                next.ServeHTTP(w, r.WithContext(ctx))
                return
            }

            // 尝试 JWT Bearer / ?token=
            raw := extractBearerOrToken(r)
            if raw != "" {
                c, err := auth.Parse(cfg.JWTSecret, raw)
                if err == nil {
                    if rdb == nil || !isRevoked(r.Context(), rdb, c) {
                        ctx := context.WithValue(r.Context(), ctxClaims, c)
                        next.ServeHTTP(w, r.WithContext(ctx))
                        return
                    }
                }
            }

            http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
        })
    }
}
```

改造后的 `Routes()`：

```go
func Routes(a *App) http.Handler {
    r := chi.NewRouter()
    r.Use(chimw.Recoverer, chimw.Timeout(60*time.Second))
    r.Use(middleware.TraceID, middleware.RequestLog(a.Log), telemetry.PrometheusMiddleware)

    r.Get("/health", a.Health)
    r.Post("/v1/auth/login", a.Login)

    r.Route("/v1", func(r chi.Router) {
        r.Use(middleware.UnifiedAuth(a.Cfg, a.Log, a.Redis))
        // ... 所有 /v1 端点
    })

    r.Route("/internal", func(r chi.Router) {
        r.Use(middleware.UnifiedAuth(a.Cfg, a.Log, a.Redis)) // ← 同一个！
        // ... 所有 /internal 端点
    })

    return r
}
```

**收益：**
- `InternalHMACAuth` 不再需要独立存在，认证逻辑集中在一个函数
- 新增端点时不需要纠结"用哪个中间件"
- /internal 路由自动获得 JWT API Key 的降级认证能力（运维友好）

**需要额外处理：**
- /internal 端点应拒绝 JWT/API Key 认证的请求（内部端点只接受 HMAC）
  - 可以在 UnifiedAuth 返回的 Claims 中标记 `AuthMethod`（`"jwt"` / `"apikey"` / `"hmac"`）
  - /internal handler 检查 `claims.AuthMethod != "hmac"` → 403

### 2.2 出站层：统一的调度与治理

出站层目前分散在几个地方：

| 出站目标 | 当前实现 | 治理缺失 |
|----------|----------|----------|
| gRPC → Python | `worker/nl2sql.go` | 无 TLS，无 retry，无 circuit breaker |
| NATS → 异步任务 | `async/task.go` | 发布即忘，无 Confirm |
| Redis → 缓存/限流 | 各处直接调用 | 无连接池监控 |
| Postgres → 业务 | `pgxpool` | 有连接池，缺慢查询日志 |

出站层统一治理的方向：

```
┌─────────────────────────────────────────┐
│           出站调用封装                    │
│                                         │
│  统一 TraceContext 注入                  │
│  统一 Timeout/Deadline 传播              │
│  统一 Retry 策略（指数退避，最大 3 次）    │
│  统一 Circuit Breaker（连续失败 → 熔断）  │
│  统一 Metrics（延迟、错误率、吞吐量）     │
└─────────────────────────────────────────┘
```

当前 gRPC 调用已经部分做到了（TraceContext 注入、metadata 传递），但 NATS、Redis 的出站调用没有统一的横切关注点处理。

---

## 三、三层模型（作为分析框架保留）

虽然物理上是两层，但"三层"作为理解通信路径的分析框架仍有价值。以下保留原始分析。

### 3.1 前置路径 — 外部访问

**位置：** `internal/middleware/middleware.go` + `internal/handlers/handlers.go` (Routes 函数)

**数据流：**

```
Browser → Go POST /v1/sessions/{id}/messages
  ├─ TraceID 注入
  ├─ 结构化请求日志（API Key 脱敏）
  ├─ Prometheus 指标
  ├─ 认证（JWT / API Key / ?token=）
  │   └─ JWT 吊销检查（Redis JTI 黑名单）
  ├─ RBAC（viewer < operator < admin）
  ├─ 限流（Redis 滑动窗口，30 次/分钟/用户）
  └─ 业务处理
```

**存在意义：** 所有面向用户的功能通过同一套认证/授权/限流体系接入。SaaS 多租户模型的核心——一个用户一个 token，访问自己工作区的所有资源。

**关键设计决策：**
- **双认证**：交互式用户使用 JWT（HS256），服务间调用使用 `X-Hub-Api-Key`
- **SSE 兼容**：中间件同时支持 `Authorization: Bearer` Header 和 `?token=` 查询参数
- **SSE Token**：短期专用 JWT（1 小时，viewer 角色），降低 URL 中 Token 泄露的风险
- **JWT 吊销**：基于 Redis 的 JTI 黑名单，支持主动吊销

### 3.2 Worker 路径 — Go → Python AI 调用

**位置：** `internal/worker/nl2sql.go`

**Proto 接口：** `api/proto/nl2sql/v1/nl2sql.proto`

```
Go Handler                     Python gRPC Server
─────────                      ──────────────────
PostMessage / InternalNL2SQL   Servicer.GenerateSQL
  │                              │
  ├─ 注入 W3C TraceContext       ├─ 接收 schema_json + user_message
  ├─ 注入 x-trace-id             ├─ 构造 LLM prompt
  ├─ gRPC GenerateSQL ─────────▶ ├─ 调用 OpenAI API → 生成 SQL
  │                              └─ 返回 SQL + self_check_notes
  └─ 接收 SQL
     └─ sqlrun.QueryRows() ← 只读检查 + 行数限制 + 超时
```

**存在意义：** 架构核心原则——**Go 不写 AI 逻辑，Python 不碰数据库**。gRPC 是这条边界的协议载体。

**当前问题：**
1. gRPC 使用 `insecure.NewCredentials()`（无 TLS），内部流量明文
2. `RunAgentPipeline`（gRPC 路径）和 NATS Consumer（异步路径）是两条并行通路

### 3.3 回调路径 — Python → Go 反向通信

**位置：** `internal/handlers/handlers.go` (Routes 函数, `/internal` 路由组)

| 端点 | 调用方 | 用途 |
|------|--------|------|
| `POST /internal/nl2sql` | graph.py nl2sql_node | SQL 生成 + 执行 |
| `POST /internal/tasks/{id}/callback` | consumer.py | 异步任务完成通知 |
| `POST /internal/runs/{id}/steps` | tracing.py report_run_step | LangGraph 步骤追踪 |
| `PATCH /internal/knowledge-docs/{id}/status` | knowledge_consumer.py | 文档索引状态更新 |

**认证：** HMAC-SHA256 请求体签名（`X-Hub-Signature: sha256=<hex>`）

**存在意义：** "Go 底座"原则的必然产物。Python 需要在运行时执行 SQL（必须通过 sqlrun 守门员）、报告中间状态、通知最终结果。

**当前问题：**
1. HMAC 签名逻辑在 4 个 Python 文件中重复实现
2. `InternalNL2SQL` 与 `PostMessage` 同步路径逻辑高度重复
3. /internal 路由无速率限制保护

---

## 四、HTTP 回调 vs gRPC 回调对比

### 4.1 代码级对比（以 TaskCallback 为例）

**HTTP + HMAC（当前）：**

Python 调用方需要：手动 JSON 序列化 → 手动 HMAC 签名 → 手动构造 HTTP 请求 → 手动检查状态码。Go 接收方需要：中间件 HMAC 验证 → 手动 URL 参数解析 → 手动 JSON 解码 → 手动字段校验 → 手动枚举值校验。

**gRPC + mTLS：**

Proto 定义一次，两边自动生成桩代码。Python 直接调用类型安全的方法，Go 接收强类型的 request struct。认证在连接建立时通过 mTLS 一次性完成，业务代码零感知。

### 4.2 逐维对比

| 维度 | HTTP + HMAC | gRPC + mTLS |
|------|:-----------:|:-----------:|
| 类型安全 | JSON 弱类型，运行时校验 | Protobuf 强类型，编译期保证 |
| 认证 | 手动 HMAC（6 处代码） | 连接层自动 mTLS |
| 错误码 | HTTP 状态码，需手动映射 | gRPC Status Code，原生传递 |
| 连接管理 | 每次请求新建连接 | HTTP/2 长连接多路复用 |
| 流式能力 | 不支持 | Server Streaming 原生支持 |
| 中间件 | 手写（`InternalHMACAuth`） | 成熟 interceptor 生态（OTel/Recovery/Validator） |
| 代码生成 | 手写 struct + json tag | `protoc` 自动生成 |
| 调试 | `curl`/Postman 直接测试 | 需 `grpcurl` 或 grpc-gateway |
| 实现成本（现状） | 已实现 | 需要新建 |

### 4.3 选型建议

| 回调端点数量 | 推荐协议 | 理由 |
|:---:|------|------|
| < 5 个 | HTTP + HMAC | 实现成本低，调试方便 |
| 5-10 个 | HTTP 为主，核心路径 gRPC | 渐进迁移，新旧共存 |
| > 10 个 | 统一 gRPC | 类型安全 + 代码生成收益显著 |

当前 4 个回调端点，HTTP 回调本身不是瓶颈。**核心问题是 HMAC 逻辑重复**，而非协议选择。

---

## 五、生产化演进路径

### 5.1 方案一：入站认证统一 + HMAC 公共模块（推荐优先级：P0）

**目标：** 合并 `Auth` 和 `InternalHMACAuth` 为 `UnifiedAuth`，提取 Python HMAC 公共模块。

**改造清单：**

| # | 改造项 | 改动量 | 收益 |
|---|--------|:---:|------|
| 1 | Go `middleware.UnifiedAuth` 替代 `Auth` + `InternalHMACAuth` | ~80 行 | 认证逻辑集中 |
| 2 | Claims 增加 `AuthMethod` 字段，/internal handler 校验只允许 HMAC | ~20 行 | 内部端点安全隔离 |
| 3 | Python 提取共享 HMAC 模块 `hub_ai/internal_auth.py` | ~30 行 | 消除 4 处重复 |
| 4 | gRPC 连接加 TLS（自签证书） | ~10 行 | 内部流量加密 |
| 5 | `/internal` 路由加基础限流 | ~15 行 | 防止内部滥用 |
| 6 | gRPC 添加 `context.WithTimeout` deadline 传播 | ~5 行 | 防止级联超时 |

**总改动量：** ~160 行

**适用阶段：** 现在

### 5.2 方案二：架构重构（推荐优先级：P1）

**目标：** 支持水平扩展和多副本部署。

```
                         ┌─── Load Balancer ───┐
                         │                      │
                    ┌────┴────┐           ┌────┴────┐
                    │ Go API  │           │ Go API  │  ← 无状态，水平扩展
                    │ (实例1) │           │ (实例2) │
                    └────┬────┘           └────┬────┘
                         │                    │
                    ┌────┴────┬────┬────┬─────┴────┐
                    │         │    │    │          │
                    ▼         ▼    ▼    ▼          ▼
              ┌─────────┐ ┌──────┐ ┌──────┐  ┌──────────┐
              │Postgres │ │Redis │ │NATS  │  │ChromaDB  │  ← 有状态层
              │(主从)   │ │      │ │      │  │          │
              └─────────┘ └──────┘ └──┬───┘  └──────────┘
                                      │
              ┌───────────────────────┼────────────┐
              │                       │            │
              ▼                       ▼            ▼
       ┌────────────┐         ┌────────────┐  ┌──────────┐
       │ai-worker #1│         │ai-worker #2│  │knowledge │
       │gRPC+NATS   │         │gRPC+NATS   │  │consumer  │  ← 消费者拆分
       │consumer    │         │consumer    │  │          │
       └────────────┘         └────────────┘  └──────────┘

关键变化:
  1. Go API: 无状态设计 → 多副本水平扩展
  2. Python Worker 拆分:
     - gRPC Server (GenerateSQL) → 无状态，独立扩展
     - Agent Consumer (LangGraph) → NATS Queue Group 负载均衡
     - Knowledge Consumer (ChromaDB) → 独立进程
  3. NATS Queue Group: 同一 subject 多消费者自动分发
  4. mTLS: gRPC、NATS、Redis 全链路加密
```

**适用阶段：** 灰度发布 / 多用户上线前

### 5.3 方案三：协议统一（长期愿景）

**目标：** 消灭 HTTP → gRPC 混合，全链路统一 gRPC。

```protobuf
// 新增：Python → Go 反向调用服务
service HubInternalService {
  rpc ExecuteSQL(ExecuteSQLRequest) returns (ExecuteSQLResponse);
  rpc ReportRunStep(ReportRunStepRequest) returns (ReportRunStepResponse);
  rpc CompleteTask(CompleteTaskRequest) returns (CompleteTaskResponse);
  rpc UpdateDocStatus(UpdateDocStatusRequest) returns (UpdateDocStatusResponse);
}
```

**收益：** 单一协议 + mTLS 统一认证 + gRPC 原生 deadline/retry/circuit-breaker + 自动代码生成。

**代价：** 需要改 Python NATS Consumer（异步 → gRPC client），重构量大（~5000 行），调试需要额外工具（grpcurl）。

**适用阶段：** 回调端点超过 10 个或需要双向流式通信时。

### 5.4 方案选择决策矩阵

| 维度 | 方案一：认证统一 | 方案二：架构重构 | 方案三：协议统一 |
|------|:---:|:---:|:---:|
| 改动量 | ~160 行 | ~2000 行 | ~5000 行 |
| 风险 | 低 | 中 | 高 |
| 核心收益 | 认证集中，消除重复 | 可扩展，可运维 | 架构一致性 |
| 统一认证中间件 | ✓ | ✓ | ✓ |
| gRPC TLS | ✓ | ✓ | ✓ |
| NATS Queue Group | ✗ | ✓ | ✓ |
| 消除 HMAC 重复 | ✓ (公共模块) | ✓ (公共模块) | ✓ (完全消灭) |
| Go 无状态多副本 | ✗ | ✓ | ✓ |
| Python 进程拆分 | ✗ | ✓ | ✓ |

**推荐路径：** 方案一（现在）→ 方案二（多用户上线前）→ 方案三作为架构北极星。

---

## 六、安全边界总览

| 边界 | 机制 | 位置 | 当前状态 | 生产建议 |
|------|------|------|----------|----------|
| 外部 API | JWT / API Key 双认证 | `middleware.Auth` | ✓ 已实现 | 预共享密钥轮换策略 |
| Token 吊销 | Redis JTI 黑名单 | `auth.IsRevoked` | ✓ 已实现 | 审计日志记录吊销事件 |
| 内部回调 | HMAC-SHA256 签名 | `middleware.InternalHMACAuth` | ✓ 已实现 | 合并入 UnifiedAuth，加 IP 白名单 |
| 角色控制 | Viewer < Operator < Admin | `middleware.RequireMinRole` | ✓ 已实现 | 添加自定义角色支持 |
| 速率限制 | Redis 滑动窗口 | `ratelimit.Allow` | ✓ 已实现 | /internal 路由补充限流 |
| SQL 注入防护 | 只读关键字检测 + 子查询包装 | `sqlrun` | ✓ 已实现 | 考虑 SQL 语法解析替代关键字匹配 |
| 密码存储 | AES-256-GCM 加密 | `crypto/aes.go` | ✓ 已实现 | 密钥定期轮换，未来接入 KMS |
| 路径遍历 | UUID 格式校验 | `reports.go` | ✓ 已实现 | 补充目录白名单 |
| 内部流量 | gRPC insecure | `worker/nl2sql.go` | ⚠ 明文 | **加 TLS** |
| 数据库密码 | 环境变量传入 | `docker-compose.yml` | ⚠ 无加密 | **Docker Secret / Vault** |

---

## 七、相关文件索引

| 文件 | 层级 | 说明 |
|------|:---:|------|
| `cmd/api/main.go` | 入口 | App 组装 + Server 启动 |
| `internal/handlers/handlers.go` | 入站 | 路由定义、PostMessage、InternalNL2SQL |
| `internal/middleware/middleware.go` | 入站 | Auth、InternalHMACAuth、RequireMinRole |
| `internal/ratelimit/limiter.go` | 入站 | 滑动窗口限流器 |
| `internal/ssebus/bus.go` | 入站 → 出站 | SSE 内存发布/订阅总线 |
| `internal/worker/nl2sql.go` | 出站 | gRPC 客户端封装（Go → Python） |
| `internal/async/task.go` | 出站 | 异步任务队列（DB + NATS） |
| `internal/sqlrun/run.go` | 守门员 | SQL 只读执行守门员 |
| `api/proto/nl2sql/v1/nl2sql.proto` | 出站 | gRPC 服务定义 |
| `services/ai/hub_ai/__main__.py` | Python | gRPC Server + NATS Consumer 启动 |
| `services/ai/orchestrator/consumer.py` | Python → 入站 | NATS 消费者，回调 Go `/internal/tasks` |
| `services/ai/orchestrator/graph.py` | Python → 入站 | LangGraph 编排图，回调 Go `/internal/nl2sql` |
| `services/ai/orchestrator/tracing.py` | Python → 入站 | 步骤追踪，回调 Go `/internal/runs` |
| `services/ai/orchestrator/knowledge_consumer.py` | Python → 入站 | 知识索引消费者，回调 Go `/internal/knowledge-docs` |

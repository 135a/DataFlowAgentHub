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

---

## 八、架构优劣评估

> 基于实际代码的诚实评估，不粉饰，不回避问题。

### 8.1 优点

#### A1. 安全边界是真正强制的，不是靠约定

```
❌ 常见做法：文档写"Python 不要直连数据库"，靠 Code Review 守住
✅ 本项目：Python 物理上根本没有数据库连接，所有 SQL 必经 sqlrun 守门员
```

`sqlrun.IsReadOnlySQL()` 用最粗暴但最可靠的关键字匹配挡住写操作——绕过它需要在 Go 代码里写一个新的 SQL 执行路径。这不是"信任 Python 不乱来"，而是"Python 想乱来也做不到"。

同样的，数据源密码在数据库里是 AES-256-GCM 密文，泄露数据库也拿不到明文。

#### A2. Go/Python 职责分离干脆利落

```
Go 侧：
  ✓ HTTP 路由、认证、限流、SSE           ← 网络基建
  ✓ 数据库连接池、迁移、Schema 发现       ← 数据基建
  ✓ SQL 执行、行限制、超时控制            ← 安全边界

Python 侧：
  ✓ LLM 调用、SQL 生成                   ← AI 能力
  ✓ LangGraph 多 Agent 编排              ← AI 编排
  ✓ RAG 文档分块、向量检索               ← 知识库
```

没有灰色地带："这段 AI prompt 逻辑写在 Go 还是 Python？"——答案永远是 Python。"这个 SQL 查询谁执行？"——答案永远是 Go。

#### A3. Docker Compose 一键启动，依赖自包含

```bash
docker compose -f deploy/compose/docker-compose.yml up -d
# → Postgres + Redis + NATS + ChromaDB + Go API + Python AI Worker 全栈启动
```

对演示、面试、快速验证极其友好。不需要外部 SaaS 依赖（除了可选 OpenAI API）。

#### A4. 路由决策显式化

```go
// 不靠文档约定"哪个端点需要 operator 角色"——代码即文档
r.With(middleware.RequireMinRole("operator")).Post("/data-sources", ...)
r.With(middleware.RequireMinRole("operator")).Post("/approvals/{id}/decide", ...)
```

中间件链的可读性很高——扫一眼 `Routes()` 就知道每个端点的认证和鉴权要求。

#### A5. 异步任务有降级设计

```go
// async/task.go
func (c *Client) EnqueueTask(...) {
    // 1. 写入 DB（主路径）
    c.DB.Exec(...)
    // 2. 发布 NATS（增强路径，失败不阻塞）
    if c.NATS != nil { c.NATS.Publish(...) }
    return taskID, nil  // NATS 挂了，任务仍可通过 DB 轮询被消费
}
```

关键中间件降级策略：
- **NATS 不可用**：任务仍在 DB，可被轮询消费
- **Redis 不可用**：限流器 fail-open 放行

#### A6. 技术栈选择务实

| 组件 | 选择 | 判断 |
|------|------|------|
| Go 路由 | chi（非 Gin） | chi 更轻，标准库兼容 |
| 数据库驱动 | pgx（非 database/sql） | 原生 PostgreSQL 支持，连接池性能好 |
| 消息队列 | NATS（非 Kafka） | 轻量级，Docker 单进程，MVP 够用 |
| 向量库 | ChromaDB（非 Pinecone） | 自托管，无外部依赖 |
| AI 编排 | LangGraph（非自研） | 减少造轮子，状态图模型适合 Agent 流程 |

---

### 8.2 缺点

#### B1. 异步路径的多跳往返

```
同步路径：Browser → Go → gRPC → Python → SQL → Go → Browser
          （清晰，2 跳）

异步路径：Browser → Go → NATS → consumer.py → LangGraph
            → nl2sql_node → HTTP /internal/nl2sql → Go
            → gRPC → Python → SQL → Go → 返回 → nl2sql_node
            → analysis_node → report_node
            → HTTP /internal/tasks/callback → Go → Browser
          （混乱，7+ 跳，多种协议切换）
```

`nl2sql_node` 从 Python 回调 Go 的 `/internal/nl2sql`，Go 又通过 gRPC 调用 Python 的 `GenerateSQL`——**绕了一圈回到同一个 Python 进程**。SQL 生成的 LLM 调用本该直接在 Python 本地完成。

#### B2. InternalNL2SQL 与 PostMessage 逻辑重复

```go
// InternalNL2SQL: ~60 行，其中 80% 跟 PostMessage 同步路径完全一样
func (a *App) InternalNL2SQL(w http.ResponseWriter, r *http.Request) {
    // 解析请求 → gRPC GenerateSQL → sqlrun 执行 → 返回结果
    // 跟 PostMessage 387-413 行逻辑重复
}
```

当两个 80% 相同的函数同时存在，维护者必须记得同时修改两边。

#### B3. 两条 Agent Pipeline 路径并存

```
路径 A（gRPC）:  Go → RunAgentPipeline RPC → Python 跑 LangGraph → 返回结果
路径 B（NATS）:  Go → NATS → consumer.py → Python 跑 LangGraph → HTTP 回调

相同点：都跑同一个 workflow_graph
不同点：入口协议不同，结果返回方式不同，错误处理不同
```

路径 B 经历了完整调试，而路径 A 在 `nl2sql.go` 只有一个客户端封装方法，没有消费端实现。"加一个新 Agent 节点，是改 gRPC 路径还是 NATS 路径？"

#### B4. Python 进程职责耦合

```python
# __main__.py — 一个进程做了三件事
def main():
    # 1. gRPC Server（GenerateSQL + RunAgentPipeline）
    server = grpc.server(...)

    # 2. NATS Agent Consumer（消费 hub.tasks.agent_pipeline）
    consumer_thread = threading.Thread(target=start_consumer)

    # 3. NATS Knowledge Consumer？——根本就没启动！
    # knowledge_consumer.py 的 run_knowledge_consumer() 没有被调用
```

gRPC Server 和 NATS Consumer 共享同一个进程，一个崩溃可能影响另一个。而且 `knowledge_consumer.py` 根本就没被 `__main__.py` 启动——知识索引的全链路实际上是断的。

#### B5. Python 侧缺乏抽象层

```python
# consumer.py — HMAC 签名逻辑
def sign_body(secret, body): ...

# knowledge_consumer.py — 完全相同
# tracing.py — 完全相同
# graph.py — 连函数都没提取，内联在 nl2sql_node 里
```

4 个文件实现同一套 HMAC 签名，修改密钥逻辑需要改 4 处。Python 代码组织处于"原型期"——功能能跑，但没有公共模块。

#### B6. 测试覆盖极度不均

```
Go 侧:
  auth/jwt_test.go        ✓ 完整
  sqlrun/run_test.go      ✓ 完整
  config/config_test.go   ✓ 完整
  schema/*_test.go        △ 基础（集成测试因缺 Postgres 而 skip）
  handlers/*_test.go      ✗ 整个 handlers 包零测试
  middleware/*_test.go    ✗ 零测试
  async/*_test.go         ✗ 零测试

Python 侧:
  tests/                  △ 基础（mock 测试）
  agents/、orchestrator/   ✗ 关键路径没有集成测试
```

最需要测试的 `PostMessage`——整个系统的核心路径，220 行代码，包含认证、限流、Schema 发现、SQL 执行、SSE 推送、路由分支——完全没有测试。

#### B7. 基础设施的维护负担

对于 MVP，依赖了 6 个中间件：

```
Postgres + Redis + NATS + ChromaDB + gRPC + Docker Compose
```

每个都需要版本管理、数据持久化、健康检查、日志采集、备份策略。`GET /health` 只查了 Postgres 和 Redis，NATS 和 ChromaDB 的健康状态完全不可见。对演示来说正好（一键启动），对长期维护来说偏重。

#### B8. 配置管理是环境变量的堆砌

```go
// config.go — 26 个环境变量
func Load() (*Config, error) {
    // HUB_JWT_SECRET, HUB_INTERNAL_HMAC_SECRET, HUB_DB_ENCRYPTION_KEY
    // HUB_DATABASE_URL, HUB_REDIS_ADDR, HUB_NATS_URL
    // HUB_LLM_BASE_URL, HUB_LLM_MODEL, HUB_LLM_API_KEY
    // HUB_SEED_EMAIL, HUB_SEED_PASSWORD
    // ... 还有 14 个
}
```

新增一个配置项需要碰 3 处：config.go 的 struct + Load() + .env.example。没有配置文件分层（开发/测试/生产），没有 secret 和 config 的区分。

---

### 8.3 评分矩阵

| 维度 | 评分 | 说明 |
|------|:---:|------|
| **安全设计** | ★★★★★ | 强制边界，零信任，密码加密，SQL 守门 |
| **职责分离** | ★★★★★ | Go/Python 边界清晰，没有灰色地带 |
| **演示体验** | ★★★★☆ | Docker 一键启动，但依赖 6 个中间件 |
| **代码组织（Go）** | ★★★★☆ | package 划分合理，中间件链清晰 |
| **代码组织（Python）** | ★★☆☆☆ | 缺少公共模块，重复代码多 |
| **核心路径清晰度** | ★★★☆☆ | 同步路径干净，异步路径绕圈 |
| **测试覆盖** | ★★☆☆☆ | Go 侧不均匀，handlers 零测试 |
| **可扩展性** | ★★☆☆☆ | 单进程耦合，但架构预留了扩展点 |
| **可维护性** | ★★★☆☆ | Go 侧好，Python 侧差，配置管理粗糙 |
| **运维复杂度** | ★★☆☆☆ | 6 个中间件，健康检查不全 |

### 8.4 一句话总结

**安全边界是亮点，异步路径是痛点。Go 侧整体比 Python 侧成熟一个层次。演示满分，生产还差两三个迭代。**

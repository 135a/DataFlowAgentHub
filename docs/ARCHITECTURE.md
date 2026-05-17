# DataFlowAgentHub 完整架构图

> 最后更新：2026-05-17（代码质量治理后）

---

## 一、系统全景拓扑

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                              浏览器 (React SPA)                                │
│                     Vite + TypeScript, nginx 反向代理                           │
└────────────────────────────┬─────────────────────────────────────────────────┘
                             │ HTTP/SSE
                             ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│                            nginx (web 容器)                                    │
│                /v1/* , /health , /version  →  proxy_pass api:8080             │
└────────────────────────────┬─────────────────────────────────────────────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│                              Go API :8080 (chi)                               │
│                                                                               │
│  ┌──────────────────────────── 中间件链 ────────────────────────────────────┐ │
│  │  Recoverer → Timeout(60s) → TraceID → RequestLog → Prometheus           │ │
│  │      → Auth (JWT) → RequireMinRole (viewer/operator/admin)              │ │
│  └──────────────────────────────────────────────────────────────────────────┘ │
│                                                                               │
│  ┌───────────────┐ ┌──────────┐ ┌────────────┐ ┌──────────┐ ┌─────────────┐ │
│  │   handlers/   │ │  ssebus/ │ │ nl2sqlexec/│ │  async/  │ │ grpcserver/ │ │
│  │  HTTP 路由     │ │ 内存事件  │ │ NL2SQL编排  │ │ 异步任务  │ │ gRPC 内部   │ │
│  │  + service    │ │  总线    │ │            │ │ NATS Push │ │ 服务 :9090  │ │
│  └───────┬───────┘ └──────────┘ └─────┬──────┘ └────┬─────┘ └──────┬──────┘ │
│          │                            │             │              │        │
│  ┌───────┴───────┐ ┌──────────┐ ┌────┴──────┐ ┌───┴──────┐ ┌────┴──────┐ │
│  │  middleware/   │ │ratelimit/│ │  sqlrun/  │ │ schema/  │ │ telemetry/│ │
│  │ JWT/日志/Trace │ │Redis限流  │ │ 只读SQL   │ │ 表发现   │ │Prometheus │ │
│  └───────────────┘ └──────────┘ └───────────┘ └──────────┘ └───────────┘ │
└───────┬───────┬──────────┬──────────┬────────────────┬────────────────────┘
        │       │          │          │                │
        ▼       ▼          ▼          ▼                ▼
   ┌────────┐ ┌──────┐ ┌──────┐ ┌──────────┐ ┌─────────────────┐
   │Postgres│ │Redis │ │ NATS │ │  Python  │ │  Go gRPC :9090  │
   │ :5432  │ │:6379 │ │:4222 │ │ :50051   │ │  (mTLS 可选)    │
   │ 主库    │ │缓存   │ │ 消息  │ │  NL2SQL  │ │  ← Python 回调  │
   └────────┘ └──────┘ └──┬───┘ │  gRPC    │ └─────────────────┘
                          │     └────┬─────┘
                          │ NATS 订阅 │
                          ▼          │
                   ┌──────────────────┴───────────────────────────┐
                   │            Python AI Worker                   │
                   │                                               │
                   │  ┌─────────────────────────────────┐         │
                   │  │ hub_ai/ (gRPC Server :50051)    │         │
                   │  │   GenerateSQL() → OpenAI LLM    │         │
                   │  └─────────────────────────────────┘         │
                   │                                               │
                   │  ┌─────────────────────────────────┐         │
                   │  │ orchestrator/consumer.py        │         │
                   │  │   订阅 NATS → LangGraph 图       │         │
                   │  │   结果通过 gRPC 回调 Go :9090    │         │
                   │  └─────────────────────────────────┘         │
                   │                                               │
                   │  ┌─────────────────────────────────┐         │
                   │  │ agents/ (分析 / 图表 / 报告)     │         │
                   │  │   pandas + matplotlib + openpyxl │         │
                   │  └─────────────────────────────────┘         │
                   │                                               │
                   │  ┌──────────────────┐ ┌──────────────────┐   │
                   │  │ rag/ (ChromaDB)  │ │ internal_client  │   │
                   │  │ 向量嵌入 + 语义搜索│ │ gRPC → Go :9090 │   │
                   │  └──────────────────┘ └──────────────────┘   │
                   └──────────────┬───────────────────────────────┘
                                  │
                                  ▼
                         ┌──────────────┐    ┌──────────────┐
                         │  ChromaDB    │    │ OpenAI API   │
                         │  :8000       │    │ (兼容接口)    │
                         └──────────────┘    └──────────────┘
```

---

## 二、双路径请求处理

```
                      用户消息 POST /v1/sessions/{id}/messages
                                  │
                                  ▼
                    ┌─────────────────────────────┐
                    │      关键词路由判断           │
                    │  "analyze" / "report" /     │
                    │  "分析" / "报告" / 图表?      │
                    └──────────┬──────────────────┘
                               │
              ┌────────────────┼────────────────┐
              │ 否 (简单查询)   │                 │ 是 (复杂分析)
              ▼                                 ▼
┌──────────────────────────┐        ┌──────────────────────────┐
│     同步路径 (NL2SQL)     │        │    异步路径 (Agent Pipeline) │
├──────────────────────────┤        ├──────────────────────────┤
│ 1. Rate limit 检查       │        │ 1-5 同上                  │
│ 2. Session 归属验证      │        │                          │
│ 3. resolveSchema()       │        │ 6. EnqueueTask()         │
│    (含外部数据源连接)      │        │    INSERT async_tasks    │
│ 4. Create run + SSE      │        │    NATS Publish          │
│    "run_started"          │        │                          │
│ 5. NL2SQLExec.Execute()  │        │ 7. HTTP 202 + task_id    │
│    → gRPC Python          │        │                          │
│    → IsReadOnlySQL()     │        └──────────┬───────────────┘
│    → sqlrun.QueryRows()  │                   │
│ 6. publishSyncResult()   │                   ▼
│    SSE: sql_generated    │        ┌──────────────────────────┐
│    SSE: result           │        │  Python NATS Consumer    │
│ 7. HTTP 200 {sql, rows}  │        │  LangGraph 图:           │
└──────────────────────────┘        │                          │
                                    │  NL2SQL → gRPC Go :9090  │
                                    │  Analysis (pandas + LLM) │
                                    │  Chart (matplotlib)      │
                                    │  Report (Markdown+Excel) │
                                    │                          │
                                    │  每节点: gRPC 步骤回调    │
                                    │  完成: gRPC TaskCallback  │
                                    │                          │
                                    │  Go gRPC Server:         │
                                    │  更新 async_tasks/runs   │
                                    │  SSE: "agent_step"       │
                                    │  SSE: "result"           │
                                    └──────────────────────────┘
```

---

## 三、Go 服务间通信 (gRPC)

```
┌─────────────────────┐              ┌─────────────────────────┐
│   Python AI Worker  │              │      Go API :9090       │
│                     │              │                         │
│ HubInternalClient   │──── gRPC ──▶│ HubInternalServiceServer │
│  (internal_client)  │  (mTLS opt) │                         │
│                     │              │ • TaskCallback()        │
│ • internal_nl2sql() │              │   更新 task + run 状态   │
│ • task_callback()   │              │                         │
│ • run_step_callback││              │ • RunStepCallback()     │
│                     │              │   记录步骤 + SSE 推送    │
│                     │              │                         │
│                     │              │ • InternalNL2SQL()      │
│                     │              │   GenerateSQL → 执行    │
└─────────────────────┘              └─────────────────────────┘
```

---

## 四、HTTP 路由表

### 公开路由

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/health` | PG + Redis 健康检查 |
| GET | `/version` | 版本信息 |
| GET | `/metrics` | Prometheus 指标 |
| POST | `/v1/auth/login` | 登录 → JWT（限流: 20/min/IP） |

### 认证路由 (`/v1/*`)

| 方法 | 路径 | 角色 | 说明 |
|------|------|------|------|
| GET | `/v1/sessions` | viewer | 会话列表 |
| POST | `/v1/sessions` | viewer | 创建会话 |
| GET | `/v1/sessions/{id}/messages` | viewer | 消息列表 |
| POST | `/v1/sessions/{id}/messages` | viewer | 发送消息（限流: 30/min/user） |
| GET | `/v1/sessions/{id}/stream` | viewer | SSE 事件流 |
| POST | `/v1/sessions/{id}/sse-token` | viewer | 获取 SSE Token |
| GET | `/v1/data-sources` | viewer | 数据源列表 |
| POST | `/v1/data-sources` | operator | 创建数据源（限流: 30/min/user） |
| POST | `/v1/data-sources/{id}/test` | operator | 测试连接 |
| PUT | `/v1/data-sources/{id}` | admin | 编辑数据源 |
| DELETE | `/v1/data-sources/{id}` | admin | 删除数据源 |
| GET | `/v1/workspaces/{id}/knowledge/docs` | operator | 知识文档列表 |
| POST | `/v1/workspaces/{id}/knowledge/docs` | operator | 上传知识文档 |
| POST | `/v1/auth/register` | admin | 创建用户（限流: 10/min/user） |
| GET | `/v1/users` | admin | 用户列表 |
| PUT | `/v1/users/{id}/role` | admin | 修改角色 |
| DELETE | `/v1/users/{id}` | admin | 删除用户 |
| GET | `/v1/tasks/{taskID}` | viewer | 异步任务状态 |
| GET | `/v1/runs/{runID}/report` | viewer | 下载报告 |
| POST | `/v1/data/upload` | operator | 文件上传导入 |
| POST | `/v1/data/suggest-table` | operator | AI 建表建议 |
| POST | `/v1/data/create-table` | operator | 确认建表 |
| GET | `/v1/schema/tables` | viewer | 表结构列表 |

---

## 五、数据存储

```
PostgreSQL (:5432)           Redis (:6379)            ChromaDB (:8000)
┌────────────────────┐  ┌─────────────────────┐  ┌───────────────────────┐
│ workspaces         │  │ schema:<ws>:<key>   │  │ workspace_<uuid>      │
│ users              │  │ → 表结构缓存 JSON    │  │ → 文档嵌入向量         │
│ data_sources       │  │ TTL: 300s           │  │ 1536维 (embed-3-small)│
│ sessions           │  └─────────────────────┘  └───────────────────────┘
│ messages           │
│ runs               │  ┌─────────────────────┐
│ async_tasks        │  │ rl:<userID>:<win>  │
│ knowledge_docs     │  │ → 限流计数 (排序集)   │
│ agent_run_steps    │  └─────────────────────┘
│ audit_events       │
└────────────────────┘  ┌─────────────────────┐
                         │ jwt:revoked:<jti>  │    NATS JetStream
                         │ → 吊销列表 TTL      │  ┌──────────────────────┐
                         └─────────────────────┘  │ hub.tasks.           │
                                                  │   agent_pipeline     │
                                                  │ hub.tasks.           │
                                                  │   knowledge_index    │
                                                  └──────────────────────┘
```

---

## 六、关键设计决策

| 维度 | 决策 | 更新 |
|------|------|------|
| **Go ↔ Python** | 双向 gRPC：Go→Python (:50051) NL2SQL 生成；Python→Go (:9090) 回调 | mTLS 可选 |
| **异步解耦** | NATS JetStream 消息队列 | 不变 |
| **实时推送** | SSE 内存总线，单实例，Logger 注入且每 10 次丢弃告警 | 优化 |
| **认证** | JWT (HS256) + RBAC (viewer/operator/admin) | 移除审批关卡 |
| **限流** | Redis 排序集滑动窗口，fail-open/fail-closed 可配置 | 扩展 |
| **SQL 安全** | 关键字阻断：INSERT/UPDATE/DELETE/DROP/EXECUTE/REPLACE/VACUUM/REINDEX/COPY | 强化 |
| **错误处理** | 关键路径 error 全部记录或返回，消除 ~25 处 `_ =` 静默丢弃 | 修复 |
| **前端** | React 18 + TypeScript + Vite，组件拆分：ChatPanel / SessionSidebar / DataManagementPanel / useSSE | 重构 |
| **可观测性** | Prometheus + W3C TraceContext + zap 结构化日志 | 不变 |
| **部署** | Docker Compose 一键 6 服务 | 不变 |

---

## 七、已知风险 / Edge Cases

| 风险 | 说明 |
|------|------|
| SSE 内存总线 | 单实例，不能跨副本扩容；Consumer 慢时 32-buffer 通道会丢失事件 |
| 限流 fail-open（默认） | Redis 故障时自动放行 |
| 外部数据源连接 | 每次 Schema 发现新建连接池，用完即弃 |
| Python gRPC 无 TLS（默认） | 内网使用，可通过 HUB_GRPC_* 启用 mTLS |
| 无 CJK 字体环境 | matplotlib 中文字符回退为 DejaVu Sans |

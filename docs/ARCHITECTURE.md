# DataFlowAgentHub 完整架构图

> 生成日期：2026-05-17

---

## 一、系统全景拓扑

```
┌──────────────────────────────────────────────────────────────────────────────────────┐
│                                  外部用户 / 浏览器                                       │
│                            React SPA (Vite + TypeScript)                              │
│                        http://localhost:${WEB_PORT:-80}                                 │
└────────────────────────────┬─────────────────────────────────────────────────────────┘
                             │ HTTP/SSE
                             ▼
┌──────────────────────────────────────────────────────────────────────────────────────┐
│                           nginx (web 容器)                                              │
│                  /v1/* , /health , /version  →  proxy_pass http://api:8080             │
└────────────────────────────┬─────────────────────────────────────────────────────────┘
                             │
                             ▼
┌────────────────────────────────────────────────────────────────────────────────────────────┐
│                                      Go API :8080                                           │
│  ┌──────────────────────────────────────────────────────────────────────────────────────┐  │
│  │                            chi 路由 + 中间件链                                          │  │
│  │  Recoverer → Timeout(60s) → TraceID → RequestLog → Prometheus → Auth → RequireMinRole │  │
│  └──────────────────────────────────────────────────────────────────────────────────────┘  │
│                                                                                             │
│  ┌──────────────────┐  ┌──────────────┐  ┌───────────┐  ┌─────────────┐  ┌──────────────┐ │
│  │    handlers/     │  │   worker/    │  │  sqlrun/   │  │   schema/   │  │   async/     │ │
│  │  HTTP 路由层     │  │  gRPC 客户端  │  │ 只读SQL执行│  │ 表结构发现   │  │ 异步任务队列  │ │
│  └────────┬─────────┘  └──────┬───────┘  └─────┬─────┘  └──────┬──────┘  └──────┬───────┘ │
│           │                   │                │               │               │         │
│  ┌────────┴─────────┐  ┌──────┴───────┐  ┌─────┴─────┐  ┌──────┴──────┐                  │
│  │    ssebus/       │  │  nl2sqlexec/ │  │ ratelimit/│  │  connector/ │                  │
│  │  内存事件总线     │  │ NL2SQL编排器  │  │ 限流(Redis)│  │ PG连接池封装 │                  │
│  └──────────────────┘  └──────────────┘  └───────────┘  └─────────────┘                  │
│                                                                                             │
│  ┌──────────┐  ┌──────────┐  ┌────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────┐    │
│  │  auth/   │  │ config/  │  │ seed/  │  │migrate/  │  │  llm/    │  │  otelsetup/  │    │
│  │JWT+吊销  │  │ 环境变量  │  │ 初始用户│  │ embed迁移 │  │OpenAI客户端│  │ OTel追踪     │    │
│  └──────────┘  └──────────┘  └────────┘  └──────────┘  └──────────┘  └──────────────┘    │
└───────────┬──────────┬──────────┬──────────┬──────────────────────────────────────────────┘
            │          │          │          │
            ▼          ▼          ▼          ▼
   ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────────┐
   │PostgreSQL│ │  Redis   │ │  NATS    │ │ Python ai-worker │
   │   :5432  │ │  :6379   │ │  :4222   │ │    :50051 gRPC   │
   │ 主数据库  │ │缓存/限流  │ │ 消息队列  │ │    AI 计算面     │
   └──────────┘ └──────────┘ └────┬─────┘ └────────┬─────────┘
                                  │                 │
                                  │ NATS订阅        │ HTTP 回调
                                  ▼                 ▼
                          ┌──────────────────────────────────────┐
                          │          Python ai-worker            │
                          │  ┌───────────────────────────────┐  │
                          │  │   gRPC Servicer (:50051)      │  │
                          │  │   Health / GenerateSQL /      │  │
                          │  │   RunAgentPipeline             │  │
                          │  └───────────────────────────────┘  │
                          │                                      │
                          │  ┌───────────────────────────────┐  │
                          │  │ LangGraph StateGraph           │  │
                          │  │ START → NL2SQL → Analysis     │  │
                          │  │           → Chart → Report     │  │
                          │  │ Checkpointer: SQLite           │  │
                          │  └───────────────────────────────┘  │
                          │                                      │
                          │  ┌─────────┐ ┌────────┐ ┌────────┐ │
                          │  │ Agents  │ │  RAG   │ │Consumers│ │
                          │  │分析/图表│ │ChromaDB│ │NATS订阅 │ │
                          │  │/报告    │ │向量检索 │ │回调Go API│ │
                          │  └─────────┘ └───┬────┘ └────────┘ │
                          └─────────────────┬────────────────────┘
                                            │
                                            ▼
                                   ┌──────────────┐    ┌──────────────┐
                                   │  ChromaDB    │    │ OpenAI API   │
                                   │  :8000       │    │ (兼容接口)    │
                                   │  向量数据库   │    │  LLM 服务    │
                                   └──────────────┘    └──────────────┘
```

---

## 二、请求处理双路径

```
                      用户消息 POST /v1/sessions/{id}/messages
                                  │
                                  ▼
                    ┌─────────────────────────────┐
                    │      关键词路由判断           │
                    │  "analyze" / "report" /     │
                    │  "分析" / "报告" / "图表"?    │
                    └──────────┬──────────────────┘
                               │
              ┌────────────────┼────────────────┐
              │ 否 (简单查询)   │                 │ 是 (复杂分析)
              ▼                                 ▼
┌──────────────────────────┐        ┌──────────────────────────┐
│     同步路径 (NL2SQL)     │        │    异步路径 (Agent Pipeline) │
├──────────────────────────┤        ├──────────────────────────┤
│                          │        │                          │
│ 1. Schema 发现           │        │ 1. Schema 发现           │
│    (Redis 缓存 /         │        │    (同上)                │
│     information_schema)  │        │                          │
│                          │        │ 2. INSERT async_tasks    │
│ 2. gRPC → Python         │        │    (status=queued)      │
│    GenerateSQL()         │        │                          │
│                          │        │ 3. NATS publish          │
│ 3. IsReadOnlySQL()       │        │    hub.tasks.            │
│    安全检查              │        │    agent_pipeline        │
│                          │        │                          │
│ 4. sqlrun.QueryRows()    │        │ 4. HTTP 202 + task_id   │
│    执行查询              │        │    + SSE "run_started"   │
│                          │        │                          │
│ 5. 存储 message + run    │        └──────────┬───────────────┘
│                          │                   │
│ 6. SSE 推送结果          │                   ▼
│                          │        ┌──────────────────────────┐
│ 7. HTTP 200 {sql, rows}  │        │  Python NATS Consumer    │
│                          │        │  接收任务 → 运行图        │
└──────────────────────────┘        ├──────────────────────────┤
                                    │                          │
                                    │ ┌──────────────────────┐ │
                                    │ │ 1. nl2sql_node       │ │
                                    │ │    HTTP → Go         │ │
                                    │ │    /internal/nl2sql   │ │
                                    │ │    (HMAC签名)         │ │
                                    │ ├──────────────────────┤ │
                                    │ │ 2. analysis_node     │ │
                                    │ │    pandas 统计分析     │ │
                                    │ │    + LLM 业务解读     │ │
                                    │ ├──────────────────────┤ │
                                    │ │ 3. chart_node        │ │
                                    │ │    matplotlib 图表    │ │
                                    │ │    → /tmp/reports/    │ │
                                    │ ├──────────────────────┤ │
                                    │ │ 4. report_node       │ │
                                    │ │    Markdown + Excel   │ │
                                    │ └──────────────────────┘ │
                                    │                          │
                                    │ HTTP POST                 │
                                    │ /internal/tasks/          │
                                    │ {id}/callback             │
                                    │ (HMAC签名)                │
                                    └──────────┬───────────────┘
                                               │
                                               ▼
                                    ┌──────────────────────────┐
                                    │ Go TaskCallback handler  │
                                    │ • 更新 async_tasks       │
                                    │ • 更新 runs 状态          │
                                    │ • 插入 assistant message │
                                    │ • SSE 推送 "result"      │
                                    └──────────────────────────┘
```

---

## 三、Go API 内部模块依赖图

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              cmd/api/main.go                            │
│  加载配置 → 连接外部 → 运行迁移 → 种子数据 → 启动服务                      │
└───────────────────────────────┬─────────────────────────────────────────┘
                                │
        ┌───────────────────────┼──────────────────────────┐
        ▼                       ▼                          ▼
┌──────────────┐     ┌──────────────────┐       ┌──────────────────┐
│   config/    │     │    migrate/      │       │     seed/        │
│ HUB_* 环境变量│     │ embed.FS SQL迁移 │       │ admin用户 +      │
│ 25个配置项    │     │ 4个迁移文件       │       │ 服务API用户       │
└──────────────┘     └──────────────────┘       └──────────────────┘
                                │
                                ▼
┌───────────────────────────────────────────────────────────────────────────┐
│                         handlers/App (核心依赖聚合)                         │
│                                                                           │
│  ┌──────────┐  ┌──────────┐  ┌───────────┐  ┌──────────┐  ┌──────────┐  │
│  │   DB     │  │  Redis   │  │ NL2SQL    │  │ SSEBus   │  │  NATS    │  │
│  │ *pgxpool │  │ *Client  │  │ gRPC客户端 │  │ 内存总线  │  │ *Conn    │  │
│  └──────────┘  └──────────┘  └─────┬─────┘  └──────────┘  └────┬─────┘  │
│                                    │                            │        │
│                          ┌─────────┴─────────┐        ┌────────┴──────┐ │
│                          │  nl2sqlexec/      │        │   async/      │ │
│                          │  Executor         │        │   Client      │ │
│                          │                   │        │               │ │
│                          │  GenerateSQL ─┐   │        │  EnqueueTask  │ │
│                          │       │       │   │        │  StartReaper  │ │
│                          │       ▼       │   │        │               │ │
│                          │  IsReadOnly   │   │        └───────────────┘ │
│                          │       │       │   │                          │
│                          │       ▼       │   │                          │
│                          │  QueryRows ───┼───┼──→ Postgres / 外部数据源  │
│                          └───────────────┘   │                          │
│                                              │                          │
│  ┌──────────┐  ┌───────────┐  ┌──────────┐  │  ┌──────────┐            │
│  │ schema/  │  │ ratelimit/│  │  llm/    │  │  │ telemetry│            │
│  │ 表发现    │  │ Redis限流 │  │ OpenAI   │  │  │Prometheus│            │
│  │ 缓存      │  │ fail-open│  │ 客户端    │  │  │/metrics  │            │
│  └──────────┘  └───────────┘  └──────────┘  │  └──────────┘            │
└─────────────────────────────────────────────┘                           │
        │                                                                 │
        ▼                                                                 │
┌──────────────────────────────────────────────────────────────────────┐  │
│                         middleware/ 中间件链                           │  │
│                                                                       │  │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐              │  │
│  │ TraceID  │  │RequestLog│  │  Auth    │  │ Require  │              │  │
│  │注入/提取  │  │ zap日志  │  │双认证模式 │  │ MinRole  │              │  │
│  │          │  │ 脱敏     │  │JWT/ApiKey│  │ RBAC     │              │  │
│  └──────────┘  └──────────┘  └────┬─────┘  └──────────┘              │  │
│                                   │                                   │  │
│                          ┌────────┴────────┐                          │  │
│                          │     auth/       │                          │  │
│                          │  Sign / Parse   │                          │  │
│                          │  Revoke(Redis)  │                          │  │
│                          └─────────────────┘                          │  │
└──────────────────────────────────────────────────────────────────────┘  │
```

---

## 四、Python AI Worker 内部架构

```
┌──────────────────────────────────────────────────────────────────────┐
│                     services/ai/ (Python)                            │
│                                                                      │
│  ┌────────────────────────────────────────────────────────────────┐ │
│  │                    hub_ai/__main__.py                          │ │
│  │  启动 gRPC Server :50051 + 2个 NATS 消费者守护线程             │ │
│  └──────────┬──────────────────────┬─────────────────────────────┘ │
│             │                      │                                │
│             ▼                      ▼                                │
│  ┌────────────────────┐  ┌──────────────────────────────────────┐  │
│  │  gRPC Servicer     │  │   NATS Consumers (daemon threads)    │  │
│  │                    │  │                                      │  │
│  │  • Health()        │  │  ┌────────────────────────────────┐  │  │
│  │  • GenerateSQL()   │  │  │ consumer.py                    │  │  │
│  │    → OpenAI LLM    │  │  │ 订阅: hub.tasks.agent_pipeline │  │  │
│  │    → 返回 SQL      │  │  │ 运行 LangGraph → HTTP 回调 Go  │  │  │
│  │                    │  │  └────────────────────────────────┘  │  │
│  │  • RunAgentPipeline│  │                                      │  │
│  │    → 启动 LangGraph│  │  ┌────────────────────────────────┐  │  │
│  └────────┬───────────┘  │  │ knowledge_consumer.py           │  │  │
│           │              │  │ 订阅: hub.tasks.knowledge_index │  │  │
│           ▼              │  │ 文档分块 → 嵌入 → ChromaDB      │  │  │
│  ┌────────────────────┐  │  │ 回调 PATCH /internal/knowledge  │  │  │
│  │ orchestator/graph  │  │  └────────────────────────────────┘  │  │
│  │                    │  └──────────────────────────────────────┘  │
│  │ StateGraph:        │                                            │
│  │                    │                                            │
│  │  START             │                                            │
│  │    │               │                                            │
│  │    ▼               │                                            │
│  │ ┌──────────┐       │         ┌──────────────────────────────┐  │
│  │ │nl2sql_node│      │         │      agents/                 │  │
│  │ │HTTP→Go API│      │         │                              │  │
│  │ └────┬─────┘       │         │  data_analysis_agent.py      │  │
│  │      │             │         │  • pandas describe()         │  │
│  │      ▼             │         │  • z-score 异常检测 (3σ)     │  │
│  │ ┌──────────────┐   │         │  • OpenAI LLM 业务解读        │  │
│  │ │条件路由       │   │         │                              │  │
│  │ │chart→图表节点 │   │         │  chart_agent.py              │  │
│  │ │analyze→分析  │   │         │  • 自动选图表类型(bar/line/pie)│  │
│  │ │report→报告   │   │         │  • matplotlib 渲染 PNG       │  │
│  │ └──┬──┬──┬────┘   │         │  • CJK 字体支持               │  │
│  │    │  │  │        │         │                              │  │
│  │    ▼  ▼  ▼        │         │  report_generation_agent.py   │  │
│  │ ┌──┐┌──┐┌──┐     │         │  • Markdown 报告生成          │  │
│  │ │分││图││报│     │         │  • Excel 导出 (openpyxl)       │  │
│  │ │析││表││告│     │         │  • 嵌入图表 PNG               │  │
│  │ └──┘└──┘└──┘     │         └──────────────────────────────┘  │
│  │    │  │  │        │                                            │
│  │    └──┴──┴────────│──→ END                                    │
│  │                   │                                            │
│  │ Checkpointer:     │         ┌──────────────────────────────┐  │
│  │  SQLite           │         │      rag/                    │  │
│  │  /data/langgraph/ │         │  knowledge_base.py           │  │
│  └───────────────────┘         │  • ChromaDB HTTP 客户端       │  │
│                                │  • Collection: workspace_<id>│  │
│  ┌────────────────────┐        │  • RecursiveCharacterSplitter│  │
│  │ orchestator/state  │        │  • OpenAI text-embedding-    │  │
│  │ AgentState TypedDict│       │    3-small                   │  │
│  │ • run_id            │        └──────────────────────────────┘  │
│  │ • user_input        │                                            │
│  │ • schema_context    │        ┌──────────────────────────────┐  │
│  │ • rag_context       │        │   orchestator/tracing.py     │  │
│  │ • sql, nl2sql_result│        │   report_run_step()          │  │
│  │ • analysis_summary  │        │   HTTP POST /internal/runs/  │  │
│  │ • final_report      │        │   {runID}/steps (HMAC)       │  │
│  │ • error             │        └──────────────────────────────┘  │
│  │ • chart_paths       │                                            │
│  └─────────────────────┘                                            │
└──────────────────────────────────────────────────────────────────────┘
```

---

## 五、认证体系

```
                        请求进入 /v1/*
                              │
                              ▼
                    Authorization: Bearer <jwt> (或 ?token= 查询参数)
                              │
                              ▼
                    ┌──────────────────────┐
                    │ JWT 解析 (HS256)      │
    │ 匹配 service    │           │ auth.Parse()          │
    │ api user        │           │                      │
    │ → admin 角色    │           │ 检查Redis吊销列表     │
    └────────┬────────┘           │ jwt:revoked:<jti>    │
             │                    └──────────┬───────────┘
             │                               │
             └───────────┬───────────────────┘
                         │
                         ▼
               ┌─────────────────────┐
               │ Claims 注入 Context  │
               │ UserID, WorkspaceID │
               │ Role, JTI           │
               └──────────┬──────────┘
                          │
                          ▼
               ┌─────────────────────┐
               │ RequireMinRole      │
               │ admin > operator >  │
               │ viewer              │
               └─────────────────────┘

         /internal/* 路由认证:
         ┌──────────────────────────────────┐
         │  InternalHMACAuth                │
         │  X-Hub-Signature: sha256=<hex>  │
         │  body = HMAC-SHA256(key, body)   │
         │  用于 Python → Go 服务间调用       │
         └──────────────────────────────────┘
```

---

## 六、数据存储全景

```
┌─────────────────────────────────────────────────────────────────────┐
│                        PostgreSQL (主数据库)                         │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌──────────┐  ┌──────────┐  ┌────────────┐  ┌───────────────┐    │
│  │workspaces│  │  users   │  │data_sources│  │   sessions    │    │
│  │          │  │          │  │            │  │               │    │
│  │ id       │  │ id       │  │ id         │  │ id            │    │
│  │ name     │  │email     │  │ workspace  │  │ workspace_id  │    │
│  │          │  │pwd_hash  │  │ host/port  │  │ user_id       │    │
│  │          │  │role      │  │ db/user    │  │ data_source_id│    │
│  │          │  │workspace │  │ password   │  │ title         │    │
│  └──────────┘  └──────────┘  └────────────┘  └───────┬───────┘    │
│                                                      │             │
│  ┌──────────┐  ┌──────────┐  ┌────────────┐  ┌───────┴───────┐    │
│  │ messages │  │   runs   │  │  approval  │  │  audit_events │    │
│  │          │  │          │  │  _tasks    │  │               │    │
│  │ session  │  │ session  │  │            │  │ action        │    │
│  │ role     │  │ status   │  │ action_type│  │ payload(JSONB)│    │
│  │ content  │  │ pending  │  │ status     │  │ actor_user_id │    │
│  │ (JSONB)  │  │ _reason  │  │ payload    │  │ workspace     │    │
│  └──────────┘  └──────────┘  └────────────┘  └───────────────┘    │
│                                                                     │
│  ┌──────────────┐  ┌──────────────┐  ┌────────────────────┐        │
│  │ async_tasks  │  │knowledge_docs│  │ agent_run_steps    │        │
│  │              │  │              │  │                    │        │
│  │ task_type    │  │ doc_type     │  │ run_id             │        │
│  │ status       │  │ chroma_doc_id│  │ step_index         │        │
│  │ payload(JSONB│  │ chunk_count  │  │ agent_name         │        │
│  │ result(JSONB)│  │ status       │  │ input/output_summary│       │
│  │ expires_at   │  │ content_hash │  │ error_message      │        │
│  │ nats_seq     │  │              │  │ duration_ms        │        │
│  └──────────────┘  └──────────────┘  └────────────────────┘        │
│                                                                     │
│  ┌──────────────────┐                                               │
│  │ schema_migrations│  (migrate 版本追踪表)                          │
│  └──────────────────┘                                               │
└─────────────────────────────────────────────────────────────────────┘

┌──────────────────────────────┐  ┌───────────────────────────────────┐
│    Redis (缓存 + 限流)        │  │      ChromaDB (向量数据库)         │
├──────────────────────────────┤  ├───────────────────────────────────┤
│                              │  │                                   │
│  schema:<wsID>:<sourceKey>   │  │  Collection: workspace_<uuid>     │
│  → 表结构缓存 (TTL 300s)     │  │                                   │
│                              │  │  文档分块嵌入向量                  │
│  rl:<userID>:<window>        │  │  维度: text-embedding-3-small     │
│  → 限流计数 (滑动窗口)        │  │  (1536维)                         │
│                              │  │                                   │
│  jwt:revoked:<jti>           │  │  语义搜索 top_k                    │
│  → JWT 吊销列表 (TTL)        │  │  用于 RAG 上下文中注入            │
│                              │  │                                   │
└──────────────────────────────┘  └───────────────────────────────────┘

┌──────────────────────────────────────────────────────────────────┐
│                    NATS JetStream                               │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Subject: hub.tasks.agent_pipeline                              │
│  Publisher:  Go (async/task.go EnqueueTask)                     │
│  Consumer:   Python (orchestrator/consumer.py)                  │
│  Payload:    task_id, session_id, run_id, user_message, schema  │
│                                                                  │
│  Subject: hub.tasks.knowledge_index                             │
│  Publisher:  Go (handlers/knowledge.go UploadKnowledgeDoc)      │
│  Consumer:   Python (orchestrator/knowledge_consumer.py)        │
│  Payload:    doc_id, workspace_id, content, doc_type, chunk_size│
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

---

## 七、部署视图 (Docker Compose)

```
┌──────────────────────────────────────────────────────────────────┐
│                    Docker Compose 服务编排                         │
├──────────────────────────────────────────────────────────────────┤
│                                                                   │
│  Network: hub-net (bridge)                                       │
│                                                                   │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │  web (nginx:alpine)                                         │ │
│  │  Port: ${WEB_PORT:-80}:80                                   │ │
│  │  Volumes: ./nginx.conf → /etc/nginx/conf.d/default.conf     │ │
│  │  Depends: api                                               │ │
│  │  Build arg: VITE_API_BASE_URL=""  (同源代理)                 │ │
│  └──────────────────────────┬──────────────────────────────────┘ │
│                             │ proxy_pass http://api:8080         │
│                             ▼                                    │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │  api (golang:1.24-alpine)                                   │ │
│  │  Port: 8080                                                 │ │
│  │  Depends: postgres, redis, ai-worker, nats                  │ │
│  │  Env: 所有 HUB_* 变量                                       │ │
│  │  Volume: ./deploy/compose/init → /docker-entrypoint-initdb.d│ │
│  └──────┬───────────┬──────────┬───────────┬───────────────────┘ │
│         │           │          │           │                      │
│         ▼           ▼          ▼           ▼                      │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌──────────────────────────┐│
│  │postgres │ │  redis  │ │  nats   │ │      ai-worker           ││
│  │16-alpine│ │7-alpine │ │2.10-alp │ │      python:3.12-slim    ││
│  │  :5432  │ │  :6379  │ │ :4222   │ │      :50051              ││
│  │         │ │         │ │ :8222   │ │                          ││
│  │ Volume: │ │         │ │         │ │ Depends: chroma, nats    ││
│  │ pgdata  │ │         │ │ Volume: │ │                          ││
│  │         │ │         │ │natsdata │ │ Env: OPENAI_*, CHROMA_*, ││
│  │ Init:   │ │         │ │         │ │      NATS_URL, etc.      ││
│  │01_demo  │ │         │ │         │ │                          ││
│  │.sql     │ │         │ │         │ │ Volume: langgraph_data   ││
│  └─────────┘ └─────────┘ └─────────┘ └──────────┬───────────────┘│
│                                                  │                 │
│                                                  ▼                 │
│                               ┌──────────────────────────────────┐│
│                               │  chroma (chromadb/chroma:0.5.20)││
│                               │  :8000                           ││
│                               │  Volume: chromadata              ││
│                               └──────────────────────────────────┘│
│                                                                   │
│  Volumes (持久化):                                                 │
│    pgdata, chromadata, natsdata, langgraph_data                   │
└──────────────────────────────────────────────────────────────────┘
```

---

## 八、HTTP 路由表

| 方法 | 路径 | 认证 | 最小角色 | 处理器 |
|------|------|------|----------|--------|
| GET | `/health` | 无 | — | 健康检查 (PG + Redis) |
| GET | `/version` | 无 | — | 版本信息 |
| POST | `/v1/auth/login` | 无 | — | 用户名/密码 → JWT |
| GET | `/v1/sessions` | JWT/ApiKey | viewer | 列会话 |
| POST | `/v1/sessions` | JWT/ApiKey | viewer | 创建会话 |
| GET | `/v1/sessions/{id}/messages` | JWT/ApiKey | viewer | 列消息 (含 run_steps) |
| POST | `/v1/sessions/{id}/messages` | JWT/ApiKey | viewer | 发送消息 (核心 NL2SQL) |
| GET | `/v1/sessions/{id}/stream` | JWT/ApiKey | viewer | SSE 实时流 |
| POST | `/v1/sessions/{id}/sse-token` | JWT/ApiKey | viewer | SSE Token |
| GET | `/v1/data-sources` | JWT/ApiKey | viewer | 列数据源 |
| POST | `/v1/data-sources` | JWT/ApiKey | operator | 创建数据源 |
| POST | `/v1/data-sources/{id}/test` | JWT/ApiKey | operator | 测试连接 |
| GET | `/v1/approvals` | JWT/ApiKey | operator | 列审批任务 |
| POST | `/v1/approvals/{id}/decide` | JWT/ApiKey | operator | 审批决策 |
| GET | `/v1/workspaces/{id}/knowledge/docs` | JWT/ApiKey | operator | 列知识文档 |
| POST | `/v1/workspaces/{id}/knowledge/docs` | JWT/ApiKey | operator | 上传知识文档 |
| GET | `/v1/runs/{runID}/report` | JWT/ApiKey | viewer | 下载报告 |
| GET | `/v1/tasks/{taskID}` | JWT/ApiKey | viewer | 任务状态 |
| POST | `/internal/tasks/{id}/callback` | HMAC | — | Python 回调 |
| POST | `/internal/runs/{id}/steps` | HMAC | — | 步骤回调 |
| POST | `/internal/nl2sql` | HMAC | — | 内部 NL2SQL |
| PATCH | `/internal/knowledge-docs/{id}/status` | HMAC | — | 文档状态更新 |

---

## 九、中间件链 (按执行顺序)

```
请求入口
  │
  ▼
[1] chi.Recoverer              — panic 恢复
  │
  ▼
[2] chi.Timeout(60s)           — 请求超时
  │
  ▼
[3] middleware.TraceID          — X-Trace-Id 注入/提取
  │
  ▼
[4] middleware.RequestLog       — zap 结构化日志 (脱敏)
  │
  ▼
[5] telemetry.PrometheusMiddleware — 指标记录
  │
  ▼
[6] 路由匹配
  │  ├── /metrics → promhttp.Handler (跳过认证)
  │  ├── /health, /version, /v1/auth/login → 公开处理器
  │  └── /v1/*  → 下文继续
  ▼
[7] middleware.Auth (for /v1/*)
  │   • 检查 Bearer JWT (header 或 ?token 查询参数)
  │   • auth.Parse JWT 验证 HS256
  │   • auth.IsRevoked Redis 吊销检查
  │   • Claims 注入 context
  ▼
[8] middleware.RequireMinRole (for 受保护路由)
      admin > operator > viewer

/internal/* 独立认证链:
[1]-[5] 同上
  │
  ▼
[6] middleware.InternalHMACAuth — 验证 X-Hub-Signature: sha256=<hex>
```

---

## 十、环境变量配置总表

### Go API 配置 (`HUB_*`)

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `HUB_JWT_SECRET` | **(必填)** | JWT 签名密钥 |
| `HUB_INTERNAL_HMAC_SECRET` | **(必填)** | 服务间 HMAC 密钥 |
| `HUB_DB_ENCRYPTION_KEY` | **(必填)** | AES-256-GCM 32字节十六进制密钥 |
| `HUB_HTTP_ADDR` | `:8080` | HTTP 监听地址 |
| `HUB_DATABASE_URL` | `postgres://hub:hub@localhost:5432/hub?sslmode=disable` | PostgreSQL 连接串 |
| `HUB_REDIS_ADDR` | `localhost:6379` | Redis 地址 |
| `HUB_SEED_EMAIL` | `admin@demo.local` | 初始管理员邮箱 |
| `HUB_SEED_PASSWORD` | `changeme` | 初始管理员密码 |
| `HUB_GLOBAL_API_KEY` | (空) | 全局 API Key (admin 角色) |
| `HUB_NL2SQL_TARGET` | `localhost:50051` | gRPC worker 地址 |
| `HUB_LLM_BASE_URL` | `https://api.openai.com/v1` | LLM API 基础 URL |
| `HUB_LLM_MODEL` | `gpt-4o-mini` | LLM 模型 |
| `HUB_LLM_API_KEY` | (空) | LLM API 密钥 |
| `HUB_LLM_TIMEOUT` | `60s` | LLM HTTP 超时 |
| `HUB_APPROVAL_TTL` | `24h` | 审批任务 TTL |
| `HUB_QUERY_MAX_ROWS` | `500` | SQL 结果最大行数 |
| `HUB_QUERY_TIMEOUT` | `30s` | SQL 查询超时 |
| `HUB_SCHEMA_CACHE_TTL` | `300s` | Schema 缓存 TTL |
| `HUB_SCHEMA_MAX_TABLES` | `50` | 最大发现表数 |
| `HUB_SCHEMA_MAX_COLUMNS_PER_TABLE` | `100` | 每表最大列数 |
| `HUB_REPORTS_DIR` | `<os.TempDir>/hub-reports/` | 报告文件目录 |
| `HUB_OTEL_EXPORTER_ENDPOINT` | (空) | OpenTelemetry OTLP 端点 |
| `HUB_ENV` | `development` | 环境名称 |
| `HUB_NATS_URL` | `nats://localhost:4222` | NATS 服务器 URL |

### Python Worker 配置

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `WORKER_GRPC_ADDR` | `0.0.0.0:50051` | gRPC 监听地址 |
| `OPENAI_API_KEY` | (空) | OpenAI API 密钥 |
| `OPENAI_BASE_URL` | `https://api.openai.com/v1` | OpenAI 基础 URL |
| `OPENAI_MODEL` | `gpt-4o-mini` | OpenAI 模型 |
| `LOG_LEVEL` | `INFO` | 日志级别 |
| `CHROMA_HOST` | `localhost` | ChromaDB 主机 |
| `CHROMA_PORT` | `8000` | ChromaDB 端口 |
| `NATS_URL` | `nats://localhost:4222` | NATS URL |
| `HUB_API_INTERNAL_URL` | `http://api:8080` | Go API 内部 URL |
| `HUB_INTERNAL_HMAC_SECRET` | `dev-hmac-secret-change-me` | HMAC 密钥 |
| `LANGGRAPH_DB_PATH` | `/data/langgraph/checkpoints.db` | LangGraph 检查点路径 |

---

## 十一、核心设计决策速览

| 维度 | 决策 |
|------|------|
| **Go ↔ Python 通信** | gRPC (不安全明文通道，MVP 阶段) |
| **异步解耦** | NATS JetStream 消息队列 |
| **实时推送** | SSE (内存总线，无持久化，单实例) |
| **认证** | 双模式：用户 JWT (HS256) + 服务间 HMAC-SHA256 |
| **数据库** | PostgreSQL 16 主库 + Redis 7 缓存/限流 + ChromaDB 向量检索 |
| **AI 编排** | LangGraph StateGraph，SQLite 检查点持久化 |
| **限流** | Redis 排序集滑动窗口，Redis 不可用时 fail-open |
| **SQL 安全** | 关键字阻断写操作 (INSERT/UPDATE/DELETE/DROP 等) |
| **配置** | 全部环境变量注入 (`HUB_*` 前缀) |
| **可观测性** | Prometheus 指标 + W3C TraceContext 传播 + 结构化 JSON 日志 |
| **前端** | React 18 + TypeScript + Vite，nginx 反向代理到 Go API |
| **部署** | Docker Compose 一键启动 6 个服务 |

---

## 十二、已知风险 / Edge Cases

| 风险 | 说明 |
|------|------|
| SSE 内存总线 | 无持久化，不能跨副本扩容；Consumer 慢时 32-buffer 通道会丢失事件 |
| fail-open 模式 | 限流器和 JWT 吊销在 Redis 故障时自动放行 (MVP 策略) |
| 外部数据源连接 | 每次 Schema 发现使用单连接池，用完即弃 |
| 无查询取消 | 外部数据源查询只能通过 context timeout 取消 |
| 无 LLM API Key | 回退到 `SELECT 1 AS ok` 等 demo SQL |
| 无 CJK 字体环境 | matplotlib 图表中的中文字符回退为 DejaVu Sans (豆腐块) |
| Python gRPC 无 TLS | 无加密，仅限内网使用 |
| NATS 连接失败 | Go API 非致命，任务在 DB 中排队 |

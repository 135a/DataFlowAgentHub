# DataFlowAgentHub 面试准备分析

> 生成日期: 2026-05-16 | 基于 commit `0fdb7ca`
> 目标岗位：后端 / AI Agent 开发

---

## 一、项目当前输出形态

### 同步路径（简单 NL2SQL）

```
用户输入 → Go API → gRPC Python GenerateSQL → Go 执行 SQL → 返回
{
  "run_id": "...",
  "sql": "SELECT COUNT(*) FROM demo_sales",
  "rows": [{"count": 3}]
}
```

前端渲染：SQL 文本 + `ResultTable` 数据表格（HTML table）。

### 异步路径（Multi-Agent，勾选"深度分析"）

```
用户输入 → Go API → NATS → Python LangGraph
  ├── nl2sql_node    → 回调 Go /internal/nl2sql → 拿到 SQL + rows
  ├── analysis_node  → pandas 统计分析 + LLM 业务摘要
  └── report_node    → Markdown 报告 + Excel 导出

前端渲染：中间步骤列表 + SQL 文本 + 数据表格 + Markdown 报告文本
```

### 当前没有的东西

- **没有图表/可视化**：前端只有一个 HTML 表格组件，无任何 chart 库
- `data_analysis_agent.py` 做了均值、标准差、异常检测，但输出是纯文本
- `report_generation_agent.py` 生成的是 Markdown + Excel，不是可视化

---

## 二、架构总览

```
┌──────────────────────────────────────────────────────────────────┐
│                        当前架构                                    │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────┐    HTTP/SSE     ┌──────────┐    gRPC     ┌───────┐│
│  │ React SPA │◄──────────────►│  Go API  │◄──────────►│Python ││
│  │ (Vite)   │   JWT + HMAC    │  :8080   │            │:50051 ││
│  └──────────┘                 └────┬─────┘            └───┬───┘│
│                                    │                      │     │
│                         ┌──────────┼──────────┐    ┌──────┼───┐│
│                         │          │          │    │NATS      ││
│                    ┌────┴───┐ ┌───┴────┐ ┌───┴──┐ │consumer  ││
│                    │Postgres│ │ Redis  │ │Chroma│ │(pipeline)││
│                    └────────┘ └────────┘ └──────┘ └──────────┘│
│                                                                  │
│  关键路径：                                                       │
│  同步：用户 → Go → gRPC(Python LLM) → Go(sqlrun) → 用户          │
│  异步：用户 → Go → NATS → Python(LangGraph) → HTTP回调 → Go → SSE│
│  知识：用户 → Go → NATS → Python(Chroma) → HTTP回调 → Go          │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

### Go 代码布局（`internal/`）

| 包 | 职责 | 测试 |
|---|---|---|
| `handlers/` | HTTP 层，App 结构体 + Routes() | 无 |
| `middleware/` | TraceID、双认证、RBAC、请求日志 | 无 |
| `config/` | 26 个环境变量加载 | 有 |
| `auth/` | JWT 签发/解析/吊销 | 有 |
| `worker/` | gRPC 客户端（NL2SQL） | 无 |
| `ssebus/` | 内存 SSE 发布/订阅 | 无 |
| `sqlrun/` | 只读 SQL 执行器 | 有 |
| `schema/` | information_schema 查询 + Redis 缓存 | 有 |
| `async/` | NATS 异步任务队列 + reaper | 无 |
| `llm/` | OpenAI 兼容 HTTP 客户端 | 无 |
| `connector/` | PostgreSQL 连接池封装 | 无 |
| `crypto/` | AES-256-GCM + HMAC | 无 |
| `ratelimit/` | Redis 滑动窗口限流 | 无 |
| `telemetry/` | Prometheus metrics | 无 |
| `otelsetup/` | OpenTelemetry TracerProvider | 无 |
| `migrate/` | 嵌入式 SQL 迁移 | 无 |
| `seed/` | 默认管理员创建 | 无 |

### Python 代码布局（`services/ai/`）

| 模块 | 职责 | 测试 |
|------|------|------|
| `hub_ai/__main__.py` | gRPC 服务端 + NATS consumer 启动 | 无 |
| `orchestrator/graph.py` | LangGraph 工作流编排 | 无 |
| `orchestrator/consumer.py` | NATS agent_pipeline 消费者 | 无 |
| `orchestrator/knowledge_consumer.py` | NATS 知识索引消费者 | 无 |
| `orchestrator/state.py` | AgentState TypedDict | 无 |
| `orchestrator/tracing.py` | Run step 回调 | 无 |
| `agents/data_analysis_agent.py` | pandas 统计 + LLM 摘要 | 有 |
| `agents/report_generation_agent.py` | Markdown + Excel 报告 | 无 |
| `rag/knowledge_base.py` | ChromaDB 分块/嵌入/搜索 | 有 |

---

## 三、代码质量评估

### 做得好的

1. **Go/Python 安全边界清晰**：Python 端无数据库访问，SQL 执行被 Go 端 `sqlrun` 强制只读把关
2. **双认证体系**：JWT（用户）+ HMAC（服务间），区分明确
3. **中间件链合理**：`Recoverer → Timeout → TraceID → RequestLog → Prometheus → Auth → RBAC`
4. **Graceful degradation**：NATS 不可用时异步任务仍可写入 DB，Redis 不可用时 auth 不放行但限流失效放行
5. **可观测性骨架齐全**：Prometheus metrics + OTel tracing + 结构化日志
6. **凭证加密**：AES-256-GCM 加密数据源密码
7. **`docs/GATEWAY_ARCHITECTURE.md`**：680 行的诚实架构分析，包括安全边界图和三阶段演进路径

### 需要改进的

| 严重程度 | 问题 | 位置 |
|---------|------|------|
| P0 | **知识库消费者未启动** | `__main__.py` 只启动了 agent pipeline consumer，`knowledge_consumer` 从未 import |
| P1 | **HMAC 签名代码重复 4 次** | `graph.py`、`consumer.py`、`knowledge_consumer.py`、`tracing.py` |
| P1 | **InternalNL2SQL 与 PostMessage 80% 重复** | `handlers/handlers.go:566-613` vs `:270-414` |
| P1 | **consumer.py headers 可能未定义** | `consumer.py:62` headers 在 try 块内初始化，异常早于赋值时 except 块 NameError |
| P2 | **SSE 总线不持久、不可扩展** | `ssebus/bus.go` 内存 map+channel，单进程 |
| P2 | **LangGraph checkpointer 用 MemorySaver** | 进程重启丢失所有检查点 |
| P2 | **gRPC RunAgentPipeline 定义但未使用** | proto 定义了但实际走 NATS |
| P2 | **`migrations/` 与 `internal/migrate/` 重复** | 两套迁移文件，维护者不知道该改哪个 |
| P2 | **`pkg/` 目录为空** | 仅有 .gitkeep |
| P3 | **无 CI/CD** | 没有 GitHub Actions 或其他 CI 配置 |

---

## 四、同步/异步双路径设计分析

### 4.1 两条路径的本质

```
用户输入 "how many rows in demo_sales"

═══════════════════════════════════════════════════════════════
同步路径（当前默认）
═══════════════════════════════════════════════════════════════

  用户 ──POST──▶ Go ──gRPC──▶ Python LLM (生成SQL)
                  │                │
                  │ ◀──────────────┘
                  │
                  │ sqlrun (只读执行)
                  │
                  ▼
              立即返回 200: {sql, rows}

  延迟: ~1-2s (一次 gRPC 往返)
  输出: SQL + 数据表格
  协议: gRPC

═══════════════════════════════════════════════════════════════
异步路径（勾选"深度分析" 或 含"分析/报告"关键词）
═══════════════════════════════════════════════════════════════

  用户 ──POST──▶ Go ──NATS──▶ Python Consumer
      ◀──202──              │
                │           └──▶ LangGraph
                │                    │
                │    ┌───────────────┤
                │    │ nl2sql_node  │──HTTP──▶ Go /internal/nl2sql
                │    │              │    │       └── gRPC GenerateSQL
                │    │              │◀───┘       └── sqlrun
                │    ├───────────────┤
                │    │ analysis     │ pandas + LLM 摘要
                │    ├───────────────┤
                │    │ report       │ Markdown + Excel
                │    └───────────────┘
                │           │
                │           ▼ HTTP 回调 Go ──▶ SSE ──▶ 前端

  延迟: ~5-15s (NATS + LangGraph + 多次 HTTP 往返)
  输出: SQL + 数据表格 + 分析摘要 + Markdown 报告 + Agent 步骤追踪
  协议: NATS + HTTP 回调

═══════════════════════════════════════════════════════════════
当前问题
═══════════════════════════════════════════════════════════════

  InternalNL2SQL (异步路径中 Python 回调的那一段) 和 PostMessage
  同步路径第 388-410 行，核心逻辑完全一样：

      都是: gRPC GenerateSQL → sqlrun.QueryRows → 返回 {sql, rows}

  当前是两份独立代码，约 80% 重复。
```

### 4.2 方案对比

#### 方案 A：都保留，抽公共函数消除重复（推荐）

```
                   ┌──────────────────────────┐
                   │  App.executeNL2SQL()     │ ← 新增方法
                   │  - gRPC GenerateSQL      │
                   │  - sqlrun.QueryRows      │
                   │  - 返回 sql + rows       │
                   └──────┬───────────────────┘
                          │
          ┌───────────────┼───────────────┐
          │               │               │
    同步 PostMessage   InternalNL2SQL   未来可能的
    (直接返回给用户)   (供 Python 回调)   其他调用方
```

| 维度 | 评估 |
|------|------|
| 优点 | 同步秒回（~1s），异步深度分析（~10s），面试时两者都能演示，形成对比 |
| 缺点 | 需抽公共函数消除重复，约半天工作量 |
| 面试叙事 | "我根据场景做了执行路由——简单查询走快速通道，复杂分析走 Agent 管道，这是一种延迟与深度之间的设计权衡" |
| gRPC | `GenerateSQL` RPC 保留有意义，是 Python Worker 的核心接口 |

#### 方案 B：统一走异步，删掉同步路径和 gRPC GenerateSQL

```
所有请求 → LangGraph → nl2sql_node → 结果
                            │
            简单查询在此结束（不加 analysis/report 节点）
            复杂查询继续走 analysis + report
```

| 维度 | 评估 |
|------|------|
| 优点 | 架构更统一，所有请求走同一条代码路径 |
| 缺点 | 简单查询从 1s 变 5s+，面试体验下降；gRPC 失去存在价值 |
| 面试叙事 | "我统一了执行路径，所有请求都经过 Agent 编排"（叙事单一，没有对比） |

### 4.3 推荐结论：方案 A

**理由：**

1. **面试演示效果好**——先演示同步"秒回"让人觉得很流畅，再勾选"深度分析"展示 Agent 全流程，两者形成鲜明对比
2. **改动最小**——抽一个 `executeNL2SQL` 方法，两个 handler 各调一下，半天搞定
3. **面试叙事清晰**——"这是两种执行模式的设计权衡：低延迟 vs 深度分析"，展示了架构决策能力
4. **gRPC 保留有意义**——`GenerateSQL` 是 Python Worker 唯一对外暴露的 gRPC 接口，去掉后 gRPC 就没有存在价值了

**重构后的变化：**

| 文件 | 变化 |
|------|------|
| `internal/handlers/handlers.go` | 新增 `App.executeNL2SQL()` 方法，`PostMessage` 和 `InternalNL2SQL` 都调用它 |
| `internal/handlers/handlers.go` PostMessage | 同步路径从 ~40 行缩减到 ~15 行 |
| `internal/handlers/handlers.go` InternalNL2SQL | 从 ~50 行缩减到 ~20 行 |

**面试时的叙事脚本：**

> "这个系统有两条执行路径。当用户问简单问题时——比如'有多少行数据'——
> 走同步快速通道，gRPC 调用 LLM 生成 SQL 后直接执行并返回，延迟在 1 秒左右。
> 当用户问复杂问题——比如'分析销售趋势并生成报告'——系统会自动路由到
> LangGraph 多 Agent 管道，由 nl2sql、analysis、report 三个 Agent 协作完成，
> 用户通过 SSE 实时看到每个 Agent 的执行状态。这是延迟和深度之间的权衡。"

---

## 五、测试覆盖率热力图

```
                   有测试    无测试
handlers/           ░░░░     ██████████  ← 最关键的 658 行 0 覆盖
middleware/         ░░░░     ██████
async/              ░░░░     ████
worker/             ░░░░     █████
crypto/             ░░░░     ████████
llm/                ░░░░     ████████
auth/               ██████   ░░ (缺 revoke)
config/             ██████   ░░
sqlrun/             ████████ ░░
schema/             █████    ░░░░
```

**Go 测试仅覆盖工具包，HTTP 层零覆盖。Python 测试仅覆盖 agent 和 rag 的单元测试。**

---

## 六、面试场景分析

### 面试官类型与评分侧重

| 面试官背景 | 核心关注点 | 项目匹配度 |
|-----------|-----------|-----------|
| **AI Agent 开发** | LangGraph 编排、Prompt 工程、RAG 管道、Agent 可观测性 | ★★★★☆ 骨架好，需打磨 |
| **后端/架构** | Go 分层设计、安全边界、双认证、graceful degradation | ★★★★☆ 设计清晰 |
| **全栈** | 端到端数据流、SSE 实时推送、前后端协作 | ★★★☆☆ 前端较简陋 |
| **创业公司** | 技术选型理由、架构演进路径、MVP 快速迭代 | ★★★★☆ GATEWAY_ARCHITECTURE.md 加分 |

### 面试官大概率会走的冒烟流程

```
1. docker compose up -d                     → 必须一把起来
2. curl /health                             → 必须所有依赖 ok
3. curl /v1/auth/login                      → 必须拿到 token
4. 发消息 → 看到 SQL + 结果                  → 必须端到端通
5. 发 export 消息 → 看到审批                  → 必须通
6. 上传知识文档 → 状态变为 completed          → 必须通 ← 当前不通！
7. curl SSE 端点 → 收到事件                  → 必须通
8. (可选) 配 OPENAI_API_KEY → LLM 生成 SQL    → 加分项
```

### 面试官大概率会问的问题

1. "为什么选 Go 做控制面、Python 做计算面？" → 准备好架构决策理由
2. "LangGraph 图怎么设计的？为什么三个节点？" → 准备好拓扑设计理由
3. "Prompt 怎么设计的？迭代过几次？" → 准备好 prompt 设计思路
4. "Agent 出错了怎么处理？" → 展示 run steps 追踪 + reaper 机制
5. "你怎么保证 SQL 安全？" → 展示 `sqlrun.IsReadOnlySQL()` 守卫
6. "为什么这个代码重复了？" → 修掉 HMAC 和 InternalNL2SQL 的重复
7. "怎么测试的？核心流程有测试吗？" → 需要补 handler 集成测试

---

## 七、两周修补计划

### 第 1 周：修硬伤，确保能跑

| 天 | 任务 | 产出 |
|---|------|------|
| Day 1-2 | 修 knowledge consumer 启动问题；Python 代码质量重构（HMAC 去重、包结构）；端到端自测冒烟清单 | 冒烟清单全过 |
| Day 3-4 | 消除 Go 侧代码重复（InternalNL2SQL/PostMessage 抽公共函数）；修 consumer.py headers NameError | 代码审查能过关 |
| Day 5-6 | 新增 `chart_agent.py`：根据数据特征自动选择图表类型，生成 ECharts/Plotly JSON | Agent 工作流从 3 节点扩展为 4 节点 |
| Day 7 | 写 3 个 handler 集成测试（PostMessage 同步、PostMessage 异步、KnowledgeUpload） | 核心流程有测试 |

### 第 2 周：打磨，让面试官印象深刻

| 天 | 任务 | 产出 |
|---|------|------|
| Day 8-9 | 前端加 `recharts` 图表渲染 + ErrorBoundary + Skeleton 加载态 + TypeScript 类型化 | 前端体验合格 |
| Day 10 | 写 `docs/AGENT_DESIGN.md`（LangGraph 设计理由、prompt 迭代、状态管理） | 面试时可以展开讲 |
| Day 11 | 清理项目：删除 `migrations/`（与 `internal/migrate/` 重复）、删除或填充 `pkg/`、修所有 FIXME/TODO | 项目整洁 |
| Day 12 | Docker Compose 全栈测试（反复 up/down 幂等性）；修 Dockerfile 版本对齐 | 面试官 clone 即跑 |
| Day 13 | 录 3 分钟 demo 视频 + 写 README 里的演示场景说明 | 面试官没时间跑也能了解 |
| Day 14 | 缓冲日 / 补充压测数据 | 从容应对 |

### 如果要加图表 Agent

当前多 Agent 工作流：

```
nl2sql → analysis → report (纯文本)
```

建议扩展为：

```
nl2sql → analysis ─┬→ report (Markdown + Excel)
                    └→ chart  (ECharts/Plotly 配置 JSON)
```

新增 `services/ai/agents/chart_agent.py`：
- 根据 analysis 结果自动选择图表类型（柱状图/折线图/饼图/散点图）
- 生成前端可消费的 chart 配置 JSON
- 在 LangGraph 图中增加并行分支

前端用 `recharts`（轻量、React 原生）渲染图表。这在面试中是绝佳的谈资。

---

## 八、与 Java 短链接项目的互补策略

```
面试自我介绍：

"我有两个项目展示了两个技能方向——

短链接项目（Java）：高并发系统的工程能力，
  包括多级缓存、分库分表、API 限流熔断。

DataFlowAgentHub（Go + Python）：AI Agent 架构设计能力。
  我设计了 LangGraph 多 Agent 编排图、RAG 知识检索管道、
  Go 控制面 + Python 计算面的异构架构，
  以及 HMAC 双层认证的安全模型。

两个项目加在一起 = 全栈工程 + AI 系统设计"
```

| 维度 | Java 短链接 | DataFlowAgentHub |
|------|-----------|------------------|
| 技术栈 | Java 单体 | Go + Python 异构 |
| 业务复杂度 | 单一功能做深 | 多功能做广 |
| 核心亮点 | 高并发、短链算法、缓存 | AI Agent、多协议协作、安全设计 |
| 面试展示 | "我能写高效代码" | "我能设计 AI 系统" |

---

## 九、文件索引

| 文档 | 说明 |
|------|------|
| `README.md` | 快速入门（中文） |
| `CLAUDE.md` | Claude Code 项目指令 |
| `docs/DEPLOY.md` | 单机 Compose 部署 |
| `docs/GATEWAY_ARCHITECTURE.md` | 架构分析与三阶段演进 |
| `docs/ISSUES.md` | 41 个 issue 详细清单 |
| `docs/SMOKE_CHECKLIST.md` | 8 步面试演示冒烟清单 |
| `docs/SSE_PROXY.md` | Nginx/Caddy SSE 代理配置 |
| `docs/MIGRATIONS.md` | 迁移指南 |
| `Go+Python 一体化数据智能体平台完整提纲.md` | 原始设计文档 |
| `api/openapi/v1/openapi.yaml` | OpenAPI 3.0 规范 |
| `api/proto/nl2sql/v1/nl2sql.proto` | gRPC 服务定义 |

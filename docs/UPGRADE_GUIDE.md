# DataFlowAgentHub 升级改进指南

> 基于 2026-05-16 代码审查 | commit `0fdb7ca`
> 目标：两周内提升到可面试水平 | 岗位：后端 / AI Agent 开发

---

## 阅读指引

- **赶时间**：直接看 [P0](#p0--功能阻断必须立即修复) 和 [P1](#p1--代码质量消除重复)，修完项目就能跑完整流程
- **准备面试**：P0 + P1 + P2 全修，再加 [附录 A：面试叙事准备](#附录-a面试叙事准备)
- **完整两周计划**：按顺序全部执行
- **本文与 `INTERVIEW_PREP_ANALYSIS.md` 的关系**：分析文档侧重"为什么"，本文档侧重"怎么改"

---

## P0：功能阻断，必须立即修复

### P0-1：知识库消费者未启动

**现状**：`services/ai/hub_ai/__main__.py` 只启动了 agent pipeline consumer，`knowledge_consumer` 从未被启动。用户上传知识文档后状态永远 `pending`，RAG 索引流程完全断裂。

**位置**：`services/ai/hub_ai/__main__.py:194-206`

**当前代码**：
```python
# 9.3 Start NATS consumer in background
def start_consumer():
    import asyncio
    from orchestrator.consumer import run_consumer
    try:
        asyncio.run(run_consumer())
    except Exception as e:
        logging.error(f"Consumer died: {e}")

import threading
consumer_thread = threading.Thread(target=start_consumer, daemon=True)
consumer_thread.start()
logging.getLogger(__name__).info("Started NATS consumer thread")
```

**修改为**：
```python
# 9.3 Start NATS consumers in background
def start_agent_consumer():
    import asyncio
    from orchestrator.consumer import run_consumer
    try:
        asyncio.run(run_consumer())
    except Exception as e:
        logging.error(f"Agent consumer died: {e}")

def start_knowledge_consumer():
    import asyncio
    from orchestrator.knowledge_consumer import run_knowledge_consumer
    try:
        asyncio.run(run_knowledge_consumer())
    except Exception as e:
        logging.error(f"Knowledge consumer died: {e}")

import threading
agent_thread = threading.Thread(target=start_agent_consumer, daemon=True)
agent_thread.start()
knowledge_thread = threading.Thread(target=start_knowledge_consumer, daemon=True)
knowledge_thread.start()
logging.getLogger(__name__).info("Started NATS consumer threads (agent + knowledge)")
```

**验证**：`docker compose up -d` 后上传知识文档，5 秒内 `GET /v1/workspaces/{id}/knowledge/docs` 显示 `status: "completed"`。

**面试叙事**："两个 NATS 消费者分别处理 Agent 管道和知识索引，使用 daemon 线程随 gRPC server 一起启动。失败时 NATS JetStream 会重新投递消息。"

---

## P1：代码质量，消除重复

### P1-1：Python 侧 HMAC 签名重复（4 处 → 1 处）

**现状**：`sign_body()` 和 `make_headers()` 函数在以下文件中重复定义：

| 文件 | 行号 | 重复内容 |
|------|------|---------|
| `orchestrator/consumer.py` | 14-25 | `sign_body()` + `make_headers()` |
| `orchestrator/knowledge_consumer.py` | 14-25 | `sign_body()` + `make_headers()`（完全相同） |
| `orchestrator/tracing.py` | 11-14 | `sign_body()`，`make_headers` 逻辑内联在 30-33 |
| `orchestrator/graph.py` | 41-44 | HMAC 逻辑内联，无函数封装 |

**修复步骤**：

**Step 1** — 新建 `services/ai/hub_ai/shared.py`：

```python
"""Shared utilities for internal HTTP calls to Go API."""
import hmac
import hashlib


def sign_body(secret: str, body: bytes) -> str:
    """Return X-Hub-Signature header value for the request body."""
    mac = hmac.new(secret.encode(), body, hashlib.sha256)
    return f"sha256={mac.hexdigest()}"


def make_headers(secret: str, body_bytes: bytes) -> dict:
    """Build headers with HMAC signature for internal API calls."""
    return {
        "X-Hub-Signature": sign_body(secret, body_bytes),
        "Content-Type": "application/json",
    }
```

**Step 2** — 修改 `orchestrator/consumer.py`：
- 删除第 14-25 行的 `sign_body()` 和 `make_headers()` 函数定义
- 在文件顶部 import 区添加：`from hub_ai.shared import sign_body, make_headers`

**Step 3** — 修改 `orchestrator/knowledge_consumer.py`：
- 删除第 14-25 行的 `sign_body()` 和 `make_headers()` 函数定义
- 添加：`from hub_ai.shared import sign_body, make_headers`

**Step 4** — 修改 `orchestrator/graph.py`：
- 删除第 41-44 行的内联 HMAC 逻辑
- 添加：`from hub_ai.shared import make_headers`
- 第 40-45 行改为：
  ```python
  body_bytes = json.dumps(body).encode()
  headers = make_headers(secret, body_bytes)
  ```
- 删除不再需要的 `import hmac` 和 `import hashlib`

**Step 5** — 修改 `orchestrator/tracing.py`：
- 删除第 11-14 行的 `sign_body()` 函数定义
- 添加：`from hub_ai.shared import sign_body`
- 删除不再需要的 `import hmac` 和 `import hashlib`

**面试叙事**："发现四个文件里有完全相同的 HMAC 签名逻辑，抽了一个共享模块 `hub_ai/shared.py`。这是典型的 DRY 重构，面试官很容易注意到重复代码。"

---

### P1-2：合并双路径架构 + 消除 80% 重复代码

#### 背景：当前的两条执行路径

系统有两种 NL2SQL 执行方式，它们在 `PostMessage` handler 内通过关键词路由分叉：

```
用户输入 "how many rows in demo_sales"

═══════════════════════════════════════════════════════════════
路径 A：同步（默认，不含"分析/报告"关键词，未勾选深度分析）
═══════════════════════════════════════════════════════════════

  用户 ──POST──▶ Go PostMessage ──gRPC──▶ Python LLM (生成SQL)
                                       │
                                    sqlrun (只读执行)
                                       │
                                       ▼
                                   立即返回 200:
                                   {run_id, sql, rows}

  延迟: ~1-2s
  输出: SQL + 数据表格
  协议: gRPC

═══════════════════════════════════════════════════════════════
路径 B：异步（含"分析/报告"关键词，或显式传 workflow=agent_pipeline）
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

  延迟: ~5-15s
  输出: SQL + 数据表格 + 分析摘要 + Markdown 报告 + Agent 步骤追踪
  协议: NATS + HTTP 回调
```

**路由逻辑**（`handlers/handlers.go:364-372`）：

```go
isComplex := strings.Contains(low, "分析") || strings.Contains(low, "报告") ||
    strings.Contains(low, "analyze") || strings.Contains(low, "report")
useAgentPipeline := (workflow == "agent_pipeline") || (workflow == "auto" && isComplex)
```

**核心问题**：路径 A 的 `PostMessage:388-410` 和路径 B 中 Python 回调的 `InternalNL2SQL:590-609`，本质都是 **gRPC GenerateSQL → sqlrun.QueryRows → 返回 {sql, rows}**，两份独立代码约 80% 重复。

#### 方案决策：保留双路径 + 抽公共函数

```
┌──────────────────────────────────────────────────────────────┐
│                     方案对比                                  │
├────────────────────┬─────────────────┬───────────────────────┤
│                    │ 方案 A（推荐）    │ 方案 B                │
│                    │ 保留双路径+去重   │ 统一走异步            │
├────────────────────┼─────────────────┼───────────────────────┤
│ 同步查询延迟        │ ~1-2s           │ ~5-10s（走NATS绕一圈）│
│ 面试演示效果        │ 先秒回再深度分析  │ 简单查询也要等        │
│ 架构叙事            │ 延迟与深度权衡    │ 叙事单一              │
│ 代码改动量          │ 小（半天）        │ 大（重构图+删gRPC）  │
│ gRPC 存在价值       │ 保留，有意义      │ 失去存在价值          │
└────────────────────┴─────────────────┴───────────────────────┘
```

**选择方案 A**。理由：改动小、面试演示对比感强、gRPC 接口被两条路径共享体现设计意图。

#### 修复步骤

**Step 1** — 在 `internal/handlers/handlers.go` 新增 `App.executeNL2SQL()` 方法：

```go
// executeNL2SQL runs the core NL2SQL pipeline: gRPC GenerateSQL → sqlrun.QueryRows.
//
// This is the single code path shared by:
//   - PostMessage (synchronous fast track: user → Go → gRPC → result)
//   - InternalNL2SQL (async agent pipeline: Python LangGraph → HTTP callback → Go)
//
// Both paths generate SQL via the same Python LLM worker and execute it through
// the same read-only SQL guard, ensuring consistent behavior regardless of
// which entry point is used.
func (a *App) executeNL2SQL(ctx context.Context, traceID, userMessage, schemaJSON, dialect string) (string, []map[string]any, error) {
    gen, err := a.Nl2sql.GenerateSQL(ctx, traceID, "", userMessage, schemaJSON, dialect)
    if err != nil {
        st, _ := status.FromError(err)
        return "", nil, fmt.Errorf("nl2sql gRPC: %s", st.Message())
    }
    if !gen.GetOk() {
        return "", nil, fmt.Errorf("nl2sql: %s", gen.GetErrorMessage())
    }
    sql := strings.TrimSpace(gen.GetSql())
    rows, err := sqlrun.QueryRows(ctx, a.DB, sql, a.Cfg.QueryMaxRows, a.Cfg.QueryTimeout)
    if err != nil {
        return "", nil, err
    }
    return sql, rows, nil
}
```

**Step 2** — `PostMessage` 同步路径改为调用 `a.executeNL2SQL()`：

```go
// 替换 handlers.go:388-410
sql, rows, err := a.executeNL2SQL(ctx, trace, body.Text, schemaJSON, dialect)
if err != nil {
    a.finishRunFailed(ctx, rid, sid, err.Error(), codes.Internal)
    errJSON(w, http.StatusBadGateway, err.Error())
    return
}
a.Bus.Publish(sid, ssebus.Event{Type: "sql_generated", Data: map[string]string{"sql": sql}})
// ... 后续存消息、publish result 不变 ...
```

**Step 3** — `InternalNL2SQL` 同样改为调用 `a.executeNL2SQL()`：

```go
// 替换 handlers.go:590-609
sql, rows, err := a.executeNL2SQL(r.Context(), traceID, body.UserMessage, body.SchemaJSON, body.Dialect)
if err != nil {
    a.Log.Error("internal nl2sql failed", zap.Error(err))
    errJSON(w, http.StatusBadGateway, err.Error())
    return
}
JSON(w, http.StatusOK, map[string]any{"sql": sql, "rows": rows, "ok": true})
```

#### 重构后的架构图

```
                   ┌──────────────────────────┐
                   │  App.executeNL2SQL()     │ ← 唯一的 NL2SQL 执行入口
                   │  - gRPC GenerateSQL      │
                   │  - sqlrun.QueryRows      │
                   │  - 返回 sql + rows       │
                   └──────┬───────────────────┘
                          │
          ┌───────────────┼───────────────┐
          │               │               │
    PostMessage      InternalNL2SQL    未来可能的
    (同步快速通道)    (供 Python 回调)   其他调用方
    │               │
    ▼               ▼
  返回给用户       返回给 LangGraph
  (200 JSON)      analysis_node 继续
```

#### 面试叙事

> "这个系统有两条执行路径。当用户问简单问题时——比如'有多少行数据'——走同步快速通道，gRPC 调用 LLM 生成 SQL 后直接执行并返回，延迟在 1 秒左右。当用户问复杂问题——比如'分析销售趋势并生成报告'——系统会自动路由到 LangGraph 多 Agent 管道，由 nl2sql、analysis、report、chart 四个 Agent 协作完成，用户通过 SSE 实时看到每个 Agent 的执行状态。
>
> 两条路径共享同一个 `executeNL2SQL` 方法，保证无论走哪条路径，SQL 生成和执行的一致性。这是延迟和深度之间的设计权衡——不是所有请求都需要启动完整的 Agent 管道。"

---

### P1-3：修复 consumer.py 潜在的 NameError

**现状**：`services/ai/orchestrator/consumer.py:31` 中 `task_id = ""` 在 try 块外初始化，但实际上 `headers` 变量在改造后已经通过 `make_headers()` 调用，不再是手动构建。检查发现当前代码已经安全——`task_id` 在第 31 行就赋值了空字符串。

**当前代码已经是安全的**（`task_id = ""` 在 try 之前），无需修改。此项从 ISSUES.md 的 #7 已修复。

---

## P2：架构改进，提升稳健性

### P2-1：LangGraph checkpointer 从 MemorySaver 迁至 SqliteSaver

**现状**：`orchestrator/graph.py:110` 使用 `MemorySaver()`，进程重启后所有正在运行的 Agent 工作流状态丢失。

**修复**：

```python
# graph.py line 110 替换为：
import sqlite3
from langgraph.checkpoint.sqlite import SqliteSaver

DB_PATH = os.environ.get("LANGGRAPH_DB_PATH", "/app/data/langgraph_checkpoints.db")
os.makedirs(os.path.dirname(DB_PATH), exist_ok=True)
conn = sqlite3.connect(DB_PATH, check_same_thread=False)
memory = SqliteSaver(conn)
```

**额外依赖**：`requirements.txt` 无需新增（`langgraph` 自带 `SqliteSaver`）。

**Docker Compose**：在 `docker-compose.yml` 的 ai-worker 服务中挂载 volume：
```yaml
volumes:
  - ai_data:/app/data
```

**面试叙事**："MVP 阶段用 MemorySaver 快速验证，之后迁移到了 SqliteSaver。这样即使 ai-worker 重启，正在运行的 Agent 管道也能从中断点恢复。"

---

### P2-2：统一迁移文件管理

**现状**：~~存在两套迁移文件——~~ 已统一。
- `internal/migrate/001-004_*.sql`：Go embed 打包，启动时自动执行，所有迁移文件均在此目录
- ~~`migrations/005-007_*.sql`：未自动执行，需要手动操作~~ 已删除冗余副本

**修复**：

1. ~~确认 `internal/migrate/` 的内容覆盖了 `migrations/` 的需求（002=async_tasks, 003=knowledge_docs, 004=agent_run_steps，对应 migrations 的 005-007）~~ 已确认
2. ~~如果已覆盖 → 删除 `migrations/` 目录~~ 已删除
3. ~~如果未完全覆盖 → 将缺失内容合并到 `internal/migrate/`，再删除 `migrations/`~~ 无需操作

**检查方法（已完成）**：
```bash
# 以下验证已通过，内容一致
diff internal/migrate/002_async_tasks.sql migrations/005_async_tasks.sql
diff internal/migrate/003_knowledge_docs.sql migrations/006_knowledge_docs.sql
diff internal/migrate/004_agent_run_steps.sql migrations/007_agent_run_steps.sql
```

---

### P2-3：清理空目录和死代码

| 项目 | 操作 | 理由 |
|------|------|------|
| `pkg/` | 删除整个目录 | 仅含 `.gitkeep`，面试官会问"这个目录为什么是空的？" |
| `gRPC RunAgentPipeline` | 删除 proto 中的 RPC 定义 + 重新生成桩代码，或添加注释说明预留 | 定义了但不用，面试官看 proto 会困惑 |
| `Dockerfile.api` 的 Go 版本 | 确认 `go.mod` 中的版本与 Dockerfile 对齐 | 当前 `go.mod` 指定 1.25.0，Dockerfile 用 1.25 |

---

### P2-4：SSE 总线增加可观测性

**现状**：`ssebus/bus.go` 的 `Publish` 在 channel 满时静默丢弃消息。

**改进**：在 Publish 方法中加一个丢弃计数器的日志：

```go
func (b *Bus) Publish(sessionID string, evt Event) {
    b.mu.RLock()
    ch, ok := b.subs[sessionID]
    b.mu.RUnlock()
    if !ok {
        return
    }
    select {
    case ch <- evt:
    default:
        // 消费者太慢，丢弃事件（避免阻塞发布者）
        atomic.AddInt64(&b.dropped, 1)
        log.Printf("ssebus: dropped event type=%s for session=%s (total dropped: %d)",
            evt.Type, sessionID, atomic.LoadInt64(&b.dropped))
    }
}
```

**这个改进很小但面试能展开讲**："SSE 总线用了非阻塞发布策略，防止慢消费者阻塞整个系统。被丢弃的事件数量通过日志暴露，后续可以接入 Prometheus 指标。"

---

## P3：功能增强，提升演示效果

### P3-1：新增 chart_agent（图表 Agent）

**背景**：当前多 Agent 工作流只有 3 个节点（nl2sql → analysis → report），输出是纯文本 Markdown。面试时如果用户问"分析销售趋势"却只看到一堆文字，体验不够好。

**方案**：在 LangGraph 图中新增第 4 个节点 `chart_node`，与 `report_node` 并行执行。

```
当前：                               新增后：
nl2sql                               nl2sql
  │                                     │
analysis                              analysis
  │                                  ┌──┴──┐
report                           report   chart
                                  │       │
                                  └──┬────┘
                                     ▼
                                  最终输出
                                  (Markdown + Chart JSON)
```

**新增文件**：`services/ai/agents/chart_agent.py`

核心逻辑：
1. 接收 `nl2sql_result`（数据行）+ `analysis_summary`（分析摘要）
2. 用 LLM 判断数据适合什么图表类型（柱状图/折线图/饼图/散点图）
3. 生成 ECharts option JSON（前端可直接渲染）
4. 将 chart 配置存入 AgentState

```python
# chart_agent.py 核心结构
CHART_PROMPT = """Based on the following data and analysis, choose the best chart type and generate an ECharts option JSON.

Data (first 10 rows):
{data_sample}

Analysis:
{analysis}

Rules:
- For comparing categories → bar chart
- For time series → line chart
- For proportions → pie chart
- For correlation → scatter plot
- Always include title, tooltip, and legend
- Return ONLY valid JSON, no markdown wrapping
"""
```

**LangGraph 图修改**（`graph.py`）：

```python
from agents.chart_agent import chart_node

# 在 build_graph() 中：
builder.add_node("chart_node", chart_node)
builder.add_edge("analysis_node", "chart_node")
builder.add_edge("chart_node", END)
# report_node 保持不变：analysis_node → report_node → END
```

**前端修改**（`web/src/App.tsx`）：
- 检查消息中是否有 `chart` 字段
- 如果有，用 `recharts` 或 ECharts 渲染
- 新增依赖：`npm install recharts`

**面试叙事**："Agent 不只是返回文字，它还能根据数据特征自动选择合适的可视化图表——柱状图、折线图、饼图、散点图。这是 chart_agent 根据 analysis 的结果用 LLM 决定的。"

---

### P3-2：前端 Docker 化 + 体验改进

**已在 OpenSpec change `frontend-demo-production-ready` 中设计，直接执行即可。**

核心产出：
- 新增 `Dockerfile.web`（nginx 托管 Vite 构建产物）
- `docker-compose.yml` 加 `web` 服务
- ErrorBoundary 组件（防止单点异常白屏）
- Skeleton 骨架屏（替代纯文字 "loading..."）
- TypeScript 类型化（消除 `any`）
- SSE 重连指数退避（1s → 2s → 4s → 8s → 上限 30s）

---

### P3-3：Handler 集成测试

**为什么需要**：当前 Go 测试仅覆盖工具包（auth、config、sqlrun、schema），核心 handler 零覆盖。面试官问"怎么测试核心流程"时无法回答。

**最低要求的 3 个测试**：

```go
// internal/handlers/handlers_test.go

func TestPostMessage_SimpleSQL(t *testing.T) {
    // 1. 创建带 mock gRPC client 的 App
    // 2. POST /v1/sessions/{id}/messages 发 "how many rows in demo_sales"
    // 3. 验证返回 200 + sql 字段非空 + rows 非空
}

func TestPostMessage_ExportTriggersApproval(t *testing.T) {
    // 1. 发 "export sales data"
    // 2. 验证返回 202 + status = "awaiting_approval"
}

func TestKnowledgeUpload_Integration(t *testing.T) {
    // 1. POST /v1/workspaces/{id}/knowledge/docs
    // 2. 验证返回 202 + task_id 非空
    // 3. 轮询 GET /v1/workspaces/{id}/knowledge/docs
    // 4. 验证文档 status 变为 "completed"
}
```

**使用 `httptest.NewServer` + mock gRPC stub**，不需要真实依赖。

---

### P3-4：Python 侧 Prompt 工程文档化

**现状**：`__main__.py` 的 `_openai_sql()` 方法中 prompt 是字符串拼接，没有注释说明设计理由。

**改进**：将 prompt 抽为独立模板，加注释说明设计意图。

```python
# services/ai/hub_ai/prompts.py

NL2SQL_SYSTEM_PROMPT = """You are a SQL expert. Generate a single SQL query for the given question.

## Database Schema
{schema}

## Rules
1. Return ONLY the SQL statement, no markdown, no explanation
2. Always add LIMIT {max_rows} to prevent full table scans
3. Use PostgreSQL dialect
4. Only SELECT statements are allowed
5. After the SQL, add a line starting with "-- Notes:" with a brief self-check

## Question
{user_message}
"""
# 设计理由：
# - "Return ONLY the SQL" → 减少 post-processing（无需剥离 markdown）
# - 显式 LIMIT → 防止 LLM 遗漏导致全表扫描，即使 sqlrun 有兜底限制
# - "-- Notes:" 自检 → 可观测性，`self_check_notes` 字段会被记录到 run 中
```

**面试叙事**："Prompt 设计有几个刻意为之的点——强制 LIMIT 防止全表扫描，要求自检备注用于 Agent 可观测性，禁止 markdown 包裹减少解析开销。"

---

## 附录 A：执行顺序建议（两周）

### 第 1 周：功能修复 + 代码质量

| 天 | 任务 | 预计耗时 | 产出 |
|---|------|---------|------|
| Day 1 | P0-1 修 knowledge consumer 启动 | 0.5h | 知识索引端到端通 |
| Day 1 | P1-1 Python HMAC 去重（新建 shared.py + 改 4 个文件） | 2h | 代码审查无重复 |
| Day 2 | P1-2 Go 侧 executeNL2SQL 抽取 | 3h | 两条路径统一 |
| Day 2 | P2-2 统一迁移文件 | 0.5h | 删除冗余目录 |
| Day 3 | 冒烟清单 8 步全过（docker compose up → 逐条验证） | 3h | 确保面试官能跑通 |
| Day 4 | P3-3 Handler 集成测试（3 个） | 4h | 核心流程有测试 |
| Day 5 | P3-1 chart_agent 实现 + LangGraph 图改造 | 4h | Agent 从 3 节点变 4 节点 |

### 第 2 周：体验打磨 + 面试准备

| 天 | 任务 | 预计耗时 | 产出 |
|---|------|---------|------|
| Day 6-7 | P3-2 前端 Docker 化 + recharts + ErrorBoundary + Skeleton | 8h | 前端像样 |
| Day 8 | P2-1 LangGraph SqliteSaver + Docker volume | 2h | 重启不丢状态 |
| Day 8 | P2-3 清理空目录 + 死代码 | 1h | 项目整洁 |
| Day 9 | P3-4 Prompt 文档化 | 2h | 面试能展开讲 |
| Day 9 | P2-4 SSE 丢弃计数器 | 0.5h | observability 加分 |
| Day 10 | 全栈 docker compose 反复 up/down 测试 | 2h | 幂等性验证 |
| Day 10 | 修所有 FIXME / TODO | 2h | 代码无遗留标记 |
| Day 11 | 写 `docs/AGENT_DESIGN.md`（LangGraph 设计 + prompt 迭代 + 状态管理） | 3h | 面试核心文档 |
| Day 12 | 录制 3 分钟 demo 视频 | 2h | 面试官没时间跑也能看 |
| Day 12 | README 补充演示场景说明 | 1h | 降低面试官上手成本 |
| Day 13 | 缓冲日 / 补充压测 | - | 从容应对 |
| Day 14 | 缓冲日 / 面试模拟 | - | 从容应对 |

---

## 附录 B：面试叙事准备

### 10 个高频问题 + 回答要点

| # | 问题 | 回答要点 |
|---|------|---------|
| 1 | **为什么 Go + Python？** | Go 做控制面（HTTP/认证/限流/SQL 执行），Python 做 AI 计算面（LangGraph/LLM/RAG）。Python 端没有数据库访问权限，安全边界在 Go 的 `sqlrun.IsReadOnlySQL()` 强制把关 |
| 2 | **LangGraph 图怎么设计的？** | 4 节点：nl2sql → analysis → report / chart。条件路由根据用户意图跳过不需要的节点。用 SqliteSaver 做检查点持久化 |
| 3 | **Agent 出错了怎么处理？** | 每个节点有 try/except，通过 tracing 模块回调 Go 写入 `agent_run_steps` 表。NATS 消费者失败后 NAK，JetStream 重新投递。超时任务被 reaper goroutine 标记过期 |
| 4 | **Prompt 怎么设计的？** | 强制 LIMIT、禁止 markdown 包裹、要求自检备注。见 `prompts.py` 的注释 |
| 5 | **怎么保证安全？** | 双认证（JWT + HMAC）、只读 SQL 守卫（关键字检测阻断写操作）、AES-256-GCM 加密数据源密码、JWT 吊销黑名单、限流（Redis 滑动窗口） |
| 6 | **同步和异步有什么区别？** | 同步 = 低延迟快速通道（~1s），gRPC 直接调 LLM 生成 SQL 后执行返回。异步 = 多 Agent 管道（~10s），通过 NATS 触发 LangGraph，用户通过 SSE 看每个 Agent 的执行状态。这是延迟和深度之间的权衡 |
| 7 | **怎么测试的？** | Go 侧：工具包单元测试 + handler 集成测试（httptest + mock gRPC）。Python 侧：agent 和 RAG 单元测试（pytest + mock） |
| 8 | **RAG 管道怎么工作的？** | 用户上传文档 → Go 写入 DB + 发 NATS → Python knowledge consumer 接收 → ChromaDB 分块（RecursiveCharacterTextSplitter, 1000/200） → OpenAI embed → 回调 Go 更新状态 |
| 9 | **能水平扩展吗？** | MVP 阶段 SSE 总线是内存实现，后续计划迁移到 Redis pub/sub。NATS JetStream 支持多消费者负载均衡。Postgres 和 Redis 是单点，但在 Compose 单机部署范围内够用 |
| 10 | **最大的技术挑战是什么？** | 设计双协议回调路径（gRPC + NATS + HTTP 回调）+ 消除循环依赖（Python 不能直接访问 DB，必须通过 Go API）。这保证了安全边界不退化 |

---

## 附录 C：相关文档索引

| 文档 | 说明 |
|------|------|
| `docs/INTERVIEW_PREP_ANALYSIS.md` | 项目架构全面分析 + 面试策略 |
| `docs/ISSUES.md` | 41 个 issue 详细清单（历史参考，部分已修复） |
| `docs/GATEWAY_ARCHITECTURE.md` | 架构分析与三阶段演进 |
| `docs/SMOKE_CHECKLIST.md` | 8 步面试演示冒烟清单 |
| `docs/DEPLOY.md` | 单机 Compose 部署说明 |
| `CLAUDE.md` | AI 辅助开发指令 |
| `README.md` | 快速入门 |

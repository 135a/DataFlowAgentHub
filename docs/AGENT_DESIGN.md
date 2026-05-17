# Agent 设计文档

> **版本**: 基于 commit `cb8194e` | **编写日期**: 2026-05-16 | **性质**: 快照文档（随代码演进可能过时）

---

## 目录

1. [概述](#1-概述)
2. [LangGraph 图设计](#2-langgraph-图设计)
3. [Prompt 工程](#3-prompt-工程)
4. [状态管理](#4-状态管理)
5. [关键决策记录](#5-关键决策记录)
6. [后续改进方向](#6-后续改进方向)

---

## 1. 概述

### 1.1 定位

Agent 系统是 DataFlowAgentHub 的 **Python AI 计算面**，负责 Multi-Agent 流程编排。它不直接面向用户，而是通过以下路径触发：

- **同步路径**：用户发消息 → Go API 通过 gRPC 调用 Python `GenerateSQL` → 返回结果（单步 NL2SQL）
- **异步路径**：消息含 "analyze"/"report" 等关键词 → Go 端创建 `async_tasks` + 发布 NATS 消息 → Python `consumer.py` 订阅并运行 LangGraph 图 → HTTP 回调 Go API

### 1.2 设计目标

| 目标 | 说明 |
|------|------|
| **可编排** | 通过 LangGraph `StateGraph` 组合 NL2SQL、Analysis、Chart、Report 四个 Agent 节点 |
| **可观测** | 每个节点执行前后通过 HTTP 回调向 Go API 报告 `agent_run_steps` 记录 |
| **可恢复** | 基于 SQLite 的 Checkpoint 机制，进程重启后可恢复未完成的工作流 |
| **容错** | Chart Agent 失败不阻断后续 Report 生成；LLM 调用失败时降级到纯统计分析 |
| **内部安全** | Python → Go Internal API 调用使用 HMAC-SHA256 签名认证 |

### 1.3 整体架构

```
┌──────────────────────────────────────────────────────────────────────┐
│                        NATS (JetStream)                               │
│                  Topic: hub.tasks.agent_pipeline                      │
└──────────────────────────┬───────────────────────────────────────────┘
                           │ subscribe
                           ▼
┌──────────────────────────────────────────────────────────────────────┐
│  consumer.py (asyncio)                                                │
│  ┌────────────────────────────────────────────────────────────────┐  │
│  │              LangGraph StateGraph (graph.py)                    │  │
│  │                                                                │  │
│  │   START ──► nl2sql_node ──► route_next (条件边)               │  │
│  │                │              │                                 │  │
│  │                │         ┌────┼────┬────────┐                   │  │
│  │                │         ▼    ▼    ▼        ▼                   │  │
│  │                │      analysis chart report __end__             │  │
│  │                │         │     │     │                          │  │
│  │                │         ▼     ▼     │                          │  │
│  │                │      route_  route_ │                          │  │
│  │                │      after_  after_ │                          │  │
│  │                │      analysis chart │                          │  │
│  │                │         │     │     │                          │  │
│  │                │         ▼     ▼     ▼                          │  │
│  │                │      report report END                         │  │
│  │                │         │     │                                │  │
│  │                └─────────┴─────┴──── END                        │  │
│  └────────────────────────────────────────────────────────────────┘  │
│                     │                                                 │
│                     ▼                                                 │
│     SQLite Checkpointer (LANGGRAPH_DB_PATH)                          │
│     AgentState 序列化 / 恢复                                         │
└──────────────────────────────────────────────────────────────────────┘
         │                    │
         │  HTTP callback     │  Internal API calls (HMAC)
         ▼                    ▼
┌──────────────────────────────────────────────────────────────────────┐
│  Go API (:8080)                                                       │
│  /internal/nl2sql          — NL2SQL 执行（Python 回调 Go）            │
│  /internal/tasks/{id}/callback  — 异步任务结果回调                     │
│  /internal/runs/{id}/steps      — Agent 步骤追踪上报                  │
└──────────────────────────────────────────────────────────────────────┘
```

---

## 2. LangGraph 图设计

### 2.1 状态图定义

文件：`services/ai/orchestrator/graph.py`

```python
builder = StateGraph(AgentState)
builder.add_node("nl2sql_node", nl2sql_node)
builder.add_node("analysis_node", data_analysis_node)
builder.add_node("chart_node", chart_agent_node)
builder.add_node("report_node", report_generation_node)

builder.add_edge(START, "nl2sql_node")
builder.add_conditional_edges("nl2sql_node", route_next)
builder.add_conditional_edges("analysis_node", route_after_analysis)
builder.add_conditional_edges("chart_node", route_after_chart)
builder.add_edge("report_node", END)
```

### 2.2 节点说明

| 节点 | 函数 | 输入（state 字段） | 输出（写入 state） | 实现方式 |
|------|------|-------------------|-------------------|---------|
| `nl2sql_node` | `nl2sql_node()` | `user_input`, `schema_context`, `run_id` | `nl2sql_result`, `nl2sql_sql` 或 `nl2sql_error` | HTTP 回调 Go `/internal/nl2sql` |
| `analysis_node` | `data_analysis_node()` | `nl2sql_result`, `user_input` | `analysis_summary` | pandas 统计 + LLM 业务摘要 |
| `chart_node` | `chart_agent_node()` | `nl2sql_result`, `run_id` | `chart_paths` | matplotlib 自动选图（bar/line/pie） |
| `report_node` | `report_generation_node()` | `nl2sql_result`, `analysis_summary`, `chart_paths`, `run_id` | `final_report` | Markdown 模板 + Excel 导出 |

### 2.3 条件路由逻辑

#### 第一层路由：`nl2sql_node` 之后 (`route_next`)

路由依据：`workflow` 参数（显式指定）或用户输入关键词（自动推断）。

```python
def route_next(state) -> Literal["analysis_node", "chart_node", "report_node", "__end__"]:
    workflow = state.get("workflow", "auto")

    if workflow == "simple":
        return "__end__"          # 仅 NL2SQL，不触发后续 Agent
    if workflow == "agent_pipeline":
        return "analysis_node"    # 完整 Multi-Agent 流程

    # Auto 模式：关键词匹配
    if any(kw in user_input for kw in ("chart", "图表", "可视化", "plot")):
        return "chart_node"
    if any(kw in user_input for kw in ("分析", "analyze", "趋势", "对比")):
        return "analysis_node"
    if any(kw in user_input for kw in ("报告", "report", "export", "导出")):
        return "report_node"

    return "__end__"  # 默认：NL2SQL 即止
```

#### 第二层路由：`analysis_node` 之后 (`route_after_analysis`)

```python
def route_after_analysis(state):
    if state.get("workflow") == "agent_pipeline":
        return "chart_node"   # 完整流程：分析 → 图表 → 报告
    return "report_node"      # 标准流程：分析 → 报告
```

#### 第三层路由：`chart_node` 之后 (`route_after_chart`)

```python
def route_after_chart(state):
    if state.get("workflow") == "agent_pipeline":
        return "report_node"  # 图表 → 报告（完整链路）
    return "__end__"          # 仅图表，结束
```

### 2.4 完整路径枚举

| 输入关键词示例 | workflow | 执行路径 |
|-------------|----------|---------|
| "show me sales"（无特殊关键词） | auto | `nl2sql → END` |
| "分析销售趋势" | auto | `nl2sql → analysis → report → END` |
| "画一个图表" | auto | `nl2sql → chart → END` |
| "生成报告" | auto | `nl2sql → report → END` |
| "anything" | `agent_pipeline` | `nl2sql → analysis → chart → report → END` |
| "anything" | `simple` | `nl2sql → END` |

### 2.5 节点间数据流

```
                             ┌────────────────────┐
                             │   user_input        │  ← Go API 传入
                             │   schema_context    │  ← Go API 传入 (JSON)
                             │   run_id            │  ← Go API 传入
                             │   workflow          │  ← Go API 传入
                             └────────┬───────────┘
                                      │
                                      ▼
                           ┌─────────────────────┐
                           │   nl2sql_node       │
                           │   → HTTP POST       │
                           │   /internal/nl2sql  │
                           └─────────┬───────────┘
                                     │
                              ┌──────┴──────┐
                              │ 写入 state  │
                              │ nl2sql_result│ ← list[dict] (查询结果行)
                              │ nl2sql_sql  │ ← str (生成的SQL)
                              │ nl2sql_error│ ← str (失败时)
                              └──────┬──────┘
                                     │
                   ┌─────────────────┼─────────────────┐
                   ▼                 ▼                   ▼
           ┌──────────────┐  ┌──────────────┐  ┌──────────────┐
           │ analysis_node│  │ chart_node   │  │ report_node  │
           │              │  │              │  │              │
           │ 读:          │  │ 读:          │  │ 读:          │
           │ nl2sql_result│  │ nl2sql_result│  │ nl2sql_result│
           │ user_input   │  │ run_id       │  │ analysis_    │
           │              │  │              │  │   summary    │
           │ 写:          │  │ 写:          │  │ chart_paths  │
           │ analysis_    │  │ chart_paths  │  │ run_id       │
           │   summary    │  │              │  │              │
           └──────┬───────┘  └──────┬───────┘  │ 写:          │
                  │                 │           │ final_report │
                  ▼                 ▼           └──────┬───────┘
           ┌──────────────┐  ┌──────────────┐         │
           │ → report_node│  │ → report_node│         │
           │   (默认)     │  │   (agent_    │         │
           │              │  │   pipeline)  │         │
           └──────────────┘  └──────────────┘         │
                                                      ▼
                                                    END
```

---

## 3. Prompt 工程

### 3.1 NL2SQL 节点

NL2SQL 不直接使用 LLM Prompt，而是通过 HTTP 调用 Go API 的 `/internal/nl2sql` 端点。Go 侧负责：

- 从请求中获取 `user_message`（自然语言问题）和 `schema_json`（表结构 JSON）
- 调用 LLM（OpenAI 兼容 API）生成 SQL
- 执行只读 SQL（通过 `sqlrun` 包的安全检查）
- 返回结果行和生成的 SQL

**设计考量**: 将 SQL 生成和安全性检查集中在 Go 侧，复用 Go 的 `connector` / `schema` / `sqlrun` / `llm` 包，Python 侧保持轻量。

**请求格式**:
```json
{
    "user_message": "上个月每个品类的销售额是多少？",
    "schema_json": "{\"tables\": [{\"name\": \"orders\", \"columns\": [...]}]}",
    "dialect": "postgres"
}
```

**响应格式**:
```json
{
    "rows": [{"category": "电子", "sales": 125000}, ...],
    "sql": "SELECT category, SUM(amount) AS sales FROM orders ...",
    "ok": true
}
```

### 3.2 Data Analysis Agent

**System Prompt** (v1，2026-05-16):

```
You are a data analyst. Write a concise business summary of the statistical findings.
```

**User Prompt 结构**:
```
User intent: {user_input}

Statistics:
{raw_stats}         # pandas describe() 输出 + 异常检测结果
```

**设计原则**:
- **温度 0.3**: 统计分析需要事实准确性而非创造性
- **防护截断**: 输入数据超过 500 行时自动截断（`truncate_data`），防止 context overflow
- **降级策略**: LLM 调用失败或 API Key 缺失时，降级到纯 pandas 统计输出

**统计预处理**（LLM 调用前）:
1. `pandas.describe()` — 数值列的 count/mean/std/min/25%/50%/75%/max
2. 异常检测 — 3σ 原则：`|value - mean| > 3 * std`
3. 无数值列时跳过统计，直接提示 "No numeric columns found"

### 3.3 Chart Agent

图表 Agent 不使用 LLM，完全基于规则引擎。

**图表类型自动选择**:

| 条件 | 选择 |
|------|------|
| 1 文本列 + 1 数值列，≤6 行 | `pie` (饼图) |
| 标签列含时间特征（年/月/日/-/:/Q/W） | `line` (折线图) |
| 1+ 文本列 + 1+ 数值列，或 ≥2 数值列 | `bar` (柱状图) |
| 其他 | `bar` (默认) |

**数据采样**: 超过 50 行时自动等距采样

**中文支持**: 启动时扫描系统中文字体，优先匹配 `Noto Sans CJK SC` → `SimHei` → `Microsoft YaHei`，无 CJK 字体时降级到 DejaVu Sans（tofu 占位符）

**容错设计**: Chart Agent 为 Non-blocking——任何异常捕获后返回空 `chart_paths`，不影响后续 Report 生成

**伴生图表**: 当选为 `line` 且有 ≥2 个数值列时，额外生成一张 `bar` 图作为对照

### 3.4 Report Generation Agent

Report Agent 使用模板拼接而非 LLM 生成。

**Markdown 模板结构**:
```markdown
# Data Analysis Report
Generated at: {timestamp}

## Request
{user_input}

## Analysis Summary
{analysis_summary}

## Data Extract
{markdown_table}          # df.head(10).to_markdown()

## 数据可视化               # 仅当 chart_paths 非空
![chart](./{filename})
*共 N 个图表*
```

**附加输出**: 同时生成 Excel 文件 (`/tmp/reports/{run_id}.xlsx`)，可通过 Go API 报表下载端点获取

### 3.5 Prompt 迭代日志

| 版本 | 日期 | 节点 | 变更 | 原因 | 效果 |
|------|------|------|------|------|------|
| v1 | 2026-05-16 | analysis | 初始版本：`"You are a data analyst. Write a concise business summary of the statistical findings."` | MVP 阶段，简洁够用 | 覆盖基本统计分析场景 |
| v1 | 2026-05-16 | nl2sql | 从 Python LLM 调用改为回调 Go API | Go 侧统一管理 SQL 安全和 LLM 调用 | 安全边界清晰 |

---

## 4. 状态管理

### 4.1 AgentState 结构定义

文件：`services/ai/orchestrator/state.py`

```python
class AgentState(TypedDict, total=False):
    # 输入字段（由 Go API 通过 NATS 消息传入）
    run_id: str                         # 运行标识 UUID
    user_input: str                     # 用户自然语言输入
    schema_context: str                 # 数据库 schema JSON
    workflow: str                       # 工作流模式: "auto" | "simple" | "agent_pipeline"

    # NL2SQL 阶段输出
    nl2sql_result: list[dict[str, Any]] # SQL 查询结果行
    nl2sql_sql: str                     # 生成的 SQL 语句
    nl2sql_error: str                   # NL2SQL 错误信息

    # Analysis 阶段输出
    analysis_summary: str               # 统计分析摘要（LLM 生成或纯统计）

    # Chart 阶段输出
    chart_paths: list[str]              # 图表 PNG 文件路径列表

    # Report 阶段输出
    final_report: str                   # 最终 Markdown 报告

    # 共享字段
    rag_context: str                    # RAG 知识库检索结果（预留）
    sql: str                            # SQL 语句（预留）
    error: str                          # 通用错误字段（预留）
```

### 4.2 Checkpoint 机制

**选型**: `SqliteSaver`（LangGraph 内置的 SQLite Checkpointer）

```python
db_path = os.getenv("LANGGRAPH_DB_PATH", "/data/langgraph/checkpoints.db")
os.makedirs(os.path.dirname(db_path), exist_ok=True)
checkpointer = SqliteSaver.from_conn_string(db_path)
graph = builder.compile(checkpointer=checkpointer)
```

**工作机制**:
- 每个节点执行完成后，LangGraph 自动将当前 `AgentState` 序列化写入 SQLite
- 使用 `configurable.thread_id` 作为 session 隔离键
- 进程重启后，相同的 `thread_id` 可恢复中断的工作流

**当前限制**:
| 限制 | 影响 | 应对 |
|------|------|------|
| SQLite 单文件 | 多 worker 并发写入可能冲突 | MVP 阶段单一 ai-worker，不构成问题 |
| 无自动过期 | 历史 Checkpoint 无限增长 | 建议定期清理 `/data/langgraph/checkpoints.db` |
| 进程级持久 | 容器重启后 Volume 挂载保留数据 | Docker Compose 中 `langgraph_data` 卷已配置 |

### 4.3 状态序列化与恢复

**序列化**: AgentState 中的所有字段均为 JSON 可序列化类型（`str | list[dict] | list[str]`），LangGraph 内置 msgpack 序列化。

**恢复流程**:
1. NATS 消息到达 → `consumer.py` 构造 `initial_state`
2. 调用 `workflow_graph.invoke(initial_state, config)`
3. 如果 `thread_id` 已有 Checkpoint，LangGraph 从最近完成的节点恢复
4. 无 Checkpoint 时从 START 开始完整执行

### 4.4 步骤追踪

每个节点执行时通过 `report_run_step()` 向 Go API 上报步骤状态：

```python
def report_run_step(run_id, agent_name, status, input_summary="", output_summary="", error_message=""):
    payload = {
        "agent_name": agent_name,     # e.g. "nl2sql_agent", "chart_agent"
        "status": status,             # "running" | "succeeded" | "failed" | "skipped"
        "input_summary": input_summary[:1000],
        "output_summary": output_summary[:1000],
        "error_message": error_message[:1000]
    }
    httpx.post(f"{api_url}/internal/runs/{run_id}/steps", ...)
```

调用时机：
- 节点开始时：`status="running"`
- 节点成功时：`status="succeeded"`
- 节点失败时：`status="failed"`
- 无数据跳过时：`status="skipped"`

---

## 5. 关键决策记录

### D1: NL2SQL 从 Mock 到真实实现

**决策**: 将 NL2SQL 从 Python 侧 Mock 实现改为通过 HTTP 回调 Go API `/internal/nl2sql`。

**原因**:
- Go 侧已有成熟的 `llm` 包（OpenAI 兼容客户端）、`connector`（连接池）、`schema`（表发现）、`sqlrun`（只读执行+安全检查）
- Python 侧复用这些能力需要重复实现，增加维护负担
- SQL 执行的安全性（写操作阻断、行数限制）在 Go 侧已有验证

**时间线**: commit `95217d3` 完成改造

### D2: SqliteSaver vs MemorySaver vs PostgresSaver

**选择**: `SqliteSaver`

**对比**:
| 方案 | 优点 | 缺点 |
|------|------|------|
| MemorySaver | 零配置，最快 | 进程重启全部丢失 |
| SqliteSaver | 文件持久化，零运维，单文件 | 不支持并发写 |
| PostgresSaver | 生产级并发 | 增加外部依赖（MVP 不需要） |

**理由**: SqliteSaver 在零运维成本下提供了持久化能力，Docker Compose 的 Volume 挂载保证容器重启后数据保留。MVP 阶段单 worker 无并发问题。

### D3: Chart Agent 非阻塞

**决策**: Chart Agent 失败不中断后续 Report 生成。

**实现**: `chart_agent_node()` 在 `except` 块返回 `{"chart_paths": []}`，下游节点将 `chart_paths` 视为可选字段。

**理由**: 图表生成是最可能出错的环节（字体缺失、数据格式不匹配、内存不足），不应因为辅助功能失败而丢弃已生成的 NL2SQL 和分析结果。

### D4: Python gRPC servicer 当前状态

**状态**: `RunAgentPipeline` RPC 的 Python servicer 已在 `hub_ai/__main__.py` 中实现，Go gRPC 客户端桩代码已在 `internal/gen/` 中生成。但当前异步 Multi-Agent 流程实际走的是 NATS + HTTP callback 路径，gRPC 路径用于同步 `GenerateSQL` 调用。

### D5: 内部通信安全

Python 向 Go API 发起的 Internal API 调用（`/internal/nl2sql`、`/internal/tasks/{id}/callback`、`/internal/runs/{id}/steps`）使用 HMAC-SHA256 签名认证：

```python
def sign_body(secret: str, body: bytes) -> str:
    mac = hmac.new(secret.encode(), body, hashlib.sha256)
    return f"sha256={mac.hexdigest()}"

def make_headers(secret: str, body_bytes: bytes) -> dict:
    return {
        "X-Hub-Signature": sign_body(secret, body_bytes),
        "Content-Type": "application/json",
    }
```

Go 侧 `middleware/InternalHMACAuth` 验证签名。

---

## 6. 后续改进方向

| 优先级 | 方向 | 说明 |
|--------|------|------|
| 高 | PostgresSaver 替换 | 当需要多 worker 水平扩展时切换到 PostgresSaver |
| 高 | LLM 驱动的图表选择 | 当前规则引擎可处理基础场景，复杂数据需 LLM 决策 |
| 中 | Agent 可插拔 | 将 Agent 节点改为注册制，支持运行时添加/移除 Agent |
| 中 | Human-in-the-loop | 在关键节点（NL2SQL 生成 SQL 后）插入人工审批 |
| 中 | 流式输出 | 将 Agent 执行状态通过 SSE 实时推送到前端 |
| 低 | Checkpoint 自动清理 | 添加 TTL 机制定期清理历史 Checkpoint |
| 低 | Report Agent LLM 增强 | 当前模板式报告可升级为 LLM 生成的自然语言报告 |

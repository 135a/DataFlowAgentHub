# 冒烟测试清单 (Smoke Test)

验证 DataFlowAgentHub 全栈环境是否正常运行。共 8 步，按依赖顺序执行，预计耗时 10 分钟。

## 前置条件

- Docker Desktop 已安装（Windows/Mac）或 Docker Engine + Docker Compose（Linux）
- 至少 4GB 可用内存
- 已复制 `.env.example` → `.env` 并填写必填密钥：
  ```bash
  cp .env.example .env
  # 编辑 .env，至少设置 HUB_JWT_SECRET、HUB_INTERNAL_HMAC_SECRET、HUB_DB_ENCRYPTION_KEY
  # 快速生成密钥：openssl rand -hex 32
  ```
- 端口 `80`, `8080`, `5432`, `6379`, `8000`, `4222` 未被占用

---

## 第 1 步：Docker Compose 全栈启动

**目的**：确认所有 7 个容器能正常构建和启动。

```bash
cd deploy/compose
docker compose -f docker-compose.yml --env-file ../../.env up -d --build
```

**预期结果**：在 120 秒内所有容器进入 `running` 或 `healthy` 状态。

```bash
docker compose ps
```

应看到 7 个服务：`postgres`、`redis`、`chroma`、`nats`、`ai-worker`、`api`、`web`

**故障排查**：
- 镜像拉取超时 → 在 Docker Desktop `Settings → Docker Engine` 调整或移除 `registry-mirrors`
- 端口冲突 → 修改 `.env` 中的 `WEB_PORT` 或停止占用端口的本地服务

---

## 第 2 步：基础设施健康检查

**目的**：确认 Go API 和 Postgres、Redis 连接正常。

```bash
curl -s http://localhost:8080/health | jq .
```

**预期输出**：
```json
{
  "postgres": "ok",
  "redis": "ok"
}
```
HTTP 状态码：`200`

**故障排查**：
- `postgres: "down"` → 检查 Postgres 容器日志 `docker compose logs postgres`
- 连接被拒绝 → 等待 10 秒后重试，API 可能在等待 ai-worker 就绪

---

## 第 3 步：用户登录获取 JWT

**目的**：确认认证系统正常，获取后续步骤需要的 JWT token。

```bash
curl -s -X POST http://localhost:8080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@demo.local","password":"changeme"}' | jq .
```

**预期输出**：
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIs...",
  "token_type": "Bearer",
  "expires_in": 86400,
  "workspace_id": "00000000-0000-4000-8000-000000000001",
  "role": "admin"
}
```

**保存 token 为环境变量**：
```bash
export TOKEN="<access_token>"
```

**故障排查**：
- `"invalid credentials"` → 检查 `.env` 中 `HUB_SEED_EMAIL` 和 `HUB_SEED_PASSWORD` 的值
- 401 Unauthorized → 种子用户可能未创建，检查 API 日志 `docker compose logs api | grep seed`

---

## 第 4 步：创建会话

**目的**：确认会话管理功能正常。

```bash
curl -s -X POST http://localhost:8080/v1/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"title":"冒烟测试"}' | jq .
```

**预期输出**：
```json
{
  "id": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
  "title": "冒烟测试"
}
```
HTTP 状态码：`201`

**保存 session ID**：
```bash
export SID="<id>"
```

**故障排查**：
- 401 → TOKEN 环境变量设置错误或已过期，重新执行第 3 步

---

## 第 5 步：NL2SQL 自然语言查询

**目的**：验证核心 NL2SQL 管道（自然语言 → SQL 生成 → 只读执行 → 返回结果）。

```bash
curl -s -X POST "http://localhost:8080/v1/sessions/$SID/messages" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"text":"列出所有表名"}' | jq .
```

**预期输出**（需要数据源中有表；若无表，返回空 rows 也算正常）：
```json
{
  "run_id": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
  "sql": "SELECT ...",
  "rows": [...]
}
```
HTTP 状态码：`200`

**替代测试（无数据源时）**：
```bash
curl -s -X POST "http://localhost:8080/v1/sessions/$SID/messages" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"text":"show tables"}' | jq .
```

**故障排查**：
- `"worker unavailable"` → ai-worker 未就绪，检查 `docker compose logs ai-worker`
- `"schema discovery failed"` → 数据源连接问题（如无外部数据源，此错误属正常，可跳过）
- 无 OPENAI_API_KEY → ai-worker 返回 demo SQL，功能仍可用

---

## 第 6 步：SSE 实时事件推送

**目的**：验证服务端推送事件（SSE）流正常工作。

**终端 1 — 建立 SSE 连接**：
```bash
curl -s -N "http://localhost:8080/v1/sessions/$SID/stream" \
  -H "Authorization: Bearer $TOKEN"
```

**终端 2 — 发送消息触发事件**：
```bash
curl -s -X POST "http://localhost:8080/v1/sessions/$SID/messages" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"text":"show tables"}' | jq .
```

**预期结果（终端 1 输出）**：
```
event: user_message
data: {"text":"show tables"}

event: run_started
data: {"run_id":"..."}

event: sql_generated
data: {"sql":"SELECT ..."}

event: result
data: {"sql":"...","rows":[...],"notes":"..."}
```

**故障排查**：
- 无事件输出 → SSE 连接超时，检查 token 有效性，关闭终端 1 后重连

---

## 第 7 步：异步 Multi-Agent 管道

**目的**：验证异步任务（NATS 消息 → Python LangGraph → 回调）正常工作。

```bash
curl -s -X POST "http://localhost:8080/v1/sessions/$SID/messages" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"text":"分析数据趋势"}' | jq .
```

**预期输出**（异步作业已入队）：
```json
{
  "run_id": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
  "task_id": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
  "status": "pending_async"
}
```
HTTP 状态码：`202`

**查询异步任务状态**：
```bash
export TID="<task_id>"
sleep 15
curl -s "http://localhost:8080/v1/tasks/$TID" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

**预期结果**：任务状态可能为 `running`、`completed` 或 `failed`（取决于 LLM API 配置）。只要有明确状态返回而非 404，即表示异步管道正常。

**故障排查**：
- 请求未触发异步（返回 200 而非 202）→ 检查关键词：中文"分析"或英文"analyze"
- 任务一直是 `pending` → 检查 NATS 连接：`docker compose logs nats`

---

## 第 8 步：审批关卡

**目的**：验证人工审批流程（export 关键词触发审批任务）。

```bash
curl -s -X POST "http://localhost:8080/v1/sessions/$SID/messages" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"text":"export 数据报告"}' | jq .
```

**预期输出**：
```json
{
  "run_id": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
  "status": "awaiting_approval"
}
```
HTTP 状态码：`202`

**查看待审批列表**：
```bash
curl -s "http://localhost:8080/v1/approvals" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

**故障排查**：
- 未触发审批 → 检查关键词："export" 必须在消息中
- 审批列表为空 → 等待 2 秒后重试（数据库异步写入）

---

## 常见问题排查

| 问题 | 原因 | 解决 |
|---|---|---|
| `docker compose up` 失败 | `.env` 文件不存在或缺少必填项 | `cp .env.example .env` 并设置密钥 |
| API 返回 502 | ai-worker gRPC 连接失败 | `docker compose logs ai-worker` |
| 登录返回 401 | 种子用户未创建 | 检查 API 日志，重启 API 容器 |
| NL2SQL 返回"worker unavailable" | ai-worker gRPC 未就绪 | 等待 10 秒后重试 |
| SSE 无事件 | Token 过期或无效 | 重新登录获取新 token |
| 异步任务一直 pending | NATS 消息未送达 | `docker compose logs nats` 检查 JetStream |

---

## 冒烟测试通过标准

- [x] 第 1 步：所有 7 个容器 running
- [x] 第 2 步：`/health` 返回 200，postgres + redis 均为 ok
- [x] 第 3 步：登录返回 JWT token
- [x] 第 4 步：创建会话返回 201
- [x] 第 5 步：NL2SQL 返回 SQL 和 rows
- [x] 第 6 步：SSE 收到 `sql_generated` 和 `result` 事件
- [x] 第 7 步：异步任务返回 202 和 task_id
- [x] 第 8 步：审批返回 202 和 `awaiting_approval`

# DataFlowAgentHub

**面试向 MVP**：Go（chi）控制面 + Python gRPC NL2SQL worker + Postgres + Redis，Docker Compose 一键起；带最小 Web（Vite + React）与 OpenAPI 草稿。

## 架构（ASCII）

```
[Browser] --HTTP--> [Go API :8080] --gRPC--> [Python ai-worker :50051]
                         |                        |
                    [Postgres]               [OpenAI-compatible]
                    [Redis]
```

## 快速开始

1. 复制环境变量：`cp .env.example .env`，编辑 `HUB_JWT_SECRET`（生产勿用默认值）。
2. 启动：`docker compose -f deploy/compose/docker-compose.yml --env-file .env up -d --build`
3. 健康检查：`curl -s http://127.0.0.1:8080/health`
4. 前端（开发）：`cd web && npm install && npm run dev`（默认代理到 `http://127.0.0.1:8080`）

## 本地 Go（不跑 Docker 时）

```powershell
$env:GOMODCACHE="$PWD\.gomodcache"
$env:GOSUMDB="off"
go mod tidy
go run ./cmd/api
```

需本机 Postgres/Redis 与 `HUB_*` 环境变量；并先启动 `services/ai` worker（见 `services/ai/README.md`）或调整 `HUB_NL2SQL_TARGET`。

## 文档

- 部署：`docs/DEPLOY.md`
- 迁移：`docs/MIGRATIONS.md`
- SSE 反代：`docs/SSE_PROXY.md`
- 冒烟：`docs/SMOKE_CHECKLIST.md`
- OpenAPI：`api/openapi/v1/openapi.yaml`

## OpenSpec

变更文档：`openspec/changes/bootstrap-enterprise-data-agent/`（`/opsx:apply` 实现任务、`/opsx:archive` 归档）。

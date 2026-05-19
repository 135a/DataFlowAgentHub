# DataFlowAgentHub

**对话式数据分析平台**：Go 控制面 + Python AI 计算面，自然语言提问 → SQL 生成与执行 → 结果可视化。

## 架构

```
Browser/SPA ──HTTP/SSE──▶ nginx ──▶ Go API :8080 (chi) ──gRPC──▶ Python :50051 (NL2SQL)
                                  │        │        │
                             MySQL    Redis    NATS
                                  │        │        │
                            Go gRPC :9090  ◀──gRPC── Python (回调)
                                  │
                            ChromaDB (向量检索)
```

**双路径**: 简单查询走同步 NL2SQL（gRPC → SQL 生成 → 执行 → 200）；复杂分析走异步 Agent Pipeline（NATS → LangGraph → gRPC 回调 → SSE 推送）。

## 快速开始

```bash
# 1. 配置环境变量
cp .env.example .env   # 编辑 HUB_JWT_SECRET（生产勿用默认值）

# 2. Docker Compose 一键启动（6 服务）
docker compose -f deploy/compose/docker-compose.yml --env-file .env up -d --build

# 3. 验证
curl -s http://127.0.0.1:8080/health

# 4. 前端开发
cd web && npm install && npm run dev   # 代理到 :8080
```

## 本地开发

```powershell
# Go
$env:GOMODCACHE="$PWD\.gomodcache"; $env:GOSUMDB="off"
go mod tidy && go run ./cmd/api

# 前端
cd web && npm run dev

# 测试
go test ./...
```

## 文档

| 文档 | 说明 |
|------|------|
| `docs/ARCHITECTURE.md` | 完整架构（服务拓扑、路由表、数据存储、配置项） |
| `docs/ARCHITECTURE_DIAGRAM.md` | 架构全景图（ASCII） |
| `docs/REQUEST_FLOW.md` | 请求处理流程详解 |
| `docs/DEPLOY.md` | 部署指南 |
| `docs/SMOKE_CHECKLIST.md` | 冒烟测试清单 |
| `docs/CODE_QUALITY_AUDIT.md` | 代码质量审计报告（13 项已全部修复） |
| `api/openapi/v1/openapi.yaml` | OpenAPI 规范 |

## 近期改进 (2026-05-18)

- **MySQL 迁移**: 移除全部 PostgreSQL/pgx 依赖，统一使用 MySQL (`database/sql` + `go-sql-driver/mysql`)，SQL 方言全部切换至 MySQL
- **错误处理**: 消除 ~25 处 `_ =` 静默丢弃 error，关键路径全面日志化
- **限流扩展**: login(20/min/IP) + register(10/min/user) + datasource(30/min/user) + fail-closed 可配
- **测试补充**: handler 集成测试，覆盖 datasource/user/knowledge/data 端点
- **代码组织**: PostMessage 拆分为编排 + 辅助函数；App.tsx 812→280 行（4 组件 + 1 hook）
- **韧性提升**: Python OpenAI 60s / LangGraph 120s 超时保护
- **安全加固**: SQL 关键字拦截新增 EXECUTE/REPLACE/VACUUM/REINDEX/COPY
- **服务通信**: 内部回调从 HTTP+HMAC 迁移至 gRPC+mTLS

## Context

当前项目一键启动后（`docker compose -f deploy/compose/docker-compose.yml --env-file .env up -d --build`），没有系统化的验证步骤。面试官或新开发者需自行摸索如何验证系统正常运行。需要一个 8 步冒烟清单，按依赖顺序覆盖全部核心路径。

当前服务拓扑：
```
Browser/React → Go API (:8080) → Python ai-worker (:50051)
                    |              |
               Postgres, Redis   ChromaDB
                    |
               NATS (JetStream)
```

核心路径：基础设施就绪 → 认证 → 会话管理 → NL2SQL 执行 → 异步任务 → SSE 实时事件 → 审批关卡。

## Goals / Non-Goals

**Goals:**
- 创建 `docs/SMOKE_TEST.md`，包含 8 步冒烟验证流程
- 每步包含 curl 命令和预期输出，面试官可直接复制执行
- 覆盖全栈核心路径：Docker 启动 → 健康检查 → 登录 → 创建会话 → NL2SQL 查询 → SSE 事件 → 异步分析 → 审批
- 补充 3 个 handler 集成测试，覆盖健康检查、登录、NL2SQL 端点

**Non-Goals:**
- 不创建自动化 CI 冒烟流水线
- 不修改现有业务逻辑
- 不实现新的 API 端点
- 不创建压力测试或性能基准

## Decisions

### 1. 8 步冒烟清单按依赖排序

**选择**：按服务拓扑依赖顺序排列 8 步，每一步依赖前一步成功。

1. Docker Compose 全栈启动
2. 基础设施健康检查（Postgres、Redis、ChromaDB）
3. 用户登录获取 JWT
4. 创建会话并发送 NL2SQL 查询
5. 验证 SQL 执行结果
6. 验证 SSE 实时事件推送
7. 验证异步 Agent 管道（analyze 关键字触发）
8. 验证审批关卡（export 关键字触发）

**理由**：每步验证上一步基础设施/服务是否正常，面试官可线性执行快速定位问题。

### 2. 集成测试覆盖 3 个核心端点

**选择**：健康检查、登录、NL2SQL 三个 handler 的 HTTP 层集成测试。

- 健康检查：验证 `/health` 返回 200，Postgres 和 Redis 状态为 ok
- 登录：验证 `/v1/auth/login` 返回 JWT token
- NL2SQL：验证 `/v1/sessions/{id}/messages` 返回 SQL 和结果行

**理由**：这三个测试覆盖了"系统是否活着 → 是否能认证 → 核心功能是否正常"的最短关键路径。

### 3. 测试使用 httptest 标准库

**选择**：使用 Go 标准库 `net/http/httptest` 进行 handler 集成测试，mock 外部依赖（gRPC 客户端、Redis）。

**理由**：不依赖外部服务，可离线运行，CI 友好。

## Risks / Trade-offs

- **集成测试 mock 范围**：mock gRPC 客户端意味着不测试真实的 Python AI Worker 交互 → 冒烟清单手动步骤（4-8）补充此覆盖
- **Docker Compose 环境差异**：不同机器可能资源不足 → 清单开头注明最低资源要求（4GB RAM）

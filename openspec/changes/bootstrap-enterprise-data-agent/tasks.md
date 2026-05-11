## 1. 仓库与工程骨架

- [ ] 1.1 建立 Monorepo 目录：`cmd/`、`internal/`、`pkg/`、`services/ai/`、`api/proto/`、`web/`、`deploy/compose/`、`docs/`
- [ ] 1.2 初始化 Go module 与 Python 项目（依赖锁定：`go.mod`、`pyproject.toml` 或 `requirements.txt`）
- [ ] 1.3 添加根级 `README.md`：项目一句话、架构图（ASCII 即可）、本地启动命令入口

## 2. API 契约（Proto / OpenAPI）

- [ ] 2.1 在 `api/proto/` 定义控制面 ↔ NL2SQL worker 的 gRPC 服务（GenerateSQL、健康检查、版本字段）
- [ ] 2.2 配置 proto 代码生成（Go `protoc-gen-go` + `grpc-go`；Python `grpcio-tools`）并纳入 CI 或 `make gen`
- [ ] 2.3 编写对外 **OpenAPI v1** 草稿（会话、消息、SSE、审批、数据源）并与实现同步迭代

## 3. 本地与单机运行时（Docker Compose）

- [ ] 3.1 编写 `deploy/compose/docker-compose.yml`：`postgres`、`redis`、`api`、`ai-worker`；可选 `nginx` 统一入口
- [ ] 3.2 提供 `.env.example`（数据库、JWT secret、LLM Base URL/API Key、Redis URL），文档说明不落盘真实密钥
- [ ] 3.3 验证 `docker compose up --build` 可拉起全栈（无 Kubernetes 依赖）

## 4. 元数据与迁移

- [ ] 4.1 设计并迁移 Postgres 表：`users`/`workspaces`/`roles`（最小）、`sessions`、`messages`、`runs`、`audit_events`、`approval_tasks`、`data_sources`
- [ ] 4.2 接入迁移工具（如 goose/atlas）并在 `docs/` 记录升级与回滚策略

## 5. 控制面 API 基础（Go）

- [ ] 5.1 实现 HTTP 服务骨架（路由、中间件、统一错误体、请求超时）
- [ ] 5.2 实现 `/health` 与 `/version`（或等价），依赖检查 Postgres/Redis
- [ ] 5.3 实现 trace id 注入与结构化日志（`zap` 等），贯穿 HTTP → gRPC metadata

## 6. tenant-identity（最小 RBAC）

- [ ] 6.1 实现 JWT 签发与校验（登录可简化为种子用户或注册开关）
- [ ] 6.2 实现 API Key 鉴权（单 workspace 或全局演示 key），与 JWT 互斥或并存策略写清
- [ ] 6.3 实现 `viewer`/`operator`/`admin` 最小权限：保护审批与数据源写接口
- [ ] 6.4 实现 workspace 资源隔离校验（跨 workspace 返回固定策略：404 或 403）

## 7. llm-gateway（OpenAI 兼容）

- [ ] 7.1 封装上游 HTTP 客户端：Base URL、模型、超时、重试退避、429/5xx 处理
- [ ] 7.2 实现限流钩子（Redis 计数器或内存版 MVP），在调用前短路
- [ ] 7.3 日志掩码：禁止打印 API Key 与 Authorization 原文

## 8. data-connectivity

- [ ] 8.1 抽象连接器接口（Ping、元数据列表、执行只读查询），先实现 PostgreSQL + MySQL 之一为 MVP
- [ ] 8.2 强制查询超时、最大行数、只读会话/账号校验（按设计文档默认值）
- [ ] 8.3 数据源 CRUD API：创建/更新不回显密码明文；连接测试端点

## 9. nl2sql-engine（Python worker）

- [ ] 9.1 实现 gRPC 服务端：消费 proto 定义的请求/响应
- [ ] 9.2 集成 OpenAI 兼容 SDK：构造 schema 摘要 + 用户问题 prompt，输出 SQL + 自检结构
- [ ] 9.3 只读策略：检测并拒绝 DML/DDL 生成路径，返回明确错误给控制面
- [ ] 9.4 worker `/health` 或无端口健康：通过 gRPC health 或控制面 ping

## 10. agent-orchestration

- [ ] 10.1 实现「消息 → run → 调 worker → 校验 SQL → 执行查询」状态机（内存 + DB 持久化结合）
- [ ] 10.2 实现策略门：导出/敏感动作为 pending 审批，暂停 run
- [ ] 10.3 gRPC 错误映射为对用户可理解的 HTTP/SSE 错误事件

## 11. conversation-session 与 SSE

- [ ] 11.1 会话与消息 REST：创建会话、发送消息、列出历史
- [ ] 11.2 实现 SSE：`text/event-stream`，推送中间步骤、SQL、结果摘要（表格 JSON 或分页引用）
- [ ] 11.3 在 `docs/` 增加反向代理示例：关闭缓冲 / `X-Accel-Buffering: no`（Nginx/Caddy）

## 12. human-approval

- [ ] 12.1 审批任务列表与详情 API（按 workspace 过滤）
- [ ] 12.2 批准/驳回 API，写审计、解锁编排继续或终止
- [ ] 12.3 审批 TTL 与过期终态（定时任务或懒过期检查）

## 13. observability-telemetry

- [ ] 13.1 暴露 Prometheus 兼容 metrics（HTTP 计数、延迟、错误；可选 gRPC 客户端指标）
- [ ] 13.2 Python worker 侧结构化日志与 trace metadata 对齐（至少相同 trace id header）
- [ ] 13.3 （可选）接入 OpenTelemetry SDK；面试版至少保证 metrics + trace id 日志关联

## 14. web 前端（Vite + React + TS）

- [ ] 14.1 脚手架 `web/`：路由、API client、环境变量 `VITE_API_BASE_URL`
- [ ] 14.2 对话页：消息列表、输入框、SSE 消费与事件渲染（含错误与终态）
- [ ] 14.3 结果展示：表格组件；图表可选（与 `design.md` Open Questions 对齐选型）
- [ ] 14.4 审批页：pending 列表、批准/驳回操作、权限不足提示
- [ ] 14.5 登录/令牌页（与后端鉴权方式一致）；生产构建静态资源镜像或由 Nginx 托管

## 15. 演示数据与上线清单

- [ ] 15.1 提供 `deploy/compose/init/` 或种子 SQL：demo 库表与示例问题（面试叙事）
- [ ] 15.2 `docs/DEPLOY.md`：单机服务器步骤、TLS、Compose、日志与备份提示（无 K8s）
- [ ] 15.3 冒烟脚本或 checklist：会话 → 流式回答 → 触发审批 → 批准 → 结果可见

## 16. 文档与 proposal 对齐（可选清理）

- [ ] 16.1 更新 `proposal.md`：移除「K8s 交付」「前端 non-goal」等与当前设计冲突的表述，或增加「已 superseded by design」说明块
- [ ] 16.2 在 `openspec/specs/` 留空说明：主规格将在 archive/sync 阶段合并（若流程需要）

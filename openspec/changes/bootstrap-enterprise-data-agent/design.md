## Context

本仓库目标是做一个**可面试演示、可真实上线到一台服务器**的 DataAgent：Go 控制面 + Python NL2SQL 计算面，OpenAI 兼容 LLM，带 **Human-in-the-loop** 与基础「企业感」能力（鉴权、审计、限流、可观测的最小子集）。

`proposal.md` 写成偏「多租户 SaaS + K8s」形态；你已明确约束为：**个人作品集 / 面试项目、需要前端、不需要 K8s、要能部署到服务器**。本设计以该约束为准，并在「决策」里记录与 proposal 的差异，避免后续实现分叉。

## Goals / Non-Goals

**Goals:**

- 端到端可演示：浏览器前端 → Go API（REST + SSE）→（内部 gRPC）→ Python NL2SQL worker → 受控执行只读 SQL → 表格/图表结果回显。
- **单机可上线**：`docker compose` 一键起全栈；生产可用 **一台 Linux 服务器 + Docker + 反向代理（Nginx/Caddy）** 部署，无需 Kubernetes。
- **前端必须有**：提供对话页、连接配置（面试可用环境变量/单用户配置简化）、结果展示与「待审批」面板（Human-in-the-loop 最小闭环）。
- **工程可信度**：结构化日志、请求级 trace id、基础指标（QPS/延迟/错误率）、SQL 安全护栏（超时、行数上限、只读账号）。
- **面试叙事清晰**：目录结构、契约（OpenAPI + Proto）、关键边界（控制面/计算面）一眼能讲清楚。

**Non-Goals:**

- Kubernetes、Helm、多集群弹性调度。
- 完整多租户 SaaS 商业化能力（计量计费、复杂租户隔离策略）；面试版以「单部署实例 + 少量用户/演示账号」为主，保留后续扩展点。
- 插件市场、复杂 BI 拖拽式建模、向量 RAG（可作为后续变更）。

## Decisions

### D1：与 `proposal.md` 的对齐方式（刻意调整）

- **调整**：交付形态从 **K8s** 改为 **Docker Compose + 单机部署**；**增加 Web 前端**（proposal 曾将前端列为 non-goal）。
- **保留**：Go 控制面 / Python 计算面分工、gRPC 契约、NL2SQL、OpenAI 兼容 LLM 网关思路、Human-in-the-loop、基础审计/配额/限流。
- **理由**：面试项目更看重「可运行 + 可展示 + 可讲架构」，K8s 会显著增加运维与评审成本；前端是演示体验的核心。

### D2：前端技术栈

- **选择**：`web/` 使用 **Vite + React + TypeScript**（或 Next.js 仅当需要 SSR/SEO 时；默认不需要）。
- **理由**：与 Go API 解耦清晰，静态资源可由 Nginx 托管；部署简单、面试常见技术栈。
- **替代方案**：Next.js 全栈（否决原因：与 Go 控制面重复、部署心智负担更高）。

### D3：API 形态与实时性

- **选择**：Go 提供 **OpenAPI v1**：会话消息 REST + **SSE** 流式输出中间步骤/最终 SQL/表格摘要。
- **理由**：浏览器友好、比纯 WebSocket 更易排障；面试场景足够展示「流式 Agent」。
- **替代方案**：WebSocket（否决原因：运维与前端复杂度更高，收益有限）。

### D4：Go ↔ Python 通信

- **选择**：**gRPC + Protobuf**，仓库 `api/proto/` 为单一事实来源；Python worker 无公网端口，仅内网访问。
- **理由**：强类型、性能好、适合「控制面调用计算面」。
- **替代方案**：HTTP JSON（否决原因：契约易漂移）；NATS（否决原因：单机部署不必要）。

### D5：数据库与元数据

- **选择**：
  - **应用元数据**（用户/会话/审计/审批任务）：**PostgreSQL**。
  - **被分析的业务库**：MySQL/Postgres/ClickHouse/Doris 通过连接器接入（面试 MVP 可先只做 Postgres + MySQL）。
- **理由**：与 proposal 一致，且 Postgres 适合存 JSON 审计载荷。

### D6：LLM 接入

- **选择**：仅 **OpenAI 兼容** Base URL + API Key（环境变量注入）；Go 侧集中做重试、超时、简单退避与 Token 计数（可先近似）。
- **理由**：对齐 proposal，面试对接成本最低。

### D7：Human-in-the-loop 最小实现

- **选择**：将「导出 CSV」「执行非只读语句（若未来放开）」「跨库访问（若启用）」等标记为 **Policy Gate**；生成 `approval_task` 记录，前端展示待办；审批后写入审计并继续编排。
- **理由**：用最少表结构与 UI 讲清楚治理故事。

### D8：部署拓扑（无 K8s）

- **选择**：`deploy/compose/docker-compose.yml` 包含：`postgres`、`redis`（会话/限流）、`api`（Go）、`ai-worker`（Python）、`web`（构建静态资源镜像或 nginx 统一入口）。
- **生产**：单机上 `compose` + 宿主机 Nginx/Caddy **TLS 终止**与域名反代；Secrets 用 `.env`（演示）或 Docker secrets（加分项）。

## Risks / Trade-offs

- **[风险] proposal 与实现范围漂移** → **[缓解]** 在 `tasks.md` 里把「移除/替换 K8s 相关交付物」列为显式任务；specs 阶段用 `platform-runtime` 写清楚「Compose 为唯一官方部署」。
- **[风险]「企业级」叙事与「单人部署」冲突** → **[缓解]** 用「安全默认值 + 可演示治理」表达企业感：只读 SQL、行数上限、审批、审计；避免过度承诺多租户隔离等级。
- **[风险] SSE 经过反向代理缓冲** → **[缓解]** Nginx 配置关闭缓冲（`proxy_buffering off` / `X-Accel-Buffering: no`），并在设计评审清单中列为部署检查项。
- **[风险] NL2SQL 安全风险** → **[缓解]** 强制只读账号、表/列 allowlist（MVP 可先全库只读 + 行数限制）、查询超时。

## Migration Plan

本变更首次落地，不涉及存量数据迁移；部署步骤建议写进 `tasks.md` 与 README（实现阶段）：

1. 准备一台 Linux 服务器，安装 Docker + Compose Plugin。
2. 配置 `.env`：数据库密码、LLM Base URL/API Key、JWT secret。
3. `docker compose up -d --build`。
4. 配置反向代理域名 → `web`/`api`（推荐同域 `/api` 前缀，降低 CORS 复杂度）。
5. 冒烟：创建会话 → 提问 → 看到流式输出 → 触发一次需要审批的动作 → 审批通过 → 结果回显。

**回滚**：保留上一版镜像 tag；数据库变更使用向前兼容迁移（实现阶段用 goose/atlas 等工具）。

## Open Questions

- 前端图表库选型（ECharts vs Recharts）与「面试演示默认值」。
- 面试环境是否提供「一键 demo 数据集」（Docker init SQL）以增强故事性。
- 用户体系：单用户 + API Key 是否足够；是否需要注册登录（Email/密码）以增强真实感。

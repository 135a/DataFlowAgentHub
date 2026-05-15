## Why

> **与 `design.md` 对齐（2026）**：实现与交付以 **Docker Compose + 单机部署** 为准，并包含 **Web 前端**；**不包含 Kubernetes**。下文若仍出现「K8s 交付」或「前端 non-goal」等历史表述，视为已被 `design.md` 替代，以设计文档为准。

企业用户在做数据分析时长期面临三难：①传统 BI 学习成本高、自助度低；②市面上的对话式数据分析产品大多是「单语言栈」——纯 Python 方案在网关/编排/并发上吃力，纯 Go 方案又难以承接 LLM / 数据科学生态；③开源 NL2SQL 项目普遍是 demo 级，缺少多租户、审计、可观测、人工审批等企业必备能力。

DataFlowAgentHub 通过 **Go 控制面 + Python AI 计算面** 的一体化架构，自带多租户 SaaS 与 Human-in-the-loop 治理，目标是用一个平台同时拿到「企业级稳定性」和「现代 Agent/LLM 能力」。本变更是项目的零号 bootstrap，目标是搭出端到端可验证的 **对话式 NL2SQL** 垂直闭环 + 平台骨架。

## What Changes

- **NEW** 建立 Go 控制面：API 网关、Agent 编排引擎、会话/任务调度、多租户身份与配额。
- **NEW** 建立 Python AI 计算面：NL2SQL Planner/Executor、Schema Linking、自我纠错循环，作为可水平扩缩的 worker pool。
- **NEW** 定义 **Go ↔ Python 跨语言契约**：以 gRPC + Protobuf 为主、可选 NATS/Kafka 用于异步长任务；统一在 `api/proto/` 维护。
- **NEW** OpenAI 兼容 LLM 网关：统一鉴权、限流、重试、Token 计量、成本归属到租户。
- **NEW** 数据连接层：支持 MySQL / PostgreSQL / ClickHouse / Doris 的元数据发现与查询执行；连接凭据通过租户级 Secret 管理。
- **NEW** 对话会话能力：多轮上下文、Schema 选择、修正反馈、可解释的中间步骤记录。
- **NEW** **Human-in-the-loop 审批工作流**：将「写回 SQL / 数据导出 / 跨敏感字段查询 / 高 Token 消耗动作」标记为需审批节点，由人工 approve/reject 后 Agent 才继续；全链路审计。
- **NEW** 多租户 SaaS 基线：工作区/用户/角色模型、JWT + API Key 双鉴权、租户级配额（QPS / Token / 存储）。
- **NEW** 可观测基线：跨 Go/Python 服务的 OpenTelemetry Trace/Metric/Log 统一上报，Agent 推理过程可回放。
- **NEW** K8s 交付：Helm Chart + Kustomize overlays，区分 `control-plane` / `ai-worker` / `gateway` 三类 Deployment，支持水平扩缩。
- **Non-goals（本变更不做）**：插件市场、可视化前端 UI（仅提供 OpenAPI，前端另立变更）、私有化部署专属能力、向量库/RAG（留给后续 `rag-knowledge` 变更）。

## Capabilities

### New Capabilities

- `agent-orchestration`：Go 实现的 Agent 运行时与状态机，负责任务编排、工具调用路由、与 Python worker 的 gRPC 协作，是控制面核心。
- `nl2sql-engine`：Python 实现的 NL2SQL 推理引擎，包含 schema linking、prompt 构造、SQL 生成、自我校验/重试，作为 stateless worker 暴露 gRPC 接口。
- `data-connectivity`：数据库连接器抽象，统一 MySQL/PostgreSQL/ClickHouse/Doris 的元数据发现与查询执行，含连接池、查询超时、行数上限保护。
- `conversation-session`：多轮对话会话与上下文管理，包含会话状态机、消息历史、中间推理步骤的可回放存储。
- `llm-gateway`：OpenAI 兼容协议的统一网关，提供 API Key 抽象、租户配额、限流熔断、Token 使用计量与成本归属。
- `tenant-identity`：多租户与身份能力，包含 workspace/user/role 数据模型、JWT + API Key 鉴权、最小可用 RBAC 策略与租户级配额执行。
- `human-approval`：Human-in-the-loop 审批工作流，定义敏感动作识别规则、待审任务队列、审批/驳回/超时策略、审计日志。
- `observability-telemetry`：跨语言可观测基线，定义 OpenTelemetry 接入规范、统一 trace context 跨 Go/Python 传播、Agent 推理过程结构化日志。
- `platform-runtime`：平台运行时与交付能力，含 Monorepo 目录约定、gRPC/Proto 契约组织、配置与 Secret 加载、Helm/Kustomize 部署清单、本地 docker-compose 开发栈。

### Modified Capabilities

无（项目首个变更，`openspec/specs/` 当前为空）。

## Impact

- **代码与目录**：建立 Monorepo 骨架 `cmd/` `internal/` `pkg/`（Go）+ `services/ai/` `services/workers/`（Python）+ `api/proto/` `deploy/helm/` `deploy/compose/` `docs/`。
- **API**：对外暴露 OpenAPI v1（REST + SSE）+ 内部 gRPC v1 契约；约定语义化版本。
- **依赖与技术栈**：
  - Go：`go-chi` 或 `gin`、`grpc-go`、`ent`/`gorm`、`zap`、`otelgrpc`、`viper`。
  - Python：`fastapi` 或纯 `grpcio`、`pydantic v2`、`sqlalchemy`、`openai` SDK、`opentelemetry-sdk`。
  - 中间件：PostgreSQL（元数据）、Redis（会话/限流）、对象存储（中间产物）；可选 NATS（异步任务）。
- **运维**：K8s 1.27+、Helm 3.x、Ingress + cert-manager；最小 3 节点集群即可起跑。
- **安全合规**：所有租户数据强制隔离至 schema 或行级 tenant_id；审计日志独立留存；敏感字段访问需走 `human-approval`。
- **跨团队**：需要数据库 DBA 配合提供测试库；后续接入企业 SSO 时需 IT 配合（本变更暂不涉及 SSO，仅留扩展点）。

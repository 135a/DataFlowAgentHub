## Why

项目当前存在 41 个已识别的问题，其中包括 3 个编译阻断级 Bug、5 个运行时阻断级 Bug 以及 6 个安全漏洞，导致项目无法正常编译、部署和运行。同时，核心 Multi-Agent 编排器中的 `nl2sql_node` 仍为 Mock 实现，知识库索引流程断裂，SSE 实时推送在浏览器端完全不可用。本次变更旨在修复所有阻断级问题、补齐核心功能缺口，并对齐"Go 底座 + Python 智能层"的架构愿景。

## What Changes

- **修复编译阻断 Bug**：补全 `InternalHMACSecret` 配置字段、修复 `tasks.go` 缺失的 import、清理 `knowledge.go` 未使用的 import
- **修复运行时阻断 Bug**：修复 `Dockerfile.ai` 缺失的 Python 模块拷贝、重新生成 gRPC 桩代码使其与 proto 同步、实现知识文档的 NATS 发布、修复 `consumer.py` 的 `NameError` 风险、将外部迁移文件嵌入 Go 自动迁移
- **安全加固**：加密存储数据源密码、消除硬编码开发密钥（改为强制环境变量校验）、修复报表下载路径遍历风险、关闭 ChromaDB `allow_reset`
- **补齐核心功能**：将 LangGraph 编排器的 `nl2sql_node` 从 Mock 替换为真实 gRPC 调用、完善知识索引全链路（上传→NATS→ChromaDB→状态回调）、修复 SSE 浏览器端认证问题（Token 传递方案）
- **建立测试基础设施**：为核心 Go 包添加单元测试、为 Python Agent 模块添加 pytest 测试
- **提升可靠性**：添加数据库迁移版本追踪、NATS 消费者消息确认机制、OTel 遥测导出、限流器滑动窗口算法
- **前端功能补全**：添加数据源管理页面、知识文档管理页面
- **对齐架构愿景**：完善 Go/Python 双向通信闭环、提升全链路可观测性

## Capabilities

### New Capabilities

- `testing-infrastructure`: Go 和 Python 核心模块的单元测试与集成测试基础设施
- `observability`: OpenTelemetry 链路追踪导出、Prometheus 指标完善、全链路日志联动
- `knowledge-pipeline`: 知识文档上传→NATS 异步分发→ChromaDB 向量化索引→状态回调的完整流程
- `frontend-management`: 数据源管理界面、知识文档管理界面、SSE 实时推送接收

### Modified Capabilities

- `nl2sql-engine`: `RunAgentPipeline` gRPC 客户端封装补全；LangGraph `nl2sql_node` 从 Mock 替换为真实 gRPC 调用；移除中文关键词硬编码路由，改为显式参数控制
- `data-connectivity`: 数据源密码加密存储（AES-256-GCM）；支持 MySQL 数据源类型；报表路径可配置化
- `schema-discovery`: 扩展 Schema 发现范围到所有 schema（非仅 `public`）；添加外部迁移文件的自动执行
- `security-hardening`: JWT 增加 `jti` 声明与黑名单吊销机制；内部 HMAC 签名用于服务间回调认证；消除所有硬编码密钥

## Impact

- **Go 后端**：`internal/config/`、`internal/handlers/`（全模块）、`internal/auth/`、`internal/worker/`、`internal/schema/`、`internal/migrate/`、`internal/async/`、`internal/ssebus/`、`internal/ratelimit/`、`internal/otelsetup/`、`internal/gen/`
- **Python AI Worker**：`services/ai/hub_ai/__main__.py`、`services/ai/orchestrator/`（全模块）、`services/ai/agents/`、`services/ai/rag/`
- **Proto & 生成代码**：`api/proto/nl2sql/v1/nl2sql.proto` 及重新生成的 Go/Python 桩代码
- **Docker 部署**：`Dockerfile.ai`、`Dockerfile.api`、`docker-compose.yml`
- **前端**：`web/src/`（App.tsx、新增页面组件、api.ts SSE 支持）
- **数据库**：新增迁移文件，调整 `data_sources` 表结构
- **依赖变更**：`requirements.txt` 补全 `tabulate`；Go 端无新增依赖

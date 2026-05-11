## ADDED Requirements

### Requirement: Monorepo 目录约定

仓库 SHALL 采用清晰边界目录：`cmd/`、`internal/`、`pkg/`（Go），`services/ai/`（Python worker），`api/proto/`（契约），`web/`（前端），`deploy/compose/`（Compose），`docs/`（运行与演示说明）。

#### Scenario: 新贡献者可本地启动

- **WHEN** 贡献者按 `docs/` 中步骤在本地运行 `docker compose`
- **THEN** 文档 MUST 列出所需环境变量清单，且 Compose MUST 能启动 postgres、redis、api、ai-worker 与 web 相关服务（具体服务名允许实现调整但能力不变）

### Requirement: 官方交付不含 Kubernetes

平台运行时规范 SHALL 以 Docker Compose 作为唯一官方编排交付；文档与默认脚本 MUST NOT 将 Kubernetes 列为必需路径。

#### Scenario: 无 K8s 仍可完整演示

- **WHEN** 用户仅具备单机 Docker 环境
- **THEN** 用户 MUST 能完成端到端演示（前端 + API + worker + 元数据库）而无需安装 kubectl

### Requirement: 生产单机部署指引

系统 SHALL 在文档中描述单机服务器部署：Compose 运行、反向代理 TLS、SSE 反代缓冲关闭检查项、`.env` 密钥管理建议。

#### Scenario: SSE 经反代可用

- **WHEN** 用户按文档配置 Nginx 或 Caddy 反代 SSE
- **THEN** 文档 MUST 明确禁用缓冲或等价配置要求，并给出最小可用示例片段

### Requirement: 健康检查与版本暴露

API 服务 SHALL 提供 `/health`（或等价）返回依赖就绪状态；系统 MUST 暴露应用版本/build 信息供排障（不得泄露密钥）。

#### Scenario: 依赖未就绪时健康检查失败

- **WHEN** PostgreSQL 不可达
- **THEN** `/health` MUST 报告非就绪状态以便编排器重启或告警

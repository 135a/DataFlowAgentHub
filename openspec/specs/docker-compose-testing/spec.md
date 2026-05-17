# Docker Compose Idempotency Testing

## Purpose

验证 DataFlowAgentHub 全栈 Docker Compose 部署在多轮 up/down 循环中的稳定性和幂等性，确保演示和部署的可靠性。

## Requirements

### Requirement: 多轮 up/down 服务全部启动成功

系统 SHALL 在至少 3 轮 `docker compose up -d --build` / `docker compose down` 循环中，每次 up 后所有 7 个服务（postgres、redis、chroma、nats、ai-worker、api、web）均处于 `running` 或 `healthy` 状态。

#### Scenario: 首次启动

- **WHEN** 执行 `docker compose up -d --build` 从干净状态启动
- **THEN** 所有 7 个服务在 120 秒内达到 `running` 状态，`docker compose ps -a` 输出中无 `exited` 或 `restarting` 状态

#### Scenario: 第二轮启动（模拟重启）

- **WHEN** 在首轮 `docker compose down` 之后再次执行 `docker compose up -d --build`
- **THEN** 所有服务在 120 秒内恢复 `running` 状态，数据库数据可正常读写，API 返回 200 健康检查

#### Scenario: 第三轮启动（验证幂等性）

- **WHEN** 在第二轮 `docker compose down` 之后第三次执行 `docker compose up -d --build`
- **THEN** 服务状态与首轮一致，无端口冲突、无数据卷挂载冲突、无残留 PID 文件

### Requirement: 数据持久化验证

系统 SHALL 在 `docker compose down`（不带 `-v`）后再 `up` 时，保留上一轮的数据库记录。

#### Scenario: 数据跨轮次保留

- **WHEN** 首轮启动后写入测试数据（如创建用户），执行 `docker compose down`，再 `docker compose up -d --build`
- **THEN** 之前写入的数据仍可查询，Postgres 和 Redis 数据未被清空

### Requirement: 服务依赖启动顺序

系统 SHALL 按 postgres → redis/nats/chroma → ai-worker → api → web 的顺序稳定启动，不做无序重试。

#### Scenario: 依赖就绪后启动

- **WHEN** `docker compose up -d --build` 执行
- **THEN** api 服务在 postgres、redis、ai-worker、nats 全部就绪后才开始处理请求，web 在 api 就绪后启动

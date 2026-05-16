## MODIFIED Requirements

### Requirement: 连接器元数据发现

系统 SHALL 支持对目标库进行受限元数据发现（catalog/schema/table/column 基础集合），供 NL2SQL 上下文构建使用。Go 控制面 MUST 在调用 NL2SQL worker 之前执行 schema 发现；发现结果 MUST 以结构化 JSON 格式注入 gRPC 请求。MUST 支持 Redis 缓存以降低重复扫描开销，缓存 TTL 可配置（默认 300 秒）。

#### Scenario: 元数据发现失败可恢复

- **WHEN** 目标库临时不可达或权限不足
- **THEN** 系统 MUST 返回明确错误并阻止进入执行阶段，且 MUST NOT 缓存错误状态为成功元数据

#### Scenario: 通过 information_schema 获取完整列信息

- **WHEN** Go 控制面连接至已注册的 Postgres 数据源
- **THEN** 系统 MUST 查询 `information_schema.columns WHERE table_schema = 'public'` 并按表名聚合列信息，返回包含表名、列名、数据类型的结构化结果

## ADDED Requirements

### Requirement: 数据源密码加密存储

系统 SHALL 使用 AES-256-GCM 对称加密存储 `data_sources` 表中的密码字段，加密密钥 MUST 来自环境变量 `HUB_DB_ENCRYPTION_KEY`。

#### Scenario: 创建数据源时加密密码

- **WHEN** Go API 创建新数据源记录
- **THEN** 密码 MUST 使用 AES-256-GCM 加密后以 base64 格式写入数据库，MUST NOT 以明文形式出现在任何日志或 API 响应中

#### Scenario: 使用数据源时解密密码

- **WHEN** Go API 需要连接外部数据源（如 schema 发现或查询执行）
- **THEN** 系统 MUST 从数据库读取密文，使用 `HUB_DB_ENCRYPTION_KEY` 解密后才建立连接

#### Scenario: 加密密钥未设置时拒绝启动

- **WHEN** `HUB_DB_ENCRYPTION_KEY` 环境变量为空或长度不足 32 字节
- **THEN** `config.Load()` MUST 返回明确错误，API 服务 MUST 拒绝启动

### Requirement: 数据源密码 API 响应保护

数据源列表和详情 API 响应 SHALL 永远不返回明文或密文密码，MUST 仅返回 `has_password: true/false` 布尔字段。

#### Scenario: 列出数据源时不泄露密码

- **WHEN** 用户调用 `GET /v1/data-sources`
- **THEN** 响应中每个数据源 MUST 仅包含 `has_password` 布尔字段，MUST NOT 包含 `password` 字段

### Requirement: 报表路径可配置

报表文件存储路径 SHALL 通过环境变量 `HUB_REPORTS_DIR` 配置，MUST 有合理的默认值。

#### Scenario: 使用自定义报表路径

- **WHEN** 环境变量 `HUB_REPORTS_DIR` 设置为 `/data/reports`
- **THEN** 报表下载和生成 MUST 使用 `/data/reports/` 作为文件根目录

#### Scenario: 默认报表路径

- **WHEN** 环境变量 `HUB_REPORTS_DIR` 未设置
- **THEN** 系统 MUST 使用 `os.TempDir() + "/hub-reports/"` 作为默认路径

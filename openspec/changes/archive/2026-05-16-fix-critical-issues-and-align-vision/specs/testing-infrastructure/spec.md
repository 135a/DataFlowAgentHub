## ADDED Requirements

### Requirement: Go 核心模块单元测试

Go 后端的 `internal/auth/`、`internal/sqlrun/`、`internal/schema/`、`internal/config/` 包 SHALL 拥有单元测试覆盖，MUST 覆盖正常路径与关键错误路径。

#### Scenario: JWT 签发与解析往返

- **WHEN** 使用 `auth.SignJWT` 签发的 Token 被 `auth.ParseJWT` 解析
- **THEN** 解析结果 MUST 包含签发时写入的全部 Claims（UserID、WorkspaceID、Role）

#### Scenario: SQL 只读检测拒绝写操作

- **WHEN** 输入包含 INSERT/UPDATE/DELETE/DROP/TRUNCATE/ALTER 语句的 SQL
- **THEN** `sqlrun.IsReadOnlySQL()` MUST 返回 false

#### Scenario: SQL 只读检测放行只读查询

- **WHEN** 输入仅包含 SELECT 语句的 SQL
- **THEN** `sqlrun.IsReadOnlySQL()` MUST 返回 true

#### Scenario: Config 缺少必填项时返回错误

- **WHEN** 环境变量中未设置 `HUB_JWT_SECRET` 且未设置 `HUB_INTERNAL_HMAC_SECRET`
- **THEN** `config.Load()` MUST 返回非 nil 错误

### Requirement: Python 模块单元测试

Python AI Worker 的 `agents/` 和 `rag/` 模块 SHALL 拥有 pytest 单元测试覆盖。

#### Scenario: Data Analysis Agent 处理空数据

- **WHEN** `data_analysis_agent` 收到空的 DataFrame（0 行）
- **THEN** Agent MUST 返回包含 `analysis_summary` 和 `statistics` 字段的 dict，且不抛出异常

#### Scenario: RAG 文档分块正确性

- **WHEN** 输入长度超过 chunk_size 的文本文档
- **THEN** `knowledge_base` 的分块函数 MUST 生成至少 2 个 chunk，且每个 chunk 不超过配置的 chunk_size

### Requirement: 测试可运行性

`make test` 命令 SHALL 正确执行所有 Go 测试，`cd services/ai && pytest` SHALL 正确执行所有 Python 测试。

#### Scenario: make test 无测试时输出

- **WHEN** 在项目根目录运行 `make test`
- **THEN** 命令 MUST 退出码为 0（除非测试失败），并输出测试摘要

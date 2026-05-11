## ADDED Requirements

### Requirement: 受控执行只读查询

数据连接层 SHALL 使用只读数据库凭据执行查询，并且 MUST 强制查询超时与最大返回行数上限。

#### Scenario: 超时与行数上限生效

- **WHEN** 下游查询执行时间超过配置上限或结果行数超过上限
- **THEN** 系统 MUST 中止查询并返回可区分错误（超时 vs 行数截断策略按实现文档固定一种），且 MUST 记录审计事件

### Requirement: 连接器元数据发现

系统 SHALL 支持对目标库进行受限元数据发现（catalog/schema/table/column 基础集合），供 NL2SQL 上下文构建使用。

#### Scenario: 元数据发现失败可恢复

- **WHEN** 目标库临时不可达或权限不足
- **THEN** 系统 MUST 返回明确错误并阻止进入执行阶段，且 MUST NOT 缓存错误状态为成功元数据

### Requirement: 凭据与连接配置存储

连接凭据 MUST 以安全默认方式存储：仅服务端持有，前端 MUST NOT 接收完整明文密码回显；面试版允许单租户配置但 MUST 保持该不变量。

#### Scenario: API 不回显密钥

- **WHEN** 客户端查询已保存的数据源配置
- **THEN** 响应 MUST 掩码敏感字段或仅返回是否存在配置的指示，不得返回完整密码原文

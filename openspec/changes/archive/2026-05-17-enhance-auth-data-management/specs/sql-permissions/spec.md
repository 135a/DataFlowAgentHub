## ADDED Requirements

### Requirement: SQL 操作分类

系统 SHALL 对用户提交的 SQL 语句进行分类，识别操作类型。

#### Scenario: 分类 SELECT
- **WHEN** 用户提交以 SELECT 开头的 SQL
- **THEN** 系统识别为 SELECT 类型

#### Scenario: 分类 INSERT
- **WHEN** 用户提交以 INSERT 开头的 SQL
- **THEN** 系统识别为 INSERT 类型

#### Scenario: 分类 CREATE TABLE
- **WHEN** 用户提交以 CREATE TABLE 开头的 SQL
- **THEN** 系统识别为 CREATE_TABLE 类型

#### Scenario: 分类 CREATE DATABASE
- **WHEN** 用户提交以 CREATE DATABASE 开头的 SQL
- **THEN** 系统识别为 CREATE_DATABASE 类型

### Requirement: 角色权限矩阵

系统 SHALL 按以下矩阵控制 SQL 执行权限：

| 操作类型 | viewer | operator | admin |
|----------|--------|----------|-------|
| SELECT | ✓ | ✓ | ✓ |
| INSERT | ✗ | ✓ | ✓ |
| UPDATE | ✗ | ✓ | ✓ |
| CREATE TABLE | ✗ | ✓ | ✓ |
| CREATE DATABASE | ✗ | ✓ | ✓ |
| DELETE | ✗ | ✗ | ✓ |
| DROP TABLE | ✗ | ✗ | ✓ |
| DROP DATABASE | ✗ | ✗ | ✓ |
| ALTER | ✗ | ✗ | ✓ |
| TRUNCATE | ✗ | ✗ | ✓ |

#### Scenario: operator 执行 INSERT 成功
- **WHEN** operator 角色用户提交 INSERT 语句
- **THEN** 系统执行并通过

#### Scenario: viewer 执行 INSERT 被拒
- **WHEN** viewer 角色用户提交 INSERT 语句
- **THEN** 系统返回 403

#### Scenario: operator 执行 DELETE 被拒
- **WHEN** operator 角色用户提交 DELETE 语句
- **THEN** 系统返回 403

#### Scenario: admin 执行 DROP 成功
- **WHEN** admin 角色用户提交 DROP TABLE 语句
- **THEN** 系统执行并通过

### Requirement: 系统表黑名单

系统 SHALL 保护以下系统表，任何用户（含 admin）不可读写：

users, workspaces, sessions, messages, runs, audit_events, async_tasks, knowledge_docs, data_sources, agent_run_steps

#### Scenario: 查询系统表被拒
- **WHEN** 用户提交 SELECT * FROM users
- **THEN** 系统返回 403

#### Scenario: schema 发现不包含系统表
- **WHEN** 用户访问表结构页面
- **THEN** 系统不展示任何系统表

#### Scenario: 系统表名不在 schema 结果中
- **WHEN** AI 生成 SQL 时获取 schema 上下文
- **THEN** schema JSON 中不包含系统表

# Handler Tests

## Purpose

为 Go HTTP handler 层提供集成测试覆盖，使用 `net/http/httptest` 和真实数据库连接验证端点行为。

## Requirements

### Requirement: 数据源 CRUD handler 测试

系统 SHALL 为 `handlers/datasources.go` 中的 Create、List、Update、Delete 端点提供测试覆盖。

#### Scenario: 创建数据源缺少字段返回 400

- **WHEN** operator 角色的用户向 `POST /v1/data-sources` 发送不完整的数据源配置
- **THEN** 系统返回 400 Bad Request

#### Scenario: viewer 创建数据源被拒绝

- **WHEN** viewer 角色的用户向 `POST /v1/data-sources` 发送有效配置
- **THEN** 系统返回 403 Forbidden

### Requirement: 用户管理 handler 测试

系统 SHALL 为 `handlers/auth.go` 中的 Register、ListUsers、ChangeUserRole、DeleteUser 端点提供测试覆盖。

#### Scenario: 创建用户成功

- **WHEN** admin 角色的用户向 `POST /v1/auth/register` 发送有效的用户信息
- **THEN** 系统返回 201 并创建用户记录

#### Scenario: 注册用户缺失字段返回 400

- **WHEN** 发送缺少必填字段的注册请求
- **THEN** 系统返回 400 Bad Request

### Requirement: 知识文档和数据上传 handler 测试

系统 SHALL 为知识文档列表和文件上传端点提供测试覆盖。

#### Scenario: operator 查看知识文档列表

- **WHEN** operator 角色的用户请求知识文档列表
- **THEN** 系统返回 200

#### Scenario: viewer 上传文件被拒绝

- **WHEN** viewer 角色的用户向 `POST /v1/data/upload` 发送请求
- **THEN** 系统返回 403 Forbidden

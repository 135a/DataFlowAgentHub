## ADDED Requirements

### Requirement: 数据源 CRUD handler 测试

系统 SHALL 为 `handlers/datasources.go` 中的 Create、List、Update、Delete、Test 端点提供测试覆盖，使用 `net/http/httptest` 和 mock 依赖。

#### Scenario: 创建数据源成功

- **WHEN** operator 角色的用户向 `POST /v1/data-sources` 发送有效的数据源配置
- **THEN** 系统返回 201 并在数据库中创建数据源记录

#### Scenario: 删除数据源成功

- **WHEN** admin 角色的用户向 `DELETE /v1/data-sources/{id}` 发送请求
- **THEN** 系统返回 200 并软删除该数据源

### Requirement: 用户管理 handler 测试

系统 SHALL 为 `handlers/users.go` 中的 Create、List、UpdateRole、Delete 端点提供测试覆盖。

#### Scenario: 创建用户成功

- **WHEN** admin 角色的用户向 `POST /v1/users` 发送有效的用户信息
- **THEN** 系统返回 201 并创建用户记录

### Requirement: 内部回调 handler 测试

系统 SHALL 为 `handlers/internal.go` 中的异步任务回调端点提供测试覆盖，验证 HMAC 签名校验逻辑。

#### Scenario: 有效签名的回调成功

- **WHEN** Python worker 使用有效 HMAC 签名向 `/internal/tasks/{id}/callback` 发送结果
- **THEN** 系统更新 async_task 状态并推送 SSE 事件

#### Scenario: 无效签名被拒绝

- **WHEN** 请求的 `X-Hub-Signature` 与计算的 HMAC 不匹配
- **THEN** 系统返回 HTTP 401 Unauthorized

### Requirement: 知识文档 handler 测试

系统 SHALL 为 `handlers/data.go` 中的知识文档上传和搜索端点提供测试覆盖。

#### Scenario: 上传 Markdown 文档成功

- **WHEN** operator 角色的用户向知识文档上传端点发送有效的 Markdown 文件
- **THEN** 系统返回 201 并触发 RAG 索引流程

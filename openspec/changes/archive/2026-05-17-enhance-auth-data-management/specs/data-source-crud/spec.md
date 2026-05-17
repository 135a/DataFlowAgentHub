## ADDED Requirements

### Requirement: 查看数据源列表

所有登录用户 SHALL 可以查看数据源列表（不含密码）。

#### Scenario: 查看列表
- **WHEN** 用户访问 /data-sources 页面
- **THEN** 系统展示所有数据源（名称、类型、主机、端口、数据库名、连接状态）

### Requirement: 编辑数据源

admin SHALL 可以编辑数据源的连接参数。

#### Scenario: 编辑连接参数
- **WHEN** admin 修改数据源的主机、端口、数据库名、用户名或密码
- **THEN** 系统更新数据源配置

#### Scenario: Non-admin 编辑被拒
- **WHEN** operator 或 viewer 尝试编辑数据源
- **THEN** 系统返回 403

### Requirement: 删除数据源

admin SHALL 可以删除数据源。

#### Scenario: 删除数据源
- **WHEN** admin 删除数据源
- **THEN** 系统删除数据源记录

#### Scenario: Non-admin 删除被拒
- **WHEN** operator 或 viewer 尝试删除数据源
- **THEN** 系统返回 403

### Requirement: 测试连通性

operator 及以上角色 SHALL 可以测试数据源连接是否正常。

#### Scenario: 测试连接成功
- **WHEN** operator 对已配置的数据源执行测试连接
- **THEN** 系统尝试连接并返回成功/失败状态

#### Scenario: 测试连接 UI
- **WHEN** 用户在数据源列表页点击"测试连接"
- **THEN** 前端显示连接测试结果

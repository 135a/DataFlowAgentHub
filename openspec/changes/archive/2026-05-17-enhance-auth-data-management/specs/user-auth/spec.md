## ADDED Requirements

### Requirement: 手机号登录

系统 SHALL 支持用户使用手机号 + 密码登录。

#### Scenario: 手机号登录成功
- **WHEN** 用户输入已注册的手机号和正确密码
- **THEN** 系统返回 JWT access_token，包含用户 ID、工作区 ID、角色

#### Scenario: 手机号未注册
- **WHEN** 用户输入未注册的手机号
- **THEN** 系统返回 401 和"手机号未注册"错误

#### Scenario: 密码错误
- **WHEN** 用户输入已注册的手机号和错误密码
- **THEN** 系统返回 401 和"密码错误"

### Requirement: Admin 创建用户

仅 admin 角色可以创建新用户，新用户可以是 operator 或 viewer 角色。

#### Scenario: Admin 创建 operator 用户
- **WHEN** admin 填写姓名、手机号、密码、选择 operator 角色并提交
- **THEN** 系统创建用户并返回成功

#### Scenario: Admin 创建 viewer 用户
- **WHEN** admin 填写姓名、手机号、密码、选择 viewer 角色并提交
- **THEN** 系统创建用户并返回成功

#### Scenario: Non-admin 尝试创建用户
- **WHEN** operator 或 viewer 角色用户访问创建用户接口
- **THEN** 系统返回 403

#### Scenario: 手机号已存在
- **WHEN** admin 输入已存在的手机号创建用户
- **THEN** 系统返回 409 和"手机号已存在"错误

### Requirement: Admin 管理用户

admin SHALL 可以查看用户列表、修改用户角色、删除用户。

#### Scenario: 查看用户列表
- **WHEN** admin 访问用户管理页面
- **THEN** 系统展示所有用户（姓名、手机号、角色、创建时间）

#### Scenario: 修改用户角色
- **WHEN** admin 修改非 admin 用户的角色
- **THEN** 系统更新用户角色并返回成功

#### Scenario: 删除用户
- **WHEN** admin 删除非 admin 用户
- **THEN** 系统删除用户并返回成功

#### Scenario: Admin 不能删除自己
- **WHEN** admin 尝试删除自己的账号
- **THEN** 系统返回 400 和"不能删除自己"

### Requirement: 角色权限层级

系统 SHALL 按 admin(3) > operator(2) > viewer(1) 的层级控制 API 访问权限。

#### Scenario: 低角色用户被拒绝
- **WHEN** viewer 用户访问要求 operator 或 admin 角色的 API
- **THEN** 系统返回 403

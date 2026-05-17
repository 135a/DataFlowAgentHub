## ADDED Requirements

### Requirement: 表结构浏览页面

所有登录用户 SHALL 可以通过 `/tables` 页面查看业务表的字段结构（不含数据行）。

#### Scenario: 查看表列表
- **WHEN** 用户访问 /tables 页面
- **THEN** 系统展示所有业务表的卡片列表，每张卡片显示表名、行数、最后更新时间

#### Scenario: 查看表字段详情
- **WHEN** 用户点击展开某张表
- **THEN** 系统展示该表的所有字段（字段名、类型、约束、默认值）

#### Scenario: 搜索表名
- **WHEN** 用户在搜索框输入表名关键词
- **THEN** 系统过滤并展示匹配的表

#### Scenario: 不包含数据行
- **WHEN** 用户查看表结构
- **THEN** 系统不展示任何数据行

#### Scenario: 不包含系统表
- **WHEN** 用户访问 /tables 页面
- **THEN** 系统不展示任何系统表

### Requirement: 美观的前端展示

表结构页面 SHALL 以卡片式布局优雅展示。

#### Scenario: 卡片式布局
- **WHEN** 用户访问 /tables
- **THEN** 每张表以独立卡片展示，可折叠展开

#### Scenario: 字段表格样式
- **WHEN** 用户展开表卡片
- **THEN** 字段以格式化表格展示，类型和约束使用标签/徽标样式

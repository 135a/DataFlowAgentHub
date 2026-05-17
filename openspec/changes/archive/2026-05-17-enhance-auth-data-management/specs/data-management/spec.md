## ADDED Requirements

### Requirement: 文件上传导入数据

operator 及以上角色 SHALL 可以通过上传文件的方式向目标表导入数据。

#### Scenario: 上传 CSV 导入数据
- **WHEN** operator 选择目标表、上传 UTF-8 编码的 CSV 文件、点击导入
- **THEN** 系统解析 CSV，AI 校验列名匹配，执行 INSERT

#### Scenario: 上传 XLSX 导入数据
- **WHEN** operator 选择目标表、上传 .xlsx 文件
- **THEN** 系统解析第一个 sheet，AI 校验列名匹配，执行 INSERT

#### Scenario: 上传 SQL 执行
- **WHEN** operator 上传 .sql 文件
- **THEN** 系统按 ; 分割语句，逐条分类和权限检查，执行

#### Scenario: 列名不匹配被拒绝
- **WHEN** 上传的 CSV 列名在目标表中不存在
- **THEN** AI 判断是否笔误。是笔误则拒绝并提示建议列名，否则拒绝并报列不存在

#### Scenario: 数据格式错误被拒绝
- **WHEN** 文件中某列的值与目标表类型不兼容
- **THEN** 系统拒绝执行，返回第 X 行第 Y 列、期望类型、实际值和修改建议

#### Scenario: Viewer 看不到数据管理区域
- **WHEN** viewer 角色用户访问主页面
- **THEN** 数据管理区域不显示

### Requirement: AI 辅助建表

用户 SHALL 可以通过文字描述让 AI 设计新表结构，经用户确认后创建。

#### Scenario: AI 建议建表
- **WHEN** 用户输入文字描述需要存储的数据
- **THEN** AI 返回建议的表结构（表名、字段名、类型、约束、示例值）

#### Scenario: 用户确认建表
- **WHEN** AI 展示建议表结构后用户点击确认
- **THEN** 系统执行 CREATE TABLE

#### Scenario: 用户拒绝建表
- **WHEN** AI 展示建议表结构后用户点击拒绝
- **THEN** 系统不执行任何操作

#### Scenario: 建表后不可回退
- **WHEN** CREATE TABLE 执行成功
- **THEN** 页面提示"表已创建，此操作不可回退"

#### Scenario: 建表需显示完整 DDL
- **WHEN** AI 展示建表方案
- **THEN** 前端完整展示 CREATE TABLE SQL 语句

### Requirement: 文件 UPDATE 支持

operator 及以上角色 SHALL 可以通过上传文件更新现有数据。

#### Scenario: 上传 CSV 更新数据
- **WHEN** 用户选择"更新数据"模式、上传包含主键列和更新列的 CSV
- **THEN** AI 校验列存在后执行 UPDATE

#### Scenario: 更新缺少主键列
- **WHEN** 上传的文件中没有目标表的主键/唯一键列
- **THEN** 系统拒绝并提示需要包含主键列

### Requirement: AI 提示辅助

用户 SHALL 可以在上传文件时输入自然语言描述，辅助 AI 理解文件语义。

#### Scenario: AI 使用用户提示
- **WHEN** 用户上传 CSV 时填写 AI 提示 "amt 列是金额"
- **THEN** AI 在校验时使用此提示辅助列名匹配

### Requirement: 数据管理快速模式/深度模式

数据管理区域 SHALL 支持与查询相同的快速/深度模式选择。

#### Scenario: 快速模式
- **WHEN** 用户选择快速模式
- **THEN** 文件校验使用规则引擎（列名精确匹配、类型简单检查）

#### Scenario: 深度模式
- **WHEN** 用户选择深度模式
- **THEN** 文件校验调用 LLM 进行智能判断（笔误识别、语义匹配）

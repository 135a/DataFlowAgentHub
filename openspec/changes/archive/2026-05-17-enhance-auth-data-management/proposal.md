## Why

项目目前是单用户 MVP 状态：认证只有 email 登录无注册、SQL 执行只有只读限制无分级权限、数据只能查询无法写入、系统表和业务表混在一起没有隔离。需要在 MVP 基础上构建完整的产品级能力：多用户认证体系、分级 SQL 权限、文件导入写入能力、以及配套的前端管理页面。

## What Changes

- **认证系统改造**：从 email 登录改为姓名（手机号）+ 密码，仅 admin 可在 Web UI 上创建和管理用户
- **新增用户角色管理**：admin（唯一） / operator（多个）/ viewer（多个），admin 可修改角色和删除用户
- **SQL 分级权限**：废除单一的只读/写判断，改为按操作类型（SELECT/INSERT/UPDATE/DELETE/CREATE/DROP）+ 角色等级的权限矩阵
- **系统表保护**：schema 发现和 SQL 执行双路拦截系统表（users, workspaces 等），任何用户不可访问
- **数据管理页面**：operator+ 可通过文件上传（.csv/.xlsx/.sql）导入/更新数据，AI 预检列名和数据质量
- **AI 辅助建表**：用户文字描述数据，AI 设计表结构，用户阅读确认后创建（不可回退）
- **数据库表浏览器**：所有登录用户可查看业务表的字段结构（不含数据行），前端美观展示
- **数据源管理补齐**：新增编辑、删除、测试连接 UI
- **知识库文件上传**：支持上传 .md 文件（不局限于文本框粘贴）
- **页面结构调整**：统一为单页面，数据管理区域仅 operator+ 可见，新增 `/tables` 页面

## Capabilities

### New Capabilities
- `user-auth`: 手机号登录、admin 创建用户、角色管理（admin/operator/viewer）
- `sql-permissions`: SQL 操作分类、角色权限矩阵、系统表黑名单保护
- `data-management`: 文件上传导入（.csv/.xlsx/.sql）、AI 预检列名和数据质量、AI 辅助建表及用户确认
- `table-browser`: 业务表结构可视化浏览（仅字段，不含数据）
- `data-source-crud`: 数据源连接的编辑、删除、测试连通性
- `knowledge-file-upload`: 上传 .md 文件到知识库（非仅文本框粘贴）

### Modified Capabilities

无。所有能力均为新增。

## Impact

- **internal/sqlrun/**：核心重构，从 `IsReadOnlySQL` 改为 `ClassifySQL` + 角色权限检查 + 系统表拦截
- **internal/schema/**：schema 发现增加系统表过滤
- **internal/handlers/**：新增数据管理 handler、用户管理 handler、数据源编辑/删除 handler
- **internal/seed/**：种子账号改为手机号模式
- **internal/auth/**：JWT claims 不变，登录方式扩展支持手机号
- **internal/llm/**：复用现有 LLM 客户端进行文件导入的 AI 预检
- **web/**：新增 `/tables` 页面、数据管理区域、用户管理页面（仅 admin）、数据源编辑/删除 UI、知识库文件上传
- **services/ai/**：可能需要扩展 xlsx 解析能力
- **数据库 migration**：users 表新增 phone 字段，新增 schema 隔离或黑名单机制

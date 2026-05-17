## 1. sqlrun 重构 — SQL 分类 + 权限矩阵 + 系统表保护

- [x] 1.1 `internal/sqlrun/` 新增 `ClassifySQL()` 函数，基于关键词前缀识别 SQL 操作类型
- [x] 1.2 `internal/sqlrun/` 新增 `IsAllowedForRole(sqlType, role)` 函数，按权限矩阵判断角色是否有权执行
- [x] 1.3 `internal/sqlrun/` 新增系统表黑名单 `IsSystemTable(tableName)` + `CheckSystemTableInSQL()`
- [x] 1.4 重构 `sqlrun/run.go`：保留 `QueryRows()`，新增 `ExecuteWrite()` 写执行路径
- [x] 1.5 写执行路径集成系统表拦截：任何操作命中系统表名 → 403

## 2. schema 发现过滤

- [x] 2.1 `internal/schema/discovery.go` 的 `DiscoverSchema()` 过滤系统表名
- [x] 2.2 `CachedSchema()` 复用过滤逻辑，无需新增函数

## 3. 后端 Handler — 用户管理

- [x] 3.1 新增 `POST /v1/auth/register` — admin 创建用户（姓名、手机号、密码、角色），仅 admin 可调用
- [x] 3.2 新增 `GET /v1/users` — admin 获取用户列表
- [x] 3.3 新增 `PUT /v1/users/{id}/role` — admin 修改用户角色
- [x] 3.4 新增 `DELETE /v1/users/{id}` — admin 删除用户（不能删自己、不能删 admin）
- [x] 3.5 修改 `Login` handler：支持手机号代替 email 登录，登录响应返回角色
- [x] 3.6 数据库 migration：users 表新增 `phone TEXT` 字段
- [x] 3.7 数据库 migration：users 表新增 `name TEXT` 字段

## 4. 后端 Handler — 数据源管理补齐

- [x] 4.1 新增 `PUT /v1/data-sources/{id}` — admin 编辑数据源连接参数
- [x] 4.2 新增 `DELETE /v1/data-sources/{id}` — admin 删除数据源
- [x] 4.3 现有 `TestDataSource` handler 已存在，确认路由和权限正确

## 5. 后端 Handler — 数据管理

- [x] 5.1 新增 `POST /v1/data/upload` — 接收文件（.csv/.xlsx/.sql）+ 目标表 + 操作类型 + AI 提示
- [x] 5.2 CSV 解析：读取第一行列名，后续行数据
- [x] 5.3 XLSX 解析：使用 excelize 库解析第一个 sheet
- [x] 5.4 SQL 文件解析：按 ; 分割，逐条分类和权限检查
- [x] 5.5 AI 列名校验：调用 `internal/llm` 进行列名匹配和数据质量检查
- [x] 5.6 AI 建表：用户文字描述 → AI 生成表结构 → 返回前端确认
- [x] 5.7 建表确认端点 `POST /v1/data/create-table` — 用户确认后执行 CREATE TABLE
- [x] 5.8 文件上传执行路径整合权限检查（`ClassifySQL` + `IsAllowedForRole` + 系统表拦截）

## 6. 前端 — 导航与页面结构调整

- [x] 6.1 导航栏新增 [数据库表] 链接指向 `/tables`
- [x] 6.2 导航栏已有 [数据源] 和 [知识库] 不变
- [x] 6.3 主页面结构调整：数据管理区域默认折叠，仅 operator+ 角色可见

## 7. 前端 — 数据库表结构页面 `/tables`

- [x] 7.1 创建 `web/src/pages/TablesPage.tsx` — 卡片式布局，每张表一个卡片
- [x] 7.2 卡片内展示字段表格（字段名、类型、约束、默认值）
- [x] 7.3 表名搜索过滤
- [x] 7.4 数据行数和最后更新时间展示
- [x] 7.5 路由注册 `/tables` + 后端 `GET /v1/schema/tables`

## 8. 前端 — 数据管理区域

- [x] 8.1 主页面数据管理 UI：操作类型选择（导入/更新/创建新表）、目标表下拉、文件上传
- [x] 8.2 AI 建表确认弹窗：展示 AI 建议的表结构和 DDL，用户阅读后确认/拒绝
- [x] 8.3 文件导入执行结果展示（AI 校验报告、执行成功/失败详情）
- [x] 8.4 仅 operator+ 可见此区域，viewer 不渲染

## 9. 前端 — 数据源编辑/删除/测试 UI

- [x] 9.1 数据源列表页新增"编辑"按钮（仅 admin），弹出编辑表单
- [x] 9.2 数据源列表页新增"删除"按钮（仅 admin），带二次确认弹窗
- [x] 9.3 数据源列表页新增"测试连接"按钮（operator+），显示测试结果

## 10. 前端 — 用户管理页面 `/admin/users`

- [x] 10.1 创建 `web/src/pages/AdminUsersPage.tsx` — 用户列表示意
- [x] 10.2 新建用户表单（姓名、手机号、密码、角色选择）
- [x] 10.3 修改角色、删除用户操作（带确认）
- [x] 10.4 仅 admin 可访问，路由注册 `/admin/users`

## 11. 前端 — 知识库文件上传

- [x] 11.1 `KnowledgePage.tsx` 新增 .md 文件选择上传
- [x] 11.2 保留现有文本框上传方式

## 12. 验证

- [x] 12.1 `go build ./cmd/api` 编译通过
- [x] 12.2 `npm run build` 前端编译通过
- [x] 12.3 `go test ./...` 全部通过
- [ ] 12.4 手动冒烟：登录、建表、文件导入、表结构浏览

## 13. NL2SQL 聊天写支持（高优先级）

- [x] 13.1 `executor.go` — `Execute()` 按 `ClassifySQL` 分类路由：SELECT→QueryRows，写操作→ExecuteWrite
- [x] 13.2 `executor.go` — 写操作路径集成 `IsAllowedForRole` + `CheckSystemTableInSQL`
- [x] 13.3 `Input` 结构体新增 `Role` 字段，`PostMessage` 传入 `c.Role`
- [x] 13.4 响应处理 — 写操作返回 `rows_affected` + `type: "write"`
- [x] 13.5 Python ai-worker — 移除 `_read_only_ok` 关键字拦截，prompt 放开写操作
- [x] 13.6 Python ai-worker — prompt 禁止 AI 编造数据，要求提供具体值；支持 `ERROR:` 前缀返回澄清问题
- [x] 13.7 SSE 进度事件 — `run_started` 包含 `started_at`，`sql_generated` 包含 `elapsed_ms` + `is_write`
- [x] 13.8 SSE 结果事件 — `result` 包含 `elapsed_ms` + `started_at`，写操作额外含 `rows_affected`

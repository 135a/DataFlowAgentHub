## Context

项目当前处于单用户 MVP 阶段，认证、权限、数据写入等能力均未完善。本次变更是产品化的关键一步，涉及认证系统重构、SQL 执行权限分级、数据写入能力引入、以及配套前端界面。

当前状态：
- 认证：仅 email + 密码登录，无注册，无用户管理
- SQL 执行：`IsReadOnlySQL()` 一刀切拦截所有写操作
- schema 发现：扫描整个 public schema，系统表和业务表混在一起
- 数据写入：完全没有
- 前端：单一聊天页面，所有功能集中在 `/`

## Goals / Non-Goals

**Goals:**
- 构建完整的认证体系：手机号登录、admin 创建用户、角色管理
- 实现 SQL 分级权限：按操作类型和角色等级精确控制
- 引入数据写入能力：文件上传导入，AI 预检，建表审批
- 系统表全面保护：schema 发现和 SQL 执行双路拦截
- 补齐数据源管理：编辑、删除、测试连通性
- 知识库支持 .md 文件上传
- 新增表结构浏览页面

**Non-Goals:**
- 不引入 OAuth/SSO 第三方登录
- 不实现行级权限（RLS）
- 不支持数据库类型扩展（仍仅 PostgreSQL）
- 不做多工作区/多租户

## Decisions

### D1: SQL 权限控制 — 分类器 + 角色检查，不引入 SQL parser 依赖

- **方案**：在 `sqlrun` 包中增加 `ClassifySQL()` 函数，基于关键词前缀分类 SQL 操作类型（SELECT/INSERT/UPDATE/DELETE/CREATE/DROP/ALTER/TRUNCATE）。然后通过 `IsAllowedForRole(sqlType, role)` 检查角色权限。
- **理由**：无需引入完整 SQL parser 依赖（如 pg_query）。关键词前缀分类对 MVP 阶段足够可靠。误判风险低（语句总是以关键词开头）。
- **备选**：使用 `pg_query` Go 绑定解析 AST — 更精确但引入 C 依赖，构建复杂。

### D2: 系统表保护 — 显式黑名单，不迁移 schema

- **方案**：在 `sqlrun` 中维护 `systemTables` map，schema 发现和 SQL 执行双端引用同一黑名单。
- **理由**：改动最小。迁移到独立 schema（如 `hub_internal`）固然更干净，但涉及大量 migration 工作，且需要处理现有数据。
- **备选**：迁移到 `hub_internal` schema — 更彻底的隔离，但与当前 MVP 数据不兼容。

### D3: 认证改造 — 新增 phone 字段，保留现有 email 字段

- **方案**：`users` 表新增 `phone` 列（UNIQUE），登录时用 phone 替代 email。`email` 字段保留但不再作为登录凭证。seed 默认账号同步改为手机号。
- **理由**：用户要求以姓名（手机号）登录。保留 email 以备将来通知等用途。
- **备选**：直接替换 email 列 — 更简单但丢失了扩展性。

### D4: AI 预检 — 在 Go 侧直接调用 LLM，不走 Python

- **方案**：利用现有的 `internal/llm` 客户端，在 Go handler 中构造 prompt 调用 LLM 进行文件校验。LLM 返回结构化 JSON 结果。
- **理由**：减少跨服务调用，降低延迟。`internal/llm` 已具备 OpenAI 兼容 API 调用能力。
- **备选**：通过 gRPC 调用 Python ai-worker — 更符合现有架构，但增加链路复杂度和延迟。

### D5: 文件导入 — 小文件同步，大文件异步

- **方案**：设定阈值（如 1000 行或 1MB），小于阈值同步处理直接返回结果，大于阈值通过 NATS 异步处理。
- **理由**：小文件用户期望即时反馈，大文件同步会超时。
- **备选**：全部异步 — 实现简单但小文件体验差。

### D6: 建表审批流 — 前端确认，后端记录

- **方案**：AI 生成建表方案后返回给前端，前端展示完整 DDL + 字段说明，用户点击确认后后端执行 `CREATE TABLE`。审计日志记录操作。
- **理由**：建表不可逆，需要用户阅读确认。执行后不提供回滚（PostgreSQL DDL 不支持事务回滚）。
- **备选**：直接执行 — 体验更快但风险高。

### D7: 数据管理页面合一

- **方案**：不拆分页面。聊天查询和数据管理在同一页面，数据管理区域默认折叠，仅 operator+ 可见。
- **理由**：用户明确要求合并在一个页面。折叠设计让 viewer 看不到文件上传 UI。

## Risks / Trade-offs

| 风险 | 缓解措施 |
|---|---|
| SQL 分类器误判（如 CTE 语句前有 WITH） | 先做 `TrimSpace` + `ToUpper`，仅匹配语句首词。误判率低，发现后补充处理 |
| AI 预检增加文件上传延迟 | 同步路径设置 30s 超时，LLM 调用控制在 10s 内。超时后降级为基本校验（仅列名匹配）|
| 建表不可回退 | 前端二次确认（弹窗），用户必须阅读完整 DDL。审计日志记录操作人和时间 |
| 文件上传解析各类编码 | CSV 强制 UTF-8，非 UTF-8 报错提示。.xlsx 由 openpyxl 处理不受编码影响 |
| 角色升级后越权 | 所有 SQL 执行都经过 `ClassifySQL` + `IsAllowedForRole`，不依赖前端控制 |

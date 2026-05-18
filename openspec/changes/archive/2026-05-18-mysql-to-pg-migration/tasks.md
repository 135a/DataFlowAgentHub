## 1. 依赖清理与基础配置

- [ ] 1.1 移除 `github.com/jackc/pgx/v5` 和 `github.com/jackc/pgx/v5/pgxpool` 依赖，执行 `go mod tidy`
- [ ] 1.2 从 config 包中移除 PG 连接相关配置字段（`HUB_DATABASE_URL` 等）
- [ ] 1.3 移除 config 中 pgxpool 初始化相关逻辑
- [ ] 1.4 确认 `github.com/go-sql-driver/mysql` 在 go.mod 中，且 `database/sql` 正常工作

## 2. MySQL 迁移脚本（替代 PG 迁移）

- [ ] 2.1 编写 MySQL 版 001_init.sql：创建 `hub_platform` Database，创建 `workspaces`、`users`、`sessions`、`data_sources` 表（PG 类型 → MySQL 类型映射）
- [ ] 2.2 编写 MySQL 版 002_*.sql：创建 `messages`、`runs`、`approval_tasks`、`audit_events` 表
- [ ] 2.3 编写 MySQL 版 003_*.sql：创建 `async_tasks`、`knowledge_docs` 表
- [ ] 2.4 编写 MySQL 版 004_*.sql：创建 `agent_run_steps` 表
- [ ] 2.5 更新 migrate 包的 `embed.FS` 加载逻辑，移除旧的 PG SQL 文件
- [ ] 2.6 移除 `internal/migrate/` 中原有的 PG `.sql` 迁移脚本
- [ ] 2.7 验证迁移：启动应用确认所有表自动创建成功

## 3. SQL 执行引擎统一

- [ ] 3.1 将 `sqlrun.QueryRowsMySQL()` 重命名为 `sqlrun.QueryRows()`，删除 pgxpool 版本
- [ ] 3.2 将 `sqlrun.ExecuteWriteMySQL()` 重命名为 `sqlrun.ExecuteWrite()`，删除 pgxpool 版本
- [ ] 3.3 更新所有调用 `QueryRowsMySQL` / `ExecuteWriteMySQL` 的地方为 `QueryRows` / `ExecuteWrite`
- [ ] 3.4 统一 sqlrun 构造函数：移除 pgxpool.Pool 参数，仅接收 MySQL `*sql.DB`
- [ ] 3.5 更新 `sqlrun.IsReadOnlySQL()` 保留 MySQL 关键字校验

## 4. NL2SQL 执行路径统一

- [ ] 4.1 将 `handlers.ExecuteMySQL()` 重命名为 `handlers.Execute()`，删除 PG dialect 版本
- [ ] 4.2 更新所有 handler 中调用 `ExecuteMySQL` 的地方为 `Execute`
- [ ] 4.3 移除 AI worker 侧 PG dialect 参数传递逻辑（强制使用 MySQL dialect）
- [ ] 4.4 更新 gRPC proto 定义中可能的 dialect 字段（如不再需要方言参数）

## 5. 平台元数据读写迁移到 MySQL

- [ ] 5.1 创建 MySQL `hub_platform` 连接池初始化逻辑（替换原有的 PG 连接池）
- [ ] 5.2 更新 workspace/store 中所有 PG 查询为 MySQL 语法
- [ ] 5.3 更新 sessions、messages 等模块的数据访问层
- [ ] 5.4 更新 approval_tasks、audit_events 等近期的数据访问层
- [ ] 5.5 移除 `internal/connector/` 中的 PG 连接检测逻辑
- [ ] 5.6 移除 `internal/schema/` 包中的 PG `information_schema` 查询逻辑

## 6. 数据集/表 CRUD 改造

- [ ] 6.1 更新 handler：创建数据集时，MySQL DDL + 元数据写入使用应用层补偿模式
- [ ] 6.2 更新 handler：删除数据集时，MySQL DDL + 元数据删除
- [ ] 6.3 更新 handler：在数据集下建表时，MySQL DDL + 元数据写入使用补偿模式
- [ ] 6.4 更新 handler：删除数据集表时，MySQL DDL + 元数据删除
- [ ] 6.5 验证所有 CRUD 路径无孤立资源产生

## 7. 清理已删除的 PG 模块

- [ ] 7.1 移除 `internal/otelsetup/` 中仅与 PG 相关的 tracing 初始化代码
- [ ] 7.2 检查 `internal/seed/` 初始化逻辑，确保使用 MySQL 而不是 PG
- [ ] 7.3 搜索所有 import 中的 `pgxpool`、`jackc/pgx` 引用，全部清理
- [ ] 7.4 检查 `internal/config/` 中 PG 相关字段完全移除

## 8. Docker 和部署配置

- [ ] 8.1 从 Docker Compose 中移除 PostgreSQL 容器定义
- [ ] 8.2 更新 `.env` / `.env.example`：移除 PG 环境变量，确认 MySQL 配置完整
- [ ] 8.3 更新 Dockerfile 构建流程（如果有 PG 客户端依赖则移除）

## 9. 编译与集成测试

- [ ] 9.1 执行 `go build ./...` 确认无编译错误
- [ ] 9.2 执行 `go vet ./...` 确认无 vet 警告
- [ ] 9.3 执行 `make test` 确认所有测试通过
- [ ] 9.4 启动 Docker Compose 全栈验证：MySQL + Redis + Chroma + NATS + ai-worker + API 正常连通
- [ ] 9.5 端到端验证：创建数据集 → 建表 → 上传数据 → NL2SQL 查询 → 删除数据集 全流程

## 10. 文档

- [ ] 10.1 更新 `CLAUDE.md` 中移除 PG 相关的架构描述
- [ ] 10.2 更新 README/部署文档中数据库部分为纯 MySQL 架构

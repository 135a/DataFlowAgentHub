# 数据库迁移（MVP）

应用启动时执行 `internal/migrate` 中嵌入的 `*.sql` 文件，**按文件名排序**依次在 Postgres 上执行。

## 升级

1. 在 `internal/migrate/` 增加新的 `00x_*.sql`（例如 `002_add_column.sql`）。
2. 重建并部署 `api` 镜像；进程启动时会自动 `Exec` 整个文件。

## 回滚

当前 MVP **不提供自动 down 迁移**。生产回滚建议：

- 部署上一版本镜像；
- 手动执行反向 SQL（或从备份恢复）。

后续可替换为 goose/atlas 等工具链，与本目录 SQL 文件并存迁移。

### Multi-Agent 架构新增表的回滚说明

我们在 005~007 中引入了以下新表：
- `async_tasks`: 异步任务队列
- `knowledge_docs`: RAG 知识库文档记录
- `agent_run_steps`: Agent 运行轨迹追踪

如果需要回滚这些变更，请手动执行以下 SQL：
```sql
DROP TABLE IF EXISTS agent_run_steps CASCADE;
DROP TABLE IF EXISTS knowledge_docs CASCADE;
DROP TABLE IF EXISTS async_tasks CASCADE;
```

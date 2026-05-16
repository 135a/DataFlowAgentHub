# langgraph-persistent-state 规格说明

## ADDED Requirements

### Requirement: SqliteSaver 持久化检查点

系统 SHALL 使用 `SqliteSaver` 替代 `MemorySaver` 作为 LangGraph 检查点存储，将 Agent 状态持久化到 SQLite 数据库文件。

#### Scenario: 首次启动创建数据库

- **WHEN** ai-worker 容器首次启动且 `/data/langgraph/checkpoints.db` 不存在
- **THEN** SqliteSaver 自动创建 SQLite 数据库文件和表结构

#### Scenario: 重启保留状态

- **WHEN** ai-worker 容器重启
- **THEN** 之前保存的 LangGraph 检查点状态仍可从 SQLite 读取

#### Scenario: 环境变量配置路径

- **WHEN** 设置 `LANGGRAPH_DB_PATH=/custom/path/checkpoints.db`
- **THEN** SqliteSaver 使用指定路径，自动创建父目录

### Requirement: Docker Volume 持久化

系统 SHALL 在 docker-compose.yml 中为 ai-worker 服务挂载 `langgraph_data` 卷到 `/data/langgraph`。

#### Scenario: 容器销毁后数据保留

- **WHEN** `docker compose down` 后 `docker compose up -d`
- **THEN** 之前写入 `/data/langgraph/checkpoints.db` 的检查点数据仍然存在

#### Scenario: 显式删除卷

- **WHEN** 执行 `docker compose down -v`
- **THEN** langgraph_data 卷被删除，所有持久化状态丢失

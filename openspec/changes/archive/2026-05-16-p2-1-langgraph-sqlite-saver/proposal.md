## Why

当前 LangGraph 使用 `MemorySaver` 作为检查点存储，容器重启后所有对话状态丢失。需要切换到 `SqliteSaver` 并将 SQLite 数据库文件挂载到 Docker volume，实现重启不丢状态。

## What Changes

- 修改 `orchestrator/graph.py`：将 `MemorySaver()` 替换为 `SqliteSaver`，从环境变量读取数据库路径
- 修改 `deploy/compose/docker-compose.yml`：为 ai-worker 添加 volume 挂载 `langgraph_data`
- 新增 Python 依赖 `aiosqlite`（SqliteSaver 所需）

## Capabilities

### New Capabilities

- `langgraph-persistent-state`: LangGraph 检查点持久化存储，容器重启后 Agent 状态不丢失

### Modified Capabilities

<!-- 纯基础设施变更，无 spec 需求变更 -->

## Impact

- 修改 `services/ai/orchestrator/graph.py`
- 修改 `deploy/compose/docker-compose.yml`
- 修改 `services/ai/pyproject.toml`（新增 aiosqlite 依赖）
- 新增 Docker volume `langgraph_data`

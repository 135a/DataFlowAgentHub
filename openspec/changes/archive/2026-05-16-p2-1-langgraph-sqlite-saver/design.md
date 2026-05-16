## Context

当前 `graph.py` 使用 `MemorySaver()` 作为 LangGraph 检查点器：
```python
from langgraph.checkpoint.memory import MemorySaver
memory = MemorySaver()
graph = builder.compile(checkpointer=memory)
```

LangGraph 提供 `SqliteSaver` 和 `AsyncSqliteSaver` 作为持久化替代。由于 Consumer 线程使用 `asyncio.to_thread` 调用同步图，同步 `SqliteSaver` 即可。

## Goals / Non-Goals

**Goals:**
- 切换到 SqliteSaver，SQLite 文件路径可通过环境变量 `LANGGRAPH_DB_PATH` 配置
- Docker volume 挂载确保 ai-worker 重启不丢数据
- 默认路径：`/data/langgraph/checkpoints.db`

**Non-Goals:**
- 不迁移历史 MemorySaver 数据（首次切换从空库开始）
- 不改为 AsyncSqliteSaver（consumer 线程调用同步图）

## Decisions

### 1. SqliteSaver 配置

**选择**：通过环境变量 `LANGGRAPH_DB_PATH` 指定 SQLite 路径，默认 `/data/langgraph/checkpoints.db`。

```python
import os
from langgraph.checkpoint.sqlite import SqliteSaver

db_path = os.getenv("LANGGRAPH_DB_PATH", "/data/langgraph/checkpoints.db")
os.makedirs(os.path.dirname(db_path), exist_ok=True)
checkpointer = SqliteSaver.from_conn_string(db_path)
```

### 2. Docker volume 配置

在 docker-compose.yml 中：
- 新增命名卷 `langgraph_data`
- ai-worker 服务挂载 `langgraph_data:/data/langgraph`

## Risks / Trade-offs

- SQLite 并发写入限制：当前为单消费者线程，无并发问题 → 未来多 worker 需切换 PostgreSQL 检查点

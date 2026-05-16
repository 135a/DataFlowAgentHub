## 1. 代码变更

- [x] 1.1 修改 `orchestrator/graph.py`：导入 `SqliteSaver`，从 `LANGGRAPH_DB_PATH` 读取路径，替换 `MemorySaver`
- [x] 1.2 在 `pyproject.toml` 中添加 `langgraph-checkpoint-sqlite` 依赖

## 2. Docker 配置

- [x] 2.1 修改 `docker-compose.yml`：添加 `langgraph_data` 命名卷
- [x] 2.2 在 ai-worker 服务添加卷挂载 `langgraph_data:/data/langgraph`
- [x] 2.3 在 ai-worker 环境变量中添加 `LANGGRAPH_DB_PATH=/data/langgraph/checkpoints.db`

## 3. 验证

- [x] 3.1 `docker compose up -d --build` 启动后，验证 SQLite DB 文件创建
- [x] 3.2 发送异步任务，验证检查点写入
- [x] 3.3 `docker compose down && docker compose up -d` 验证重启后数据保留

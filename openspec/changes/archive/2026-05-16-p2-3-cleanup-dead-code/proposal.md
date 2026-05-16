## Why

项目存在空目录、未使用的导入、死代码和缺失的 `.gitignore` 规则，影响项目整洁度和面试印象。需要系统性清理。

## What Changes

- 清理未使用的 Python 导入（`sign_body` 在 consumer.py、knowledge_consumer.py 中）
- 添加 `__pycache__/` 和 `*.pyc` 到 `.gitignore`
- 删除或标注空目录 `services/ai/gen/nl2sql/v1/`
- 删除 `pkg/.gitkeep` 或保留待将来使用
- 标注（不删除）未使用的 Go 导出：`crypto/hmac.go` 的 `HMACVerify`/`FormatHMACSignature`、`connector/postgres.go` 的 `Postgres`/`Ping`/`ListPublicTables`、`llm/client.go` 的 `Client`/`ChatCompletion`

## Capabilities

### New Capabilities

- `project-cleanup`: 项目目录整洁、无死代码导入、Python 缓存文件被 gitignore 排除

### Modified Capabilities

<!-- 纯清理，无 spec 变更 -->

## Impact

- 修改 `.gitignore`（新增 `__pycache__/`、`*.pyc`）
- 修改 `services/ai/orchestrator/consumer.py`（移除未使用的 sign_body 导入）
- 修改 `services/ai/orchestrator/knowledge_consumer.py`（同上）
- 可选：删除 `services/ai/gen/nl2sql/v1/` 空目录、`pkg/.gitkeep`

## 1. .gitignore 更新

- [x] 1.1 在根 `.gitignore` 中添加 `__pycache__/` 和 `*.pyc` 规则

## 2. Python 导入清理

- [x] 2.1 修改 `services/ai/orchestrator/consumer.py`：移除未使用的 `sign_body` 导入
- [x] 2.2 修改 `services/ai/orchestrator/knowledge_consumer.py`：移除未使用的 `sign_body` 导入

## 3. 空目录处理

- [x] 3.1 在 `services/ai/gen/nl2sql/v1/` 添加 `.gitkeep` 占位文件

## 4. 验证

- [x] 4.1 运行 `git status` 确认 `__pycache__/` 不再出现
- [x] 4.2 确认 Python consumer 文件导入无误（`python -c "from services.ai.orchestrator.consumer import run_consumer"` 语法检查）

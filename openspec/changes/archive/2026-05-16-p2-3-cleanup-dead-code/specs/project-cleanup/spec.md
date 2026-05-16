# project-cleanup 规格说明

## ADDED Requirements

### Requirement: .gitignore 排除 Python 缓存

系统 SHALL 在根 `.gitignore` 中添加 `__pycache__/` 和 `*.pyc` 规则，防止 Python 字节码缓存文件被提交到版本控制。

#### Scenario: __pycache__ 被忽略

- **WHEN** 运行 `git status`
- **THEN** `services/ai/` 下的 `__pycache__/` 目录不会出现在未跟踪文件列表中

### Requirement: 移除未使用的 Python 导入

系统 SHALL 从 `consumer.py` 和 `knowledge_consumer.py` 中移除未使用的 `sign_body` 导入。

#### Scenario: 导入清理

- **WHEN** 检查 `consumer.py` 和 `knowledge_consumer.py` 的 import 语句
- **THEN** 仅导入实际使用的 `make_headers`，不包含未使用的 `sign_body`

### Requirement: 空目录保留占位

系统 SHALL 为 `services/ai/gen/nl2sql/v1/`（protobuf 生成输出目录）保留 `.gitkeep` 占位文件，确保目录结构被版本控制跟踪。

#### Scenario: 生成目录存在

- **WHEN** 克隆仓库后
- **THEN** `services/ai/gen/nl2sql/v1/` 目录存在（包含 `.gitkeep`），`make gen-py` 可正常输出到此目录

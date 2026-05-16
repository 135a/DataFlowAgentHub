## Context

扫描发现以下需要清理的项目：

| 类别 | 位置 | 处理方式 |
|---|---|---|
| 未使用的导入 | `consumer.py:8` `sign_body` | 移除 |
| 未使用的导入 | `knowledge_consumer.py:8` `sign_body` | 移除 |
| 空目录 | `services/ai/gen/nl2sql/v1/` | 添加 `.gitkeep` 占位（protobuf 生成目标目录） |
| 占位符 | `pkg/.gitkeep` | 保留（Go 共享包预留） |
| .gitignore 缺失 | 根 `.gitignore` 无 Python 缓存规则 | 添加 `__pycache__/`、`*.pyc` |

## Goals / Non-Goals

**Goals:**
- 移除未使用的 Python 导入
- .gitignore 添加 Python 缓存排除规则
- 空目录添加说明性 `.gitkeep`

**Non-Goals:**
- 不删除未使用的 Go 导出（保持 API 兼容性，未来可能用到）
- 不运行 `make gen-py` 生成 protobuf 桩代码（不属于清理范围）

## Decisions

### 1. 不删除死代码，只清理导入

Go 端的未使用导出（`HMACVerify`、`Postgres.Ping` 等）保留。它们虽当前未被调用，但属于公共 API，删除可能破坏外部引用或未来计划。

### 2. 空目录用 .gitkeep 占位

`services/ai/gen/nl2sql/v1/` 是 `make gen-py` 的输出目标目录，添加 `.gitkeep` 确保目录被 git 跟踪。

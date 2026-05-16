## Context

`services/ai/orchestrator/` 下的 4 个 Python 文件（`consumer.py`、`knowledge_consumer.py`、`graph.py`、`tracing.py`）各自实现了完全相同的 HMAC-SHA256 签名逻辑：

- `sign_body()` 函数在 `consumer.py`、`knowledge_consumer.py`、`tracing.py` 中重复定义（3 次）
- `make_headers()` 函数在 `consumer.py`、`knowledge_consumer.py` 中重复定义（2 次），`tracing.py` 和 `graph.py` 则以内联方式等价实现
- `hmac` 和 `hashlib` 导入在 4 个文件中各出现一次

这些函数的签名和行为完全一致：读取 `HUB_INTERNAL_HMAC_SECRET` 环境变量，对请求体做 HMAC-SHA256 签名，生成 `X-Hub-Signature` 请求头，用于向 Go API 发起内部回调。Go 端在 `internal/middleware/middleware.go` 中验证该签名头。

## Goals / Non-Goals

**Goals:**
- 将重复的 `sign_body()` 和 `make_headers()` 提取到 `services/ai/hub_ai/shared.py` 单一共享模块
- 4 个 orchestrator 文件均改为从 `hub_ai.shared` 导入，消除重复定义

**Non-Goals:**
- 不改变函数签名或 HMAC 算法
- 不修改 Go 端的签名验证逻辑
- 不引入新的第三方依赖
- 不更改环境变量读取方式或默认值

## Decisions

### 共享模块的位置：`hub_ai/shared.py`

**选择**：放在 `hub_ai/` 包下，而非 `orchestrator/` 或新建独立包。

**理由**：
- `hub_ai/` 是 AI worker 的核心包，已有 `__init__.py`，无需额外配置即可作为 Python 包导入
- 未来其他模块（如 `rag/`、`agents/`）也可能需要 HMAC 签名功能，放在 `hub_ai/` 下比 `orchestrator/` 更便于跨包复用
- 避免引入顶层新包，保持目录结构简洁

### 函数命名和签名保持不变

**选择**：原样保留 `sign_body(secret: str, body: bytes) -> str` 和 `make_headers(secret: str, body_bytes: bytes) -> dict` 的签名。

**理由**：调用方无需任何修改（除 import 语句外），降低改动风险和测试成本。

### `tracing.py` 一并使用 `make_headers()`

**选择**：`tracing.py` 当前只定义了 `sign_body()` 并在调用处内联构造 headers dict，改为统一使用 `make_headers()`。

**理由**：`make_headers()` 的输出与此处内联 dict 完全等价，统一使用后 `tracing.py` 也无需 import `hmac`/`hashlib`，与 `graph.py` 的处理一致。

## Risks / Trade-offs

- **循环导入风险**：`hub_ai/__main__.py` 可能间接依赖 orchestrator。`shared.py` 本身不 import 项目内其他模块（只依赖标准库 `hmac`/`hashlib`），因此不存在循环导入问题。
- **导入路径变更**：若后续 `hub_ai/` 包结构调整，需同步更新 import 路径。影响范围可控——仅 4 个 orchestrator 文件。

## Migration Plan

1. 新建 `services/ai/hub_ai/shared.py`，写入 `sign_body()` 和 `make_headers()`
2. 逐文件修改 4 个 orchestrator 文件：移除本地定义、添加 import
3. 验证方式：重启 ai-worker 容器，确认 NATS 消费者正常启动、回调 Go API 成功（无 401 签名错误）

回滚方式：若出现问题，git revert 即可恢复至原有重复代码状态。无需数据库变更或配置变更。

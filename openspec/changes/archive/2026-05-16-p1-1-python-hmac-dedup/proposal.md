## Why

`services/ai/orchestrator/` 下 4 个文件各自独立实现了相同的 HMAC-SHA256 签名逻辑（`sign_body()` 函数重复定义 3 次、`make_headers()` 重复定义 2 次、等价的 inline 代码出现 4 次），造成维护负担和一致性风险。将其提取到共享模块可消除约 40 行重复代码，降低后续修改签名逻辑时的遗漏风险。

## What Changes

- 新建 `services/ai/hub_ai/shared.py`，集中提供 `sign_body()` 和 `make_headers()` 两个 HMAC 工具函数
- 修改 `services/ai/orchestrator/consumer.py`：移除本地 HMAC 函数定义，改为从 `hub_ai.shared` 导入
- 修改 `services/ai/orchestrator/knowledge_consumer.py`：同上
- 修改 `services/ai/orchestrator/graph.py`：将 inline HMAC 代码替换为 `make_headers()` 调用，移除 `hmac`/`hashlib` 导入
- 修改 `services/ai/orchestrator/tracing.py`：移除本地 `sign_body()` 定义，将 inline headers 构造替换为 `make_headers()` 调用，移除 `hmac`/`hashlib` 导入

## Capabilities

### New Capabilities

- `shared-hmac-utility`: 提供统一的 HMAC-SHA256 请求签名工具函数（`sign_body` 和 `make_headers`），供 `orchestrator/` 下各模块复用

### Modified Capabilities

<!-- 无现有 spec 的需求变更 —— 本次改动纯属实现层面的重构，不改变外部行为 -->

## Impact

- 受影响的文件：`services/ai/hub_ai/shared.py`（新建）、`services/ai/orchestrator/consumer.py`、`services/ai/orchestrator/knowledge_consumer.py`、`services/ai/orchestrator/graph.py`、`services/ai/orchestrator/tracing.py`
- 行为不变：`sign_body()` 和 `make_headers()` 的函数签名与返回值保持完全一致
- 不涉及 API 变更、数据库变更或依赖变更

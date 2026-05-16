## ADDED Requirements

### Requirement: 共享 HMAC-SHA256 签名工具模块

系统 SHALL 在 `services/ai/hub_ai/shared.py` 中提供一个共享模块，包含 `sign_body()` 和 `make_headers()` 两个函数，供 `orchestrator/` 下各模块通过 `from hub_ai.shared import ...` 方式导入使用。

#### Scenario: sign_body 生成正确的 HMAC 签名

- **WHEN** 调用 `sign_body(secret="my-secret", body=b'{"key":"value"}')`
- **THEN** 返回值格式为 `"sha256=<hexdigest>"`，其中 `<hexdigest>` 是对给定 body 和 secret 做 HMAC-SHA256 计算得到的 64 位十六进制摘要

#### Scenario: make_headers 返回包含签名和 Content-Type 的字典

- **WHEN** 调用 `make_headers(secret="my-secret", body_bytes=b'{"key":"value"}')`
- **THEN** 返回的字典包含 `X-Hub-Signature` 键（值由 `sign_body()` 生成）和 `Content-Type` 键（值为 `"application/json"`）

#### Scenario: 不同 secret 产生不同签名

- **WHEN** 使用不同的 secret 值对相同的 body 调用 `sign_body()`
- **THEN** 返回的签名值不同

### Requirement: orchestrator 模块从共享模块导入 HMAC 工具函数

`services/ai/orchestrator/` 下所有需要 HMAC 签名的模块 SHALL 从 `hub_ai.shared` 导入 `sign_body()` 和/或 `make_headers()`，而非在本地重复定义。

#### Scenario: consumer.py 使用共享函数

- **WHEN** `consumer.py` 需要对回调请求体做 HMAC 签名
- **THEN** 其使用的 `sign_body()` 和 `make_headers()` 函数来自 `hub_ai.shared` 导入

#### Scenario: knowledge_consumer.py 使用共享函数

- **WHEN** `knowledge_consumer.py` 需要对回调请求体做 HMAC 签名
- **THEN** 其使用的 `sign_body()` 和 `make_headers()` 函数来自 `hub_ai.shared` 导入

#### Scenario: graph.py 使用共享函数

- **WHEN** `graph.py` 中 `nl2sql_node()` 需要对请求体做 HMAC 签名
- **THEN** 其使用的 headers 构造来自 `hub_ai.shared.make_headers()` 调用

#### Scenario: tracing.py 使用共享函数

- **WHEN** `tracing.py` 中 `report_run_step()` 需要对请求体做 HMAC 签名
- **THEN** 其使用的签名和 headers 构造来自 `hub_ai.shared` 导入的 `sign_body()` 和 `make_headers()`

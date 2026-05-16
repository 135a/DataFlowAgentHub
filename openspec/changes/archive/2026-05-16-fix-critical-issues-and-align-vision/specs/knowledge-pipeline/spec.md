## ADDED Requirements

### Requirement: 知识文档上传与 NATS 发布

系统 SHALL 在用户上传知识文档后，将文档元数据写入数据库，并 MUST 将索引任务发布到 NATS JetStream 供 Python Worker 消费。

#### Scenario: 上传文档触发 NATS 发布

- **WHEN** 用户通过 `POST /v1/workspaces/{id}/knowledge/docs` 上传知识文档
- **THEN** 系统 MUST 在 `knowledge_docs` 表中创建记录（status=pending），并 MUST 向 NATS `hub.tasks.knowledge_index` subject 发布包含 doc_id 和 workspace_id 的消息

#### Scenario: NATS 发布失败时记录错误

- **WHEN** NATS 连接不可用导致发布失败
- **THEN** 系统 MUST 将文档 status 设为 `failed`，记录错误信息到 `error_message` 字段，并返回 HTTP 200（文档已保存但索引待重试）

### Requirement: Python Worker 知识索引

Python AI Worker SHALL 订阅 NATS `hub.tasks.knowledge_index` 消息，执行文档分块和向量化存入 ChromaDB，完成后 MUST 通过 HTTP 回调 Go API 更新文档状态。

#### Scenario: 成功索引文档

- **WHEN** Python Worker 收到知识索引消息，且 ChromaDB 可用
- **THEN** Worker MUST 将文档分块、生成 embedding、存入 ChromaDB（collection=workspace_id），并回调 Go API 将文档 status 更新为 `completed`

#### Scenario: 索引失败回调

- **WHEN** 索引过程中 ChromaDB 不可达或 embedding 生成失败
- **THEN** Worker MUST 回调 Go API 将文档 status 更新为 `failed`，并附带错误信息

### Requirement: NATS 消息确认

Python Worker 的 NATS 消费者 SHALL 在处理完成后对消息进行确认（ack），MUST 在处理失败时发送否定确认（nak）以触发重投。

#### Scenario: 成功处理后 ack

- **WHEN** Worker 成功完成知识索引并回调 Go API
- **THEN** 消费者 MUST 调用 `msg.ack()` 确认消息已处理

#### Scenario: 临时失败后 nak

- **WHEN** Worker 因网络问题无法连接 ChromaDB（临时故障）
- **THEN** 消费者 MUST 调用 `msg.nak()` 使消息重新投递

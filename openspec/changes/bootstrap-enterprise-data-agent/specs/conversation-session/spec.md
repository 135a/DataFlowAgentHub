## ADDED Requirements

### Requirement: 会话与消息持久化

系统 SHALL 持久化会话实体与消息历史，支持多轮追问；每条用户消息 MUST 关联到会话与可选 run id。

#### Scenario: 多轮上下文可检索

- **WHEN** 用户在同一会话连续发送两条相关消息
- **THEN** 第二条消息处理时系统 MUST 能读取按顺序排列的历史消息用于编排上下文

### Requirement: SSE 事件流

系统 SHALL 为会话提供基于 HTTP 的 SSE 事件流，用于输出中间步骤、最终 SQL 摘要与结果元数据。

#### Scenario: SSE 连接建立与心跳

- **WHEN** 客户端订阅某会话的 SSE 端点且鉴权通过
- **THEN** 系统 MUST 建立 `text/event-stream` 响应并保持连接直到客户端关闭或服务器结束事件，且 MUST 发送至少一条可解析的初始事件或注释心跳策略中的一种（实现固定并在 OpenAPI 文档化）

### Requirement: 回放所需的最小结构化记录

系统 MUST 持久化关键 run 事件（例如 SQL 生成、策略门触发、审批结果、查询完成）以支持面试演示级回放。

#### Scenario: 审批后可追溯

- **WHEN** 一次 run 经历审批门并获得批准
- **THEN** 审计与消息时间线中 MUST 能查询到审批前后关键事件顺序

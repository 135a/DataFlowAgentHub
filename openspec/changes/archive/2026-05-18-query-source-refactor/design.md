## Context

当前系统在查询时要求用户选择数据集(dataset)和数据表(table)，session 绑定 `dataset_id + dataset_table_id`。schema 解析只返回单表结构，LLM 生成的 SQL 无法跨表 JOIN。知识库(ChromaDB)和数据集(MySQL)是两条路径，但前端没有显式切换入口。

## Goals / Non-Goals

**Goals:**
- 用户可以在"知识库"和"数据集"两种查询源间切换
- 数据集模式下只需选择数据库(dataset)，无需选表，支持跨表 JOIN
- 知识库查询使用 RAG(ChromaDB) + LLM，无分步进度面板
- 数据集模式继续使用现有的 step-based ProgressPanel
- 后端 `dataset_table_id` 降为可选

**Non-Goals:**
- 不改变 MySQL 连接池管理（已按 dataset 级别）
- 不改变数据库 schema
- 不修改现有的数据集 CRUD 和权限管理

## Decisions

### 1. 查询源选择器设计
- 前端新增 `QuerySourceSelector` 组件（类似 ModeSelector 的两按钮切换）
- 选择"知识库"时，隐藏数据集下拉、ModeSelector、ProgressPanel
- 选择"数据集"时，显示数据集下拉（必选）、ModeSelector、ProgressPanel

### 2. 后端 query_source 参数
- `CreateSession` 接受 `query_source` 字段："knowledge" | "dataset"
- 当 `query_source = "dataset"` 时，`dataset_id` 必填
- `dataset_table_id` 不再需要验证，改为完全可选（仅用于向前兼容）
- `PostMessage` 不再根据 `datasetTableID` 路由，改为根据 `query_source`

### 3. resolveDatasetSchema 改为多表
- 不再接受 `tableID` 参数，改为接受 `datasetID`
- 查询 `WHERE tf.table_id IN (SELECT id FROM dataset_tables WHERE dataset_id = $1 AND status = 'active')`
- 返回 `SchemaResult{Tables: [...]}` 包含所有表结构

### 4. 知识库查询路径（RAG + LLM）
- 新增 `postMessageToKnowledge` handler
- 流程：接收消息 → ChromaDB 检索(top_k=3) → 构建 prompt → LLM 生成回答 → 返回
- 同步执行（无需异步 pipeline），超时 15s
- 返回格式：`{ answer: "...", sources: [{title, content}] }`

### 5. 预计时间
- 数据集模式：复用现有 `QUICK_STEPS`/`DEEP_STEPS` + `ProgressPanel`
- 知识库模式：不展示 ProgressPanel，发送前展示文案"预计等待 2-5 秒"，发送中展示 spinner + "正在检索知识库..."

## Risks / Trade-offs

- **[兼容性]** 现有 session 如果存了 `dataset_table_id`，旧前端仍能工作，新前端不再发送此字段。向后兼容。
- **[知识库查询延迟]** LLM 生成回答可能较慢(2-5s)，同步调用可能阻塞 HTTP 响应。→ 设置 15s 超时，失败时返回 fallback 信息。
- **[知识库空结果]** 如果知识库中没有相关文档，RAG 没有上下文，LLM 回答可能不准确。→ 在返回信息中标注"未找到相关文档，回答可能不准确"。

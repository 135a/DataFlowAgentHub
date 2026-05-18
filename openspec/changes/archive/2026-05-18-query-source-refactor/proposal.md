## Why

当前系统要求用户创建会话时同时选择数据集(dataset)和具体数据表(table)，导致无法执行跨表 JOIN 查询。同时，知识库(ChromaDB)和数据集(MySQL)是两条独立的查询路径，但前端没有让用户显式选择的入口，交互混乱。知识库查询不需要分步骤的进度面板，但目前统一使用了 ProgressPanel。

## What Changes

- **新增查询源选择器**：用户在发送消息前选择 "知识库" 或 "数据集" 模式
- **数据集模式**：只选数据库(dataset)，不再选具体表(table)，支持跨表 JOIN
- **知识库模式**：不显示 ModeSelector(quick/deep) 和 ProgressPanel，只显示 spinner + 文案
- **后端 `dataset_table_id` 降为可选**：CreateSession 不再强依赖 table_id，PostMessage 根据 query_source 路由
- **resolveDatasetSchema**：改为返回数据集下**所有活跃表**的结构，而非单表
- **新增知识库查询路径**：RAG(ChromaDB) + LLM 问答，无分步流水线
- **预计时间**：数据集模式沿用现有 step-based 进度面板；知识库模式只显示简单文案 "预计等待 2-5 秒"
- **BREAKING**: 移除前端数据表(table)下拉选择器，`dataset_table_id` 不再作为必选参数

## Capabilities

### New Capabilities
- `query-source-selector`: 前端查询源选择器组件，支持知识库/数据集二选一切换
- `knowledge-qa`: 后端知识库问答路径（RAG + LLM），支持 ChromaDB 检索+LLM 回答

### Modified Capabilities

- `dataset` 修订 - 数据集查询不再绑定具体数据表，改为绑定数据库级别
- `nl2sql` 修订 - resolveDatasetSchema 返回数据集下所有表结构

## Impact

- **前端 (`web/src/App.tsx`)**: 重构查询区域布局，新增查询源选择器，条件渲染 ModeSelector/ProgressPanel
- **前端 (`web/src/components/`)**: 新增 `QuerySourceSelector` 组件
- **后端 (`internal/handlers/handlers.go`)**: `CreateSession` 接受 `query_source` 参数，`PostMessage` 按源路由
- **后端 (`internal/handlers/handlers.go`)**: `resolveDatasetSchema` 改写为返回多表 schema
- **后端 (`internal/handlers/`)**: 新增 `knowledge.go` 中 RAG Q&A handler
- **后端 (`internal/nl2sqlexec/`)**: MySQL 执行器无需改动（已按 dataset 管理连接池）
- **数据库**: 无 schema 变更

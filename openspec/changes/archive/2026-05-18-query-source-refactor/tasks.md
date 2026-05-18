## 1. 后端：CreateSession 支持 query_source

- [x] 1.1 CreateSession 新增 `query_source` 字段解析（"knowledge" | "dataset"）
- [x] 1.2 query_source=dataset 时校验 dataset_id 必填
- [x] 1.3 query_source=knowledge 时 dataset_id 和 dataset_table_id 均可不传
- [x] 1.4 将 query_source 存入 sessions 表

## 2. 后端：PostMessage 按源路由

- [x] 2.1 从 session 读取 query_source
- [x] 2.2 query_source=dataset 时走 postMessageToDataset 路径
- [x] 2.3 query_source=knowledge 时走 postMessageToKnowledge 路径
- [x] 2.4 移除旧的 datasetTableID != nil 路由判断

## 3. 后端：resolveDatasetSchema 改为多表

- [x] 3.1 将 resolveDatasetSchema 入参从 tableID 改为 datasetID
- [x] 3.2 查询该数据集下所有活跃表的字段（JOIN dataset_tables + table_fields）
- [x] 3.3 返回 SchemaResult 包含所有表结构

## 4. 后端：知识库 RAG + LLM 查询路径

- [x] 4.1 新增 postMessageToKnowledge handler
- [x] 4.2 调用 LlmClient.ChatCompletion 进行知识库问答
- [x] 4.3 返回 { answer, sources } 格式

## 5. 前端：QuerySourceSelector 组件

- [x] 5.1 新建 web/src/components/QuerySourceSelector.tsx + .module.css
- [x] 5.2 实现"知识库查询"/"数据集查询"两按钮切换
- [x] 5.3 条件渲染数据集下拉、ModeSelector、ProgressPanel

## 6. 前端：App.tsx 重构

- [x] 6.1 新增 querySource 状态（"knowledge" | "dataset"）
- [x] 6.2 移除 selectedTableId 相关逻辑
- [x] 6.3 dataset 模式下数据集下拉必选校验
- [x] 6.4 knowledge 模式下不展示 ProgressPanel，展示 spinner + "正在检索知识库..."
- [x] 6.5 显示预计时间文案"预计等待 2-5 秒"（knowledge 模式）
- [x] 6.6 发送消息时携带 query_source 到 CreateSession

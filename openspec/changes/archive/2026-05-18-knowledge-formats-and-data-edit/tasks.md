## 1. 数据库：knowledge_docs doc_type CHECK 约束更新

- [x] 1.1 新建 internal/migrate/008_knowledge_doc_types.sql — 修改 CHECK 约束为 `('text', 'pdf', 'doc', 'docx')`，移除 `markdown` 和 `sql`

## 2. 后端：知识库 multipart 上传端点

- [x] 2.1 knowledge.go 新增 UploadKnowledgeDocFromFile handler
- [x] 2.2 根据文件扩展名自动设置 doc_type
- [x] 2.3 将文件二进制作为 base64 编码加入 NATS 消息 payload
- [x] 2.4 保留原有 JSON 上传端点（内部兼容），前端不再使用
- [x] 2.5 在 Routes() 中注册新上传端点

## 3. 后端：数据上传 SQL 文件解析

- [x] 3.1 data.go 新增 parseSQL 函数 — 按分号分割语句，去除注释
- [x] 3.2 对每条语句执行 validateSQLStatement（禁止 SELECT、DROP、ALTER、TRUNCATE、DELETE）
- [x] 3.3 阻止危险语句（DROP, ALTER, TRUNCATE, DELETE）
- [x] 3.4 逐条执行并通过校验的 INSERT/UPDATE 语句，收集错误的语句编号

## 4. 后端：SQL 终端 API

- [x] 4.1 新增 /v1/data/execute 端点 — ExecuteDataSQL handler 接收 {dataset_id, sql}
- [x] 4.2 SELECT 返回 {columns, rows, total_count, truncated}，INSERT/UPDATE 返回 {rows_affected}
- [x] 4.3 统一 SQL 校验：validateSQLTerminal 关键字白名单（仅 SELECT/INSERT/UPDATE）
- [x] 4.4 UPDATE 强制 WHERE 条件校验，缺少 WHERE 的 UPDATE 拒绝执行
- [x] 4.5 MySQL EXPLAIN 语法验证（explainValidate 函数）
- [x] 4.6 data_admin+ 权限校验（复用现有 sqlrun.IsAllowedForRole）
- [x] 4.7 结果集上限 500 行，超限截断并返回 total_count 和 truncated 标志

## 5. Python：PDF/Word 文本提取

- [x] 5.1 requirements.txt 新增 pypdf 和 python-docx 依赖
- [x] 5.2 knowledge_consumer.py 检测 NATS 消息中的 file_bytes 字段
- [x] 5.3 根据 doc_type 调用对应提取器：PDF → pypdf, doc/docx → python-docx, text → 直接使用
- [x] 5.4 提取的文本传入 KnowledgeBase.add_document() 进行分块和索引

## 6. 前端：知识库上传格式更新

- [x] 6.1 KnowledgePage.tsx 文件选择器 accept 改为 `.txt,.doc,.docx,.pdf`
- [x] 6.2 KnowledgePage.tsx 移除 FileReader readAsText 逻辑，改用 multipart/form-data 提交
- [x] 6.3 调用新端点 POST /v1/workspaces/{ws}/knowledge/docs/upload

## 7. 前端：SQL 终端页面

- [x] 7.1 新建 web/src/pages/SqlTerminalPage.tsx — SQL 文本输入框 + 执行按钮 + 结果表格
- [x] 7.2 调用 POST /v1/data/execute — SELECT 结果渲染为表格，INSERT/UPDATE 显示影响行数
- [x] 7.3 错误展示：SQL 语法错误显示具体原因和位置
- [x] 7.4 数据集选择器（页面内切换不同数据集）
- [x] 7.5 快速浏览模式：从表管理页跳转时自动填入 `SELECT * FROM table_name LIMIT 50`
- [x] 7.6 结果分页：超过 500 行显示 "仅显示前 N 行，共 M 行"
- [x] 7.7 web/src/main.tsx 注册路由 /datasets/:did/sql-terminal
- [x] 7.8 DatasetTablesPage.tsx 中增加"SQL 终端"操作链接（data_admin+ 可见）

## 8. 前端：DataManagementPanel SQL 上传支持

- [x] 8.1 DataManagementPanel.tsx 文件输入已接受 .sql（accept=".csv,.xlsx,.sql"），INSERT/UPDATE 选项兼容 SQL 操作
- [x] 8.2 上传 .sql 文件时后端 uploadToDataset() 路由至 executeSQLFile 解析路径

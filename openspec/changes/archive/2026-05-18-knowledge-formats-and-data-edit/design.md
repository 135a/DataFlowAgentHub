## Context

当前知识库上传仅支持纯文本格式 (.md/.txt)，用户无法上传日常使用的 Word 文档和 PDF。数据上传已支持 CSV/XLSX 的 INSERT 操作，但缺少 UPDATE 编辑功能和 SQL 文件直接执行能力。

### 现有状态

- **知识库上传**：前端读取文件为纯文本 → JSON body 发送 → 后端硬编码 `doc_type: "markdown"` → ChromaDB 索引
- **数据上传**：multipart/form-data → 解析 CSV/XLSX → 校验列名 → 执行 INSERT
- **CHECK 约束**：`doc_type IN ('markdown', 'text', 'sql')`
- **UPDATE 操作**：前端已可选 update 操作，文件输入已接受 `.sql`，但后端无 SQL 文件解析和 UPDATE 语法校验

## Goals / Non-Goals

**Goals:**
- 知识库支持 `.txt` / `.doc` / `.docx` / `.pdf` 格式上传
- 上传的 Word/PDF 文档自动提取文本后分块嵌入 ChromaDB
- 数据管理新增 SQL 文件上传执行模式
- 新增 SQL 终端页面，支持 data_admin+ 对数据集执行 SELECT / INSERT / UPDATE
- SQL 操作强制语法和安全校验，语法错误或危险操作拒绝执行
- 编辑数据权限与上传数据相同（data_admin+）

**Non-Goals:**
- 知识库不保留 Markdown 格式支持
- 不支持图片 OCR
- 不支持 Excel/CSV 作为知识库文档格式
- SQL 终端不做行级锁或事务管理（MVP 级别）
- SQL 终端不支持 DELETE 和 DDL 语句（DROP/ALTER/TRUNCATE）

## Decisions

### 1. 文档解析架构：Python 端处理 PDF/Word 文本提取

**决策**：在 Python AI Worker 中增加 PDF/Word 解析，Go 后端仅透传文件二进制

- Go 后端新增 multipart 知识库上传端点，将原始文件作为 NATS 消息的 `file_bytes` (base64) 字段传递
- Python knowledge_consumer 使用 `pypdf` 和 `python-docx` 提取文本后，调用 `KnowledgeBase.add_document()`
- 理由：Go 生态的 PDF/Word 库不如 Python 成熟，且现有 ChromaDB 索引处理已在 Python 侧

### 2. 知识库上传：新增 multipart 端点（保留原 JSON 端点为内部使用）

**决策**：新增 `POST /v1/workspaces/{ws}/knowledge/docs/upload` 端点

- multipart/form-data：file（二进制）+ title（可选，默认文件名）
- 后端根据扩展名设置 `doc_type`：`.txt` → `text`, `.doc`/`.docx` → `doc`, `.pdf` → `pdf`
- 原 JSON body 端点可保留但前端不再使用
- 数据库 `doc_type` CHECK 约束需要 migration：`ADD 'pdf', 'doc', 'docx'`, 移除 `markdown`, `sql`

### 3. SQL 文件：解析为多条语句逐条执行

**决策**：`.sql` 文件解析为多条 SQL 语句，逐条校验 + 执行

- 使用分号分割语句（基础实现，不依赖完整 SQL 解析器）
- 每条语句通过 `sqlrun.IsReadOnlySQL()` 校验——SQL 文件中的语句不能是只读查询
- 单条失败继续执行下一条（收集错误列表）
- 返回：`{ok, rows_affected: N, errors: [...]}`

### 4. SQL 终端：替换行内编辑，采用文本编辑器 + 结果表格

**决策**：新增 `/datasets/{did}/sql-terminal` 页面，将数据编辑从"行内双击编辑"升级为"SQL 终端"模式

- 页面结构：上半部分是 SQL 文本输入框（多行 textarea），下半部分是结果展示区域
- 输入 SQL 后点击"执行"，根据 SQL 类型展示不同结果：
  - SELECT → 表格展示列名 + 行数据 + 行数统计
  - INSERT/UPDATE → 显示影响行数
- 数据集下拉选择器（可在同一页面切换不同数据集）
- "快速浏览"功能：从表管理页跳转时，自动填入 `SELECT * FROM table_name LIMIT 50`
- 执行结果分页：超过 500 行截断并显示总行数

### 5. 统一 SQL 语法校验

**决策**：后端对 SQL 终端的所有语句执行三层校验

- 第一层：关键字白名单——只允许 SELECT / INSERT / UPDATE 开头，拒绝 DROP/ALTER/TRUNCATE/DELETE
- 第二层：UPDATE 强制 WHERE——不包含 WHERE 条件的 UPDATE 拒绝执行
- 第三层：MySQL `EXPLAIN <statement>` 语法验证（利用 MySQL 的语法检查但不实际执行）
- 语法错误返回 400 + 具体错误信息和错误位置

## Risks / Trade-offs

- **PDF 解析质量**：pypdf 提取的文本可能丢失排版顺序 → 当前是知识库场景，语义 chunk 对此容忍度较高
- **大文件**：PDF/Word 可能很大（>50MB）→ NATS 消息有默认 1MB 限制，大文件需考虑分片或存储后传路径
- **SQL 文件安全**：`.sql` 文件可能包含危险语句 → 已在关键字层阻断 DROP/ALTER/TRUNCATE，且 MySQL 连接池使用受限用户
- **并发编辑**：无行级锁 → MVP 阶段可接受，后续可加乐观锁

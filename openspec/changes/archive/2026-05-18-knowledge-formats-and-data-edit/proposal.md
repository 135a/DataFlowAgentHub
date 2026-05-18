## Why

当前知识库仅支持 `.md`/`.txt` 格式的纯文本上传，无法处理用户日常使用的 Word、PDF 等文档；数据上传模块已支持 INSERT 操作，但缺乏 UPDATE 编辑功能（当前 update 选项已有但缺少 SQL 语法校验），且上传不支持 `.sql` 文件直接执行。需要扩展知识库支持的文档格式、增加数据编辑功能及 SQL 文件上传能力。

## What Changes

- **知识库格式扩展**：将上传格式从 `.md`/`.txt`/`.markdown` 改为 `.txt` / `.doc` / `.docx` / `.pdf`，移除 Markdown 专属支持，增加 PDF 和 Word 文档解析
- **编辑数据功能**：新增独立的数据编辑界面入口，支持查看已有数据并执行 UPDATE 语句（带 SQL 语法校验，语法错误拒绝执行）
- **SQL 文件上传**：数据上传功能新增 `.sql` 文件解析执行，支持批量 DML 操作（INSERT/UPDATE/DELETE）
- **权限**：编辑数据功能与上传数据同权限（data_admin+）

## Capabilities

### New Capabilities
- `data-sql-upload`: 通过 `.sql` 文件上传并执行 SQL 语句到数据集
- `data-editor`: 可视化数据编辑与 UPDATE 执行界面，含 SQL 语法校验

### Modified Capabilities
- `knowledge-base`: 文档类型支持从 markdown/txt 改为 txt/doc/docx/pdf，需增加 PDF 和 Word 解析器
- `data-upload`: 表单支持新增 `.sql` 文件格式处理，后端增加 SQL 文件解析器

## Impact

- **Go 后端**：`internal/handlers/data.go` — 新增 SQL 文件解析、UPDATE SQL 语法校验、`/v1/data/execute-sql` 端点
- **Go 后端**：`internal/handlers/knowledge.go` — 新增 PDF/Word 文档上传端点（multipart），新增 `doc_type` 字段支持
- **Go 后端**：`internal/migrate/` — `knowledge_docs` 表的 `doc_type` CHECK 约束增加 `pdf`, `doc`, `docx`
- **Python 端**：`services/ai/rag/knowledge_base.py` — 增加 PDF/Word 文本提取（pypdf / python-docx）
- **Python 端**：`services/ai/orchestrator/knowledge_consumer.py` — 处理新的 `doc_type`
- **前端**：`web/src/pages/KnowledgePage.tsx` — 文件选择器改为 `.txt,.doc,.docx,.pdf`，增加 PDF/Word 上传流程
- **前端**：`web/src/pages/DataEditorPage.tsx` — 新增数据编辑页面
- **前端**：`web/src/components/DataManagementPanel.tsx` — 操作选项新增 SQL 文件模式
- **依赖新增**：Python — `pypdf`, `python-docx`；Go — `go-speech` 或类似 PDF 库

## 1. 数据库迁移

- [x] 1.1 创建 `internal/migrate/009_knowledge_doc_types_v2.sql`，更新 CHECK 约束包含 `'markdown'`
- [x] 1.2 验证 migration 按字母序正确执行

## 2. 知识库：支持 .md 文件上传

- [x] 2.1 修改 `docTypeFromExt()` 添加 `.md` → `"markdown"` 映射和错误提示更新
- [x] 2.2 修改 `UploadKnowledgeDocFromFile` 的 `docType, err := docTypeFromExt()` 后续逻辑确保 `markdown` 类型写入数据库

## 3. 知识库：文件持久化存储

- [x] 3.1 在 `internal/config/config.go` 新增 `KnowledgeFilesDir` 字段（环境变量 `HUB_KNOWLEDGE_FILES_DIR`，默认 `/data/knowledge-files`）
- [x] 3.2 修改 `UploadKnowledgeDocFromFile`，保存文件到 `{KnowledgeFilesDir}/{workspaceID}/{docID}{ext}`
- [x] 3.3 新增 `GET /v1/knowledge/docs/{docID}/download` 端点，从磁盘读取并返回文件
- [x] 3.4 注册新路由到 `handlers.go` 的 `Routes()` 函数

## 4. 报告生成：新增 PDF/DOCX 格式

- [x] 4.1 在 `services/ai/requirements.txt` 添加 `fpdf2` 和 `python-docx` 依赖
- [x] 4.2 修改 `report_generation_agent.py`：移除 Excel 生成，新增 PDF 生成逻辑（使用 fpdf2）
- [x] 4.3 修改 `report_generation_agent.py`：新增 DOCX 生成逻辑（使用 python-docx）
- [x] 4.4 确保三种格式（.md、.pdf、.docx）统一输出到 `REPORT_OUTPUT_DIR`

## 5. 报告下载：修复路径对齐 + 格式参数

- [x] 5.1 修改 `internal/handlers/reports.go` 的 `DownloadReport`：解析 `format` 查询参数
- [x] 5.2 根据 format 参数拼接对应文件扩展名（.pdf/.md/.docx），设置正确的 Content-Type 和 Content-Disposition
- [x] 5.3 无效 format 时返回 HTTP 400

## 6. Docker Compose 共享卷配置

- [x] 6.1 在 `deploy/compose/docker-compose.yml` 新增 `reportdata` 命名卷
- [x] 6.2 挂载 `reportdata` 到 `api` 服务（`/data/reports`）并设置 `HUB_REPORTS_DIR=/data/reports`
- [x] 6.3 挂载 `reportdata` 到 `ai-worker` 服务（`/data/reports`）并设置 `REPORT_OUTPUT_DIR=/data/reports`
- [x] 6.4 新增 `knowledgefiles` 命名卷，挂载到 `api` 服务（`/data/knowledge-files`）并设置 `HUB_KNOWLEDGE_FILES_DIR=/data/knowledge-files`

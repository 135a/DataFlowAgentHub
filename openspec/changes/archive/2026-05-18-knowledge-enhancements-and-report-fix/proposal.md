## Why

知识库之前移除了 `.md` 格式支持，但用户有 Markdown 文档上传需求；同时知识库上传的文件二进制目前处理后被丢弃，无法追溯原始文件。报告下载功能（Excel）因容器路径断裂不可用，且格式单一，需要修复并扩展为 PDF/MD/DOCX 格式。

## What Changes

- 知识库 doc_type CHECK 约束重新添加 `markdown`，支持 `.md` 文件上传
- 知识库上传文件保存到磁盘持久化存储，支持后续文件下载
- 新增 `GET /v1/knowledge/docs/{docID}/download` 端点下载原始文件
- 修复报告下载功能（共享卷 + 路径对齐）
- 报告生成格式从 Excel 改为 PDF、Markdown、DOCX 三种格式
- `GET /v1/runs/{runID}/report` 增加 `format` 查询参数（pdf/md/docx）
- 新增 Python 报告导出依赖（FPDF / WeasyPrint 等 PDF 生成库）

## Capabilities

### New Capabilities
- `knowledge-file-retention`: 知识库上传原始文件的持久化存储与下载
- `report-export`: 报告多格式导出（PDF/MD/DOCX）

### Modified Capabilities
- `knowledge-base`: doc_type 重新包含 `markdown`，保留原始文件

## Impact

- `internal/migrate/009_knowledge_doc_types_v2.sql` — 新增 migration 重新添加 markdown 到 CHECK 约束
- `internal/handlers/knowledge.go` — UploadKnowledgeDocFromFile 增加文件保存逻辑
- `internal/handlers/reports.go` — DownloadReport 支持 format 参数，支持 PDF/MD/DOCX 读取
- `internal/config/config.go` — 新增知识库文件存储目录配置（可选）
- `services/ai/agents/report_generation_agent.py` — 新增 PDF/DOCX 生成逻辑
- `services/ai/requirements.txt` — 新增 PDF/DOCX 生成依赖
- `deploy/compose/docker-compose.yml` — 新增报告共享卷绑定，新增知识库文件存储卷
- 前端无变化（报告下载已是 API 触发，知识库文件下载是新端点）

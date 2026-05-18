## Context

当前平台存在三个问题：

1. **知识库不支持 `.md` 文件上传**：之前移除 `.md` 格式支持后，`doc_type` CHECK 约束中无 `markdown`，且 `docTypeFromExt()` 不支持 `.md` 扩展名映射。
2. **知识库上传文件不保留**：`UploadKnowledgeDocFromFile` 读取文件二进制后编码为 base64 通过 NATS 发送给 Python worker 进行 Chroma 索引，原始文件二进制被丢弃，无法追溯或重新下载。
3. **报告下载功能不可用且格式单一**：Python ai-worker 将报告写入 `/tmp/reports/`，Go API 从 `HUB_REPORTS_DIR` 读取——两个容器无共享卷，导致 `DownloadReport` 永远返回 404。当前仅支持 Excel 格式，需扩展为 PDF/MD/DOCX。

## Goals / Non-Goals

**Goals:**
- 重新支持 `.md` 文件上传到知识库（`doc_type = 'markdown'`）
- 知识库上传的原始文件持久化存储到磁盘，可通过 API 下载
- 修复报告下载功能（容器间共享卷 + 路径对齐）
- 报告支持 PDF、Markdown、DOCX 三种导出格式（移除 Excel）
- 新增 `GET /v1/knowledge/docs/{docID}/download` 端点
- `GET /v1/runs/{runID}/report` 增加 `format` 查询参数

**Non-Goals:**
- 不修改知识库 Chroma 索引流程（仅保留原始文件，索引逻辑不变）
- 不新增前端页面（下载已是 API 触发）
- 不保留 Excel 报告导出
- 不改造现有 MySQL 数据集存储的原始文件保留

## Decisions

### Decision 1：知识库文件存储架构

知识库上传的原始文件保存到磁盘，目录结构：

```
{HUB_KNOWLEDGE_FILES_DIR}/
  {workspace_id}/
    {doc_id}{ext}
```

- `HUB_KNOWLEDGE_FILES_DIR` 新增环境变量，默认 `/data/knowledge-files`
- 文件在上传时保存到该目录，与 NATS 索引流程并行
- 下载时通过 `doc_id` 查询数据库获取 `doc_type` 和文件名，从磁盘读取返回
- 文件删除跟随知识库文档删除（后续可通过级联或 GC 处理）

**选择原因：** 文件系统存储简单可靠，适合 MVP 阶段。理论上可用对象存储（S3/MinIO），但当前无对象存储基础设施。

### Decision 2：PDF 生成方案

选用 `fpdf2`（FPDF Python 库）作为 PDF 生成引擎：

- **优点：** 纯 Python，无系统级依赖（不需要 LaTeX、wkhtmltopdf、WeasyPrint 等），安装轻量
- **对比 WeasyPrint：** WeasyPrint 需要系统安装 `pango`、`cairo`、`gdk-pixbuf` 等 C 库，Docker 镜像构建复杂
- **对比 ReportLab：** 功能更强大但 API 更底层，`fpdf2` 对 Markdown 转 PDF 场景更友好
- **实现：** 读取已生成的 Markdown 报告文件，解析标题/段落/表格后写入 PDF

### Decision 3：DOCX 生成方案

选用 `python-docx` 库：

- Python 生态标准 DOCX 生成库，纯 Python 实现
- 读取 Markdown 报告内容，写入 DOCX 文件的标题/段落/表格
- 无需系统依赖

### Decision 4：报告格式变更

| 变更 | 说明 |
|------|------|
| 移除 Excel | `report_generation_agent.py` 不再生成 `.xlsx` 文件 |
| Markdown 保留 | 继续生成 `.md`（已有逻辑） |
| 新增 PDF | Python 生成后写入共享卷 |
| 新增 DOCX | Python 生成后写入共享卷 |

报告文件命名规则：`{runID}.md`、`{runID}.pdf`、`{runID}.docx`，统一输出到共享报告目录。

### Decision 5：Docker 共享卷

新增两个 Docker 命名卷：

| 卷名 | 挂载点 | 用途 | 挂载的服务 |
|------|--------|------|-----------|
| `reportdata` | `/data/reports` | 报告文件共享 | api, ai-worker |
| `knowledgefiles` | `/data/knowledge-files` | 知识库原始文件共享 | api, ai-worker |

ai-worker 需要挂载 `knowledgefiles` 卷以在知识索引时访问原始文件（当前不必要，但保留扩展性）。

**选择原因：** 命名卷（named volumes）比 bind mount 更易于 Docker Compose 管理，数据在容器生命周期外持久化。

### Decision 6：API 设计

```
GET /v1/knowledge/docs/{docID}/download
  → 读取知识库原始文件并返回（Content-Disposition: attachment）

GET /v1/runs/{runID}/report?format=pdf
  format: pdf | md | docx（默认 pdf）
  → 返回对应格式的报告文件
```

格式映射：
| format 参数 | 文件扩展名 | Content-Type |
|-------------|-----------|-------------|
| pdf | .pdf | application/pdf |
| md | .md | text/markdown |
| docx | .docx | application/vnd.openxmlformats-officedocument.wordprocessingml.document |

## Risks / Trade-offs

- [Risk] `fpdf2` 对表格渲染支持有限，复杂报告可能出现布局问题 → Mitigation: MD 报告结构保持简单，复杂表格建议使用 Markdown 或 DOCX 格式
- [Risk] `python-docx` 处理中文需要指定字体 → Mitigation: 设置默认中文字体（如 SimSun）
- [Risk] 文件存储占用磁盘空间，无自动清理机制 → Mitigation: MVP 阶段不处理，后续可增加文件生命周期管理
- [Risk] 并发写入共享卷可能产生冲突 → Mitigation: 文件名基于唯一 runID/docID，无并发冲突
- [Trade-off] 使用命名卷而非 bind mount → 方便 Docker 管理但不利于外部直接访问文件；MVP 阶段可接受

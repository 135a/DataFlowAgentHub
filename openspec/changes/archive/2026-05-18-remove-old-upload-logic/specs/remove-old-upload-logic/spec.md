## REMOVED Requirements

### Requirement: Legacy upload via target_table
**Reason**: 系统已迁移至基于数据集（MySQL）的上传流程，旧版 PostgreSQL `target_table` 路径已无调用者。
**Migration**: 使用 `dataset_id` + `table_id` 参数调用 `POST /v1/data/upload`

### Requirement: List public schema tables
**Reason**: `/v1/schema/tables` 返回 PostgreSQL `public` schema 表结构，已被数据集表管理 API 取代。
**Migration**: 使用 `GET /v1/datasets/{did}/tables`

### Requirement: Suggest table via AI
**Reason**: AI 建表推荐功能已禁用（原返回 410），设计文档禁止动态建表。
**Migration**: 通过数据集工作流手动创建表结构

### Requirement: Create table via AI
**Reason**: AI 动态建表功能已禁用（原返回 410），设计文档禁止动态建表。
**Migration**: 通过 `POST /v1/datasets/{did}/tables` 手动创建表

### Requirement: SQL file import to PostgreSQL
**Reason**: SQL 文件导入仅旧路径支持，新数据集流程不支持 `.sql` 文件上传。
**Migration**: 使用 CSV 或 XLSX 文件通过数据集上传流程导入

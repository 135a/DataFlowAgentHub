## ADDED Requirements

### Requirement: 上传 .md 文件

operator 及以上角色 SHALL 可以通过上传 .md 文件添加到知识库。

#### Scenario: 上传 .md 文件
- **WHEN** operator 在知识库页面选择 .md 文件并上传
- **THEN** 系统读取文件内容，作为 markdown 文档加入知识库索引队列

#### Scenario: 非 .md 文件被拒
- **WHEN** 用户上传非 .md 文件（如 .pdf、.docx）
- **THEN** 系统返回"仅支持 .md 文件"

#### Scenario: 保留文本框上传
- **WHEN** 用户选择在知识库页面手动输入内容
- **THEN** 系统按原有逻辑处理文本框内容

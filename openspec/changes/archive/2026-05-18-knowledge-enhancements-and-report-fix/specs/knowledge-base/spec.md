## MODIFIED Requirements

### Requirement: Support Markdown file upload

The system SHALL support uploading `.md` files to the knowledge base with `doc_type = 'markdown'`.

#### Scenario: MD file accepted
- **WHEN** a user uploads a `.md` file via `POST /v1/workspaces/{workspaceID}/knowledge/docs/upload`
- **THEN** the system SHALL accept the file
- **AND** set `doc_type` to `'markdown'`

#### Scenario: Unsupported file type rejected
- **WHEN** a user uploads a file with an unsupported extension
- **THEN** the system SHALL return HTTP 400 with an error message

### Requirement: Database CHECK constraint includes markdown

The `knowledge_docs.doc_type` column SHALL allow `'markdown'` as a valid value.

#### Scenario: Migration adds markdown
- **WHEN** the `009_knowledge_doc_types_v2.sql` migration runs
- **THEN** the CHECK constraint SHALL be updated to include `'markdown'`
- **AND** the constraint SHALL include: `'text'`, `'pdf'`, `'doc'`, `'docx'`, `'markdown'`

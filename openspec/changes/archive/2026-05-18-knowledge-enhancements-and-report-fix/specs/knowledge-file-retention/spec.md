## ADDED Requirements

### Requirement: Persist uploaded knowledge base files to disk

The system SHALL persist uploaded knowledge base document files to a configurable disk directory before sending them to the indexing queue.

#### Scenario: File saved on upload
- **WHEN** a user uploads a file via `POST /v1/workspaces/{workspaceID}/knowledge/docs/upload`
- **THEN** the system SHALL save the file binary to `{HUB_KNOWLEDGE_FILES_DIR}/{workspaceID}/{docID}{ext}`
- **AND** the file SHALL be saved before the NATS indexing task is enqueued

#### Scenario: Directory structure
- **WHEN** a file is saved
- **THEN** the file SHALL be stored in a subdirectory named by workspace ID
- **AND** the filename SHALL be `{docID}{original_extension}` (e.g., `550e8400-e29b-41d4-a716-446655440000.pdf`)

#### Scenario: File metadata in database
- **WHEN** a file is uploaded and saved
- **THEN** the `knowledge_docs` row SHALL include the `file_path` (relative or absolute) or enough info to reconstruct the path

### Requirement: Download original knowledge base files

The system SHALL provide an endpoint to download original uploaded knowledge base files.

#### Scenario: Successful download
- **WHEN** a client sends `GET /v1/knowledge/docs/{docID}/download`
- **AND** the document exists and the file exists on disk
- **THEN** the system SHALL return HTTP 200 with the file binary
- **AND** set `Content-Disposition: attachment; filename="{original_filename}"`

#### Scenario: Document not found
- **WHEN** a client sends `GET /v1/knowledge/docs/{docID}/download`
- **AND** the document does not exist in the database
- **THEN** the system SHALL return HTTP 404

#### Scenario: File missing from disk
- **WHEN** a client sends `GET /v1/knowledge/docs/{docID}/download`
- **AND** the document exists in the database but the file is not on disk
- **THEN** the system SHALL return HTTP 404 with an error message indicating the file is unavailable

### Requirement: Configurable storage directory

The knowledge file storage directory SHALL be configurable via environment variable.

#### Scenario: Environment variable
- **WHEN** the API server starts
- **AND** `HUB_KNOWLEDGE_FILES_DIR` environment variable is set
- **THEN** the system SHALL use that path as the root directory for knowledge file storage

#### Scenario: Default directory
- **WHEN** the API server starts
- **AND** `HUB_KNOWLEDGE_FILES_DIR` is not set
- **THEN** the system SHALL default to `/data/knowledge-files`

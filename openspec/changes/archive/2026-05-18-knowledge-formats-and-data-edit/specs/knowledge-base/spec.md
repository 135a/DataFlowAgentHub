## ADDED Requirements

### Requirement: Knowledge base accepts txt/doc/docx/pdf file uploads
The system SHALL allow users to upload `.txt`, `.doc`, `.docx`, and `.pdf` files to the knowledge base. Markdown (`.md`) SHALL no longer be directly supported.

#### Scenario: Upload a PDF document
- **WHEN** user selects a `.pdf` file and submits
- **THEN** the system stores the file, extracts text content, indexes it into ChromaDB, and the document appears in the knowledge doc list

#### Scenario: Upload a Word document
- **WHEN** user selects a `.doc` or `.docx` file and submits
- **THEN** the system stores the file, extracts text content via python-docx, indexes it into ChromaDB, and the document appears in the knowledge doc list

#### Scenario: Upload a text file
- **WHEN** user selects a `.txt` file and submits
- **THEN** the system stores the file, indexes the raw text into ChromaDB, and the document appears in the knowledge doc list

#### Scenario: Upload an unsupported format
- **WHEN** user selects a `.md` or `.markdown` file
- **THEN** the system shows an error: "不支持的文件格式，仅支持 .txt/.doc/.docx/.pdf"

### Requirement: Binary file multipart upload endpoint
The system SHALL provide a dedicated multipart upload endpoint `POST /v1/workspaces/{workspaceID}/knowledge/docs/upload` for binary file uploads.

#### Scenario: Successful multipart upload
- **WHEN** user sends a POST with multipart/form-data containing a `file` field with a `.pdf` file
- **THEN** the system returns 202 with `{id, task_id, status: "pending"}` and enqueues async indexing

#### Scenario: Upload with optional title
- **WHEN** user sends a POST with multipart/form-data containing both `file` and `title` fields
- **THEN** the system uses the provided title; if omitted, defaults to the file name without extension

### Requirement: Automatic doc_type detection
The system SHALL automatically detect `doc_type` from the uploaded file extension.

#### Scenario: Extension-based detection
- **WHEN** a `.txt` file is uploaded
- **THEN** `doc_type` is set to `text`

#### Scenario: Extension-based detection (Word)
- **WHEN** a `.doc` or `.docx` file is uploaded
- **THEN** `doc_type` is set to `doc`

#### Scenario: Extension-based detection (PDF)
- **WHEN** a `.pdf` file is uploaded
- **THEN** `doc_type` is set to `pdf`

### Requirement: Text extraction for PDF/Word documents
The Python AI worker SHALL extract text from PDF and Word documents before indexing to ChromaDB.

#### Scenario: PDF text extraction
- **WHEN** a PDF document is being indexed
- **THEN** the system uses `pypdf` to extract text content before chunking

#### Scenario: Word text extraction
- **WHEN** a Word document (`.doc`/`.docx`) is being indexed
- **THEN** the system uses `python-docx` to extract text content before chunking

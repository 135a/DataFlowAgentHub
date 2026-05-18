## ADDED Requirements

### Requirement: Generate PDF reports

The report generation agent SHALL generate PDF format reports in addition to Markdown.

#### Scenario: PDF generated alongside MD
- **WHEN** the report generation agent completes
- **AND** there is result data available
- **THEN** the system SHALL generate a `.pdf` file at `{REPORT_OUTPUT_DIR}/{runID}.pdf`

#### Scenario: PDF content structure
- **WHEN** a PDF report is generated
- **THEN** it SHALL include: report title, generation timestamp, user request summary, analysis summary, data extract table
- **AND** it SHALL handle the data table using `fpdf2` table support

### Requirement: Generate DOCX reports

The report generation agent SHALL generate DOCX format reports.

#### Scenario: DOCX generated alongside MD
- **WHEN** the report generation agent completes
- **AND** there is result data available
- **THEN** the system SHALL generate a `.docx` file at `{REPORT_OUTPUT_DIR}/{runID}.docx`

#### Scenario: DOCX content structure
- **WHEN** a DOCX report is generated
- **THEN** it SHALL include: report title, generation timestamp, user request summary, analysis summary, data extract table
- **AND** the data table SHALL be rendered as a native DOCX table

### Requirement: Remove Excel report generation

The system SHALL stop generating Excel (.xlsx) report files.

#### Scenario: Excel no longer generated
- **WHEN** the report generation agent completes
- **THEN** the system SHALL NOT generate a `.xlsx` file
- **AND** the existing `df.to_excel()` call SHALL be removed

### Requirement: Report download with format selection

The system SHALL support downloading reports in PDF, Markdown, or DOCX format via a query parameter.

#### Scenario: Download with format parameter
- **WHEN** a client sends `GET /v1/runs/{runID}/report?format=pdf`
- **AND** the run exists and is completed
- **AND** the corresponding file exists on disk
- **THEN** the system SHALL return HTTP 200 with the report file
- **AND** set the appropriate `Content-Type` header based on format

#### Scenario: Default format
- **WHEN** a client sends `GET /v1/runs/{runID}/report` without a `format` parameter
- **THEN** the system SHALL default to `pdf` format

#### Scenario: Invalid format
- **WHEN** a client sends `GET /v1/runs/{runID}/report?format=xls`
- **THEN** the system SHALL return HTTP 400 with an error message

#### Scenario: Content-Type mapping
- **WHEN** a report is downloaded
- **THEN** the Content-Type SHALL be:
  - `application/pdf` for pdf format
  - `text/markdown` for md format
  - `application/vnd.openxmlformats-officedocument.wordprocessingml.document` for docx format

### Requirement: Shared report volume between containers

The report output directory SHALL be a shared Docker volume accessible by both ai-worker and api containers.

#### Scenario: Shared Docker volume
- **WHEN** the Docker Compose stack starts
- **THEN** a named volume `reportdata` SHALL be created
- **AND** it SHALL be mounted at `/data/reports` in both `ai-worker` and `api` services

#### Scenario: Path alignment
- **WHEN** the stack is running
- **THEN** `HUB_REPORTS_DIR` in api SHALL point to `/data/reports`
- **AND** `REPORT_OUTPUT_DIR` in ai-worker SHALL point to `/data/reports`

## ADDED Requirements

### Requirement: Data upload supports SQL files
The system SHALL accept `.sql` files for data upload, parsing and executing the contained SQL statements.

#### Scenario: Upload SQL file with INSERT statements
- **WHEN** user selects a `.sql` file and submits with operation "insert"
- **THEN** the system parses the SQL file, executes INSERT statements against the target MySQL table, and returns `{ok: true, rows_affected: N}`

#### Scenario: Upload SQL file with syntax error
- **WHEN** user submits a `.sql` file containing syntactically invalid SQL
- **THEN** the system returns `{ok: false, error: "syntax error at line N: ..."}` and does not execute any statements

#### Scenario: SQL file with mixed statements
- **WHEN** a `.sql` file contains multiple statements (separated by `;`)
- **THEN** the system executes them sequentially and returns total `rows_affected` plus a list of `errors` for any failed statements

### Requirement: SQL file statement validation
The system SHALL validate SQL statements from `.sql` files before execution.

#### Scenario: Block read-only statements in SQL file
- **WHEN** a `.sql` file contains a SELECT statement
- **THEN** the system rejects it with error "不支持 SELECT 查询语句"

#### Scenario: Block dangerous statements in SQL file
- **WHEN** a `.sql` file contains DROP, ALTER, TRUNCATE, or DELETE statements
- **THEN** the system rejects those statements with a per-statement error

## ADDED Requirements

### Requirement: SQL terminal page
The system SHALL provide a SQL terminal page at `/datasets/{did}/sql-terminal` where data_admin+ users can write and execute arbitrary SQL statements against the dataset MySQL database.

#### Scenario: Execute SELECT query
- **WHEN** a user types a SELECT statement and clicks "执行"
- **THEN** the system executes the query, displays results in a table, and shows row count

#### Scenario: Execute INSERT statement
- **WHEN** a user types an INSERT statement and clicks "执行"
- **THEN** the system executes it and returns `{ok: true, rows_affected: N}`

#### Scenario: Execute UPDATE statement
- **WHEN** a user types an UPDATE statement and clicks "执行"
- **THEN** the system validates the WHERE clause is present, executes it, and returns `{ok: true, rows_affected: N}`

#### Scenario: Syntax error in SQL
- **WHEN** a user submits a malformed SQL statement (e.g., "SELEC * FROM t")
- **THEN** the system returns 400 with specific syntax error details and does NOT execute

#### Scenario: Block dangerous statements
- **WHEN** a user submits DROP, ALTER, TRUNCATE, or DELETE statements
- **THEN** the system returns 400 and does NOT execute

### Requirement: SQL validator for INSERT/UPDATE
The system SHALL validate INSERT and UPDATE statements for safety before execution.

#### Scenario: UPDATE without WHERE clause rejected
- **WHEN** a user submits an UPDATE statement without a WHERE clause
- **THEN** the system rejects with "UPDATE 语句缺少 WHERE 条件，已拒绝执行"

### Requirement: SELECT result pagination
The system SHALL paginate SELECT query results to prevent memory overflow.

#### Scenario: Large result set
- **WHEN** a SELECT query returns more than 500 rows
- **THEN** the system shows the first 500 rows and informs the user about the total count

### Requirement: SQL terminal permission
The system SHALL require data_admin+ permission to access the SQL terminal.

#### Scenario: Unauthorized access
- **WHEN** a normal_user navigates to the SQL terminal page URL
- **THEN** the system returns 403 Forbidden or redirects to an error page

### Requirement: Execute SQL via API
The system SHALL provide a `POST /v1/data/execute` endpoint for executing validated SQL statements on the dataset MySQL.

#### Scenario: Execute SELECT via API
- **WHEN** a POST request is sent to `/v1/data/execute` with `{dataset_id, sql: "SELECT * FROM t LIMIT 10"}`
- **THEN** the system validates the SQL, executes it, and returns `{ok: true, columns: [...], rows: [[...], ...], total_count: N}`

#### Scenario: Execute INSERT via API
- **WHEN** a POST request is sent to `/v1/data/execute` with `{dataset_id, sql: "INSERT INTO t VALUES (1)"}`
- **THEN** the system validates and executes, returns `{ok: true, rows_affected: 1}`

#### Scenario: Execute UPDATE via API
- **WHEN** a POST request is sent to `/v1/data/execute` with `{dataset_id, sql: "UPDATE t SET col='val' WHERE id=1"}`
- **THEN** the system validates (WHERE required), executes, returns `{ok: true, rows_affected: 1}`

### Requirement: Quick table data browser
The system SHALL provide a quick data browse feature for each table as a convenience.

#### Scenario: Browse table data
- **WHEN** a user clicks "浏览数据" on a table in the table management page
- **THEN** the system shows a pre-generated `SELECT * FROM table_name LIMIT 50` in the SQL terminal

#### Scenario: Pagination in browse mode
- **WHEN** a table has more than 50 rows
- **THEN** the system provides "加载更多" button that adds `OFFSET 50` to the query

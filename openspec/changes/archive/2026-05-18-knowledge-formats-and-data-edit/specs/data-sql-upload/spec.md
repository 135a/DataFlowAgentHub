## ADDED Requirements

### Requirement: SQL file parser in data upload
The system SHALL parse `.sql` files into individual SQL statements for execution.

#### Scenario: Multi-statement SQL file
- **WHEN** a `.sql` file contains "INSERT INTO t VALUES (1); INSERT INTO t VALUES (2);"
- **THEN** the system splits into two statements and executes each separately

#### Scenario: SQL file with comments
- **WHEN** a `.sql` file contains `--` line comments or `/* */` block comments
- **THEN** the system strips comments before parsing statements

### Requirement: SQL upload permission check
The system SHALL enforce the same permission level for SQL uploads as for CSV/XLSX uploads.

#### Scenario: Unauthorized user attempts SQL upload
- **WHEN** a user without data_admin+ permission submits a `.sql` file
- **THEN** the system returns 403 Forbidden

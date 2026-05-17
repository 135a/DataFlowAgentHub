## ADDED Requirements

### Requirement: Mode selection buttons

The system SHALL provide two visually distinct buttons for selecting query mode, placed next to the message input.

#### Scenario: Default mode is deep analysis
- **WHEN** user first opens the chat page and has no saved preference
- **THEN** the "深度分析" button SHALL be selected by default

#### Scenario: Selecting a mode changes button appearance
- **WHEN** user clicks the "快速查询" button
- **THEN** that button SHALL fill with blue color (`#2563EB`), and the other button SHALL show ghost/outlined style

#### Scenario: Selecting a mode changes button appearance (deep)
- **WHEN** user clicks the "深度分析" button
- **THEN** that button SHALL fill with purple color (`#7C3AED`), and the other button SHALL show ghost/outlined style

#### Scenario: Mode preference is persisted
- **WHEN** user selects a mode and refreshes the page
- **THEN** the previously selected mode SHALL remain selected

### Requirement: Mode description panel

The system SHALL display a brief description of the currently selected mode below the input area.

#### Scenario: Deep analysis mode description
- **WHEN** "深度分析" mode is selected
- **THEN** a description panel SHALL show: "AI will generate SQL, analyze data, create charts, and produce a report. Estimated wait time: 5-15 seconds"

#### Scenario: Quick query mode description
- **WHEN** "快速查询" mode is selected
- **THEN** a description panel SHALL show: "AI will generate SQL and return results directly. Estimated wait time: 1-3 seconds"

### Requirement: Progress panel during processing

The system SHALL display a step-by-step progress panel when a query is being processed.

#### Scenario: Quick query shows two steps
- **WHEN** user sends a message in "快速查询" mode
- **THEN** the progress panel SHALL show 2 steps: "SQL Generation" and "Query Execution"

#### Scenario: Deep analysis shows four steps
- **WHEN** user sends a message in "深度分析" mode
- **THEN** the progress panel SHALL show 4 steps: "SQL Generation", "Data Analysis", "Chart Creation", "Report Generation"

#### Scenario: Steps show status transitions
- **WHEN** each step progresses through its lifecycle
- **THEN** the step SHALL display one of three states: completed (✅), in-progress (🔄), or waiting (⏳)

#### Scenario: Progress bars reflect completion
- **WHEN** a step completes
- **THEN** its progress bar SHALL fill to 100%, and the next step SHALL transition to "in-progress"

### Requirement: Time estimation

The system SHALL display elapsed time and estimated remaining time during processing.

#### Scenario: First query shows hardcoded estimate
- **WHEN** user sends their first query ever
- **THEN** the estimated time SHALL use hardcoded default values per step

#### Scenario: Subsequent queries use historical averages
- **WHEN** user sends a query after at least one previous query
- **THEN** the estimated time SHALL use the average actual duration of previous steps stored in localStorage

#### Scenario: Elapsed time updates in real-time
- **WHEN** a step is in progress
- **THEN** the elapsed time display SHALL update at least once per second

### Requirement: SSE-driven progress updates

The system SHALL use existing SSE events to drive progress panel state transitions.

#### Scenario: Quick query progress from SSE
- **WHEN** SSE event `sql_generated` is received
- **THEN** "SQL Generation" step SHALL transition to completed
- **WHEN** SSE event `result` is received
- **THEN** "Query Execution" step SHALL transition to completed and the panel SHALL show completion

#### Scenario: Deep analysis progress from SSE
- **WHEN** SSE event `agent_step` is received with `agent_name: "nl2sql_node"`
- **THEN** "SQL Generation" step SHALL transition to in-progress
- **WHEN** SSE event `agent_step` is received with `agent_name: "analysis_node"`
- **THEN** "SQL Generation" SHALL complete and "Data Analysis" SHALL start

### Requirement: Step duration recording

The system SHALL record actual step durations to localStorage for future estimation.

#### Scenario: Durations saved after each query
- **WHEN** a query completes
- **THEN** the actual duration of each step SHALL be saved to localStorage under key `step_history`

#### Scenario: History structure
- **WHEN** durations are saved
- **THEN** they SHALL be stored as an array of objects, each containing step names and their durations in seconds

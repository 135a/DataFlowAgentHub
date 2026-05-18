## ADDED Requirements

### Requirement: User can select query source
The system SHALL allow users to choose between "知识库查询" and "数据集查询" as their query source before sending messages.

#### Scenario: Switch to dataset mode
- **WHEN** user clicks "数据集查询" button
- **THEN** dataset selector dropdown appears
- **THEN** ModeSelector (quick/deep) appears
- **THEN** ProgressPanel becomes visible after sending

#### Scenario: Switch to knowledge base mode
- **WHEN** user clicks "知识库查询" button
- **THEN** dataset selector dropdown is hidden
- **THEN** ModeSelector (quick/deep) is hidden
- **THEN** ProgressPanel is hidden
- **THEN** a text "预计等待 2-5 秒" is displayed

### Requirement: Dataset mode requires database selection
In dataset mode, the user MUST select a database (dataset) before sending a message.

#### Scenario: Send without dataset selected
- **WHEN** user clicks "发送" in dataset mode without selecting a dataset
- **THEN** system shows error "请选择数据库"
- **THEN** message is not sent

USE hub_platform;

CREATE TABLE IF NOT EXISTS agent_run_steps (
    id VARCHAR(36) PRIMARY KEY,
    run_id VARCHAR(36) NOT NULL,
    step_index INT NOT NULL DEFAULT 0,
    agent_name VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'running',
    input_summary TEXT,
    output_summary TEXT,
    error_message TEXT,
    duration_ms INT DEFAULT NULL,
    trace_id VARCHAR(255) DEFAULT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME DEFAULT NULL,
    INDEX idx_agent_run_steps_run_id (run_id, step_index)
);

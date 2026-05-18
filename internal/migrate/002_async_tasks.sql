USE hub_platform;

CREATE TABLE IF NOT EXISTS async_tasks (
    id VARCHAR(36) PRIMARY KEY,
    workspace_id VARCHAR(36) NOT NULL,
    session_id VARCHAR(36) DEFAULT NULL,
    run_id VARCHAR(36) DEFAULT NULL,
    task_type VARCHAR(255) NOT NULL DEFAULT 'agent_pipeline',
    status VARCHAR(50) NOT NULL DEFAULT 'queued',
    payload JSON DEFAULT NULL,
    result JSON DEFAULT NULL,
    error_message TEXT,
    nats_seq BIGINT DEFAULT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    expires_at DATETIME NOT NULL,
    INDEX idx_async_tasks_workspace_status (workspace_id, status),
    INDEX idx_async_tasks_expires_at (expires_at)
);

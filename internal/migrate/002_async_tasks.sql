-- migrate: up
-- 异步任务队列表
CREATE TABLE IF NOT EXISTS async_tasks (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id  UUID        NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    session_id    UUID        REFERENCES sessions(id) ON DELETE SET NULL,
    run_id        UUID        REFERENCES runs(id) ON DELETE SET NULL,
    task_type     TEXT        NOT NULL DEFAULT 'agent_pipeline',
    status        TEXT        NOT NULL DEFAULT 'queued'
                              CHECK (status IN ('queued','running','succeeded','failed','expired')),
    payload       JSONB       NOT NULL DEFAULT '{}',
    result        JSONB,
    error_message TEXT,
    nats_seq      BIGINT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at    TIMESTAMPTZ NOT NULL DEFAULT now() + INTERVAL '2 hours'
);

CREATE INDEX IF NOT EXISTS idx_async_tasks_workspace_status
    ON async_tasks (workspace_id, status);
CREATE INDEX IF NOT EXISTS idx_async_tasks_expires_at
    ON async_tasks (expires_at) WHERE status IN ('queued', 'running');

-- migrate: down
DROP TABLE IF EXISTS async_tasks;

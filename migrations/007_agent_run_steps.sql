-- migrate: up
-- Agent 运行中间步骤追踪表（记录 LangGraph 各节点的执行快照）
CREATE TABLE IF NOT EXISTS agent_run_steps (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id          UUID        NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    step_index      INT         NOT NULL DEFAULT 0,
    agent_name      TEXT        NOT NULL,
    status          TEXT        NOT NULL DEFAULT 'running'
                                CHECK (status IN ('running', 'succeeded', 'failed', 'skipped')),
    input_summary   TEXT,
    output_summary  TEXT,
    error_message   TEXT,
    duration_ms     INT,
    trace_id        TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at    TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_agent_run_steps_run_id
    ON agent_run_steps (run_id, step_index);

-- migrate: down
DROP TABLE IF EXISTS agent_run_steps;

-- migrate: up
-- 知识库文档元数据表（向量存储在 Chroma，此表记录文档来源与状态）
CREATE TABLE IF NOT EXISTS knowledge_docs (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id   UUID        NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    title          TEXT        NOT NULL,
    doc_type       TEXT        NOT NULL DEFAULT 'markdown'
                               CHECK (doc_type IN ('markdown', 'text', 'sql')),
    content_hash   TEXT        NOT NULL,
    chroma_doc_id  TEXT,
    chunk_count    INT         NOT NULL DEFAULT 0,
    status         TEXT        NOT NULL DEFAULT 'pending'
                               CHECK (status IN ('pending', 'indexed', 'failed')),
    created_by     UUID        REFERENCES users(id) ON DELETE SET NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_knowledge_docs_workspace
    ON knowledge_docs (workspace_id, status);

-- migrate: down
DROP TABLE IF EXISTS knowledge_docs;

-- migrate: up
-- 将 knowledge_docs.doc_type 的 CHECK 约束重新添加 markdown 支持
ALTER TABLE knowledge_docs
    DROP CONSTRAINT IF EXISTS knowledge_docs_doc_type_check;

ALTER TABLE knowledge_docs
    ADD CONSTRAINT knowledge_docs_doc_type_check
    CHECK (doc_type IN ('text', 'pdf', 'doc', 'docx', 'markdown'));

-- migrate: down
ALTER TABLE knowledge_docs
    DROP CONSTRAINT IF EXISTS knowledge_docs_doc_type_check;

ALTER TABLE knowledge_docs
    ADD CONSTRAINT knowledge_docs_doc_type_check
    CHECK (doc_type IN ('text', 'pdf', 'doc', 'docx'));

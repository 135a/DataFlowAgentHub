-- migrate: up
-- 将 knowledge_docs.doc_type 的 CHECK 约束从 markdown/text/sql 改为 text/pdf/doc/docx
ALTER TABLE knowledge_docs
    DROP CONSTRAINT IF EXISTS knowledge_docs_doc_type_check;

ALTER TABLE knowledge_docs
    ADD CONSTRAINT knowledge_docs_doc_type_check
    CHECK (doc_type IN ('text', 'pdf', 'doc', 'docx'));

-- migrate: down
ALTER TABLE knowledge_docs
    DROP CONSTRAINT IF EXISTS knowledge_docs_doc_type_check;

ALTER TABLE knowledge_docs
    ADD CONSTRAINT knowledge_docs_doc_type_check
    CHECK (doc_type IN ('markdown', 'text', 'sql'));

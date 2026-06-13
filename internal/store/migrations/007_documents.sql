ALTER TABLE documents ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '';
ALTER TABLE documents ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active';
ALTER TABLE documents ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

CREATE TABLE IF NOT EXISTS document_links (
    id BIGSERIAL PRIMARY KEY,
    household_id BIGINT NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    document_id BIGINT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    entity_type TEXT NOT NULL CHECK (entity_type IN ('project', 'task')),
    entity_id BIGINT NOT NULL,
    created_by BIGINT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (household_id, document_id, entity_type, entity_id)
);

CREATE INDEX IF NOT EXISTS idx_documents_household_status ON documents(household_id, status);
CREATE INDEX IF NOT EXISTS idx_document_links_document ON document_links(document_id);
CREATE INDEX IF NOT EXISTS idx_document_links_entity ON document_links(household_id, entity_type, entity_id);

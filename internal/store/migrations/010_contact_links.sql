CREATE TABLE IF NOT EXISTS contact_links (
    id BIGSERIAL PRIMARY KEY,
    household_id BIGINT NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    contact_id BIGINT NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    entity_type TEXT NOT NULL CHECK (entity_type IN ('project', 'task')),
    entity_id BIGINT NOT NULL,
    created_by BIGINT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (household_id, contact_id, entity_type, entity_id)
);

CREATE INDEX IF NOT EXISTS idx_contact_links_contact ON contact_links(contact_id);
CREATE INDEX IF NOT EXISTS idx_contact_links_entity ON contact_links(household_id, entity_type, entity_id);

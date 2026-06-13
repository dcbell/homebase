ALTER TABLE assets ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active';
ALTER TABLE assets ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

DO $$
DECLARE
    constraint_name TEXT;
BEGIN
    SELECT conname INTO constraint_name
    FROM pg_constraint
    WHERE conrelid = 'document_links'::regclass
        AND contype = 'c'
        AND pg_get_constraintdef(oid) LIKE '%entity_type%';

    IF constraint_name IS NOT NULL THEN
        EXECUTE format('ALTER TABLE document_links DROP CONSTRAINT %I', constraint_name);
    END IF;
END $$;

ALTER TABLE document_links
    ADD CONSTRAINT document_links_entity_type_check CHECK (entity_type IN ('project', 'task', 'asset'));

DO $$
DECLARE
    constraint_name TEXT;
BEGIN
    SELECT conname INTO constraint_name
    FROM pg_constraint
    WHERE conrelid = 'contact_links'::regclass
        AND contype = 'c'
        AND pg_get_constraintdef(oid) LIKE '%entity_type%';

    IF constraint_name IS NOT NULL THEN
        EXECUTE format('ALTER TABLE contact_links DROP CONSTRAINT %I', constraint_name);
    END IF;
END $$;

ALTER TABLE contact_links
    ADD CONSTRAINT contact_links_entity_type_check CHECK (entity_type IN ('project', 'task', 'asset'));

CREATE TABLE IF NOT EXISTS asset_links (
    id BIGSERIAL PRIMARY KEY,
    household_id BIGINT NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    asset_id BIGINT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    entity_type TEXT NOT NULL CHECK (entity_type IN ('project', 'task')),
    entity_id BIGINT NOT NULL,
    created_by BIGINT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (household_id, asset_id, entity_type, entity_id)
);

CREATE INDEX IF NOT EXISTS idx_assets_household_status ON assets(household_id, status, name);
CREATE INDEX IF NOT EXISTS idx_asset_links_asset ON asset_links(asset_id);
CREATE INDEX IF NOT EXISTS idx_asset_links_entity ON asset_links(household_id, entity_type, entity_id);

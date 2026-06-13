CREATE TABLE IF NOT EXISTS asset_maintenance_items (
    id BIGSERIAL PRIMARY KEY,
    household_id BIGINT NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    asset_id BIGINT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    notes TEXT NOT NULL DEFAULT '',
    cadence TEXT NOT NULL CHECK (cadence IN ('daily', 'weekly', 'monthly', 'quarterly', 'yearly')),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    next_due_at TIMESTAMPTZ,
    last_completed_at TIMESTAMPTZ,
    created_by BIGINT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE tasks ADD COLUMN IF NOT EXISTS asset_maintenance_item_id BIGINT REFERENCES asset_maintenance_items(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_asset_maintenance_items_asset ON asset_maintenance_items(asset_id, status, next_due_at);
CREATE INDEX IF NOT EXISTS idx_asset_maintenance_items_due ON asset_maintenance_items(household_id, next_due_at)
    WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_tasks_asset_maintenance_item ON tasks(asset_maintenance_item_id);

ALTER TABLE assets ADD COLUMN IF NOT EXISTS purchase_date DATE;
ALTER TABLE assets ADD COLUMN IF NOT EXISTS purchase_cost NUMERIC(12,2);
ALTER TABLE assets ADD COLUMN IF NOT EXISTS vendor TEXT NOT NULL DEFAULT '';
ALTER TABLE assets ADD COLUMN IF NOT EXISTS model TEXT NOT NULL DEFAULT '';
ALTER TABLE assets ADD COLUMN IF NOT EXISTS maintenance_cadence TEXT NOT NULL DEFAULT '';
ALTER TABLE assets ADD COLUMN IF NOT EXISTS maintenance_next_due_at TIMESTAMPTZ;
ALTER TABLE assets ADD COLUMN IF NOT EXISTS maintenance_last_completed_at TIMESTAMPTZ;

ALTER TABLE tasks ADD COLUMN IF NOT EXISTS asset_id BIGINT REFERENCES assets(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_tasks_asset ON tasks(asset_id);
CREATE INDEX IF NOT EXISTS idx_assets_maintenance_due ON assets(household_id, maintenance_next_due_at)
    WHERE status = 'active' AND maintenance_cadence <> '';

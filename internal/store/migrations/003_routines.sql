ALTER TABLE tasks ADD COLUMN IF NOT EXISTS routine_id BIGINT REFERENCES routines(id) ON DELETE SET NULL;

ALTER TABLE routines ADD COLUMN IF NOT EXISTS notes TEXT NOT NULL DEFAULT '';
ALTER TABLE routines ADD COLUMN IF NOT EXISTS assigned_to BIGINT REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE routines ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived'));
ALTER TABLE routines ADD COLUMN IF NOT EXISTS last_completed_at TIMESTAMPTZ;
ALTER TABLE routines ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

CREATE INDEX IF NOT EXISTS idx_tasks_routine ON tasks(routine_id);
CREATE INDEX IF NOT EXISTS idx_routines_household_due ON routines(household_id, next_due_at);

-- Appointments are archived, never deleted (the Archive rule).
ALTER TABLE events ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active';
ALTER TABLE events DROP CONSTRAINT IF EXISTS events_status_check;
ALTER TABLE events ADD CONSTRAINT events_status_check CHECK (status IN ('active', 'archived'));

-- Scaffolding for pulling Appointments from an external calendar provider,
-- never used: Homebase is a CalDAV server instead (ADR-0004).
ALTER TABLE events DROP COLUMN IF EXISTS source;
ALTER TABLE events DROP COLUMN IF EXISTS external_id;
ALTER TABLE events DROP COLUMN IF EXISTS sync_status;

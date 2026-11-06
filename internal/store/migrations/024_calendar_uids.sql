-- The iCalendar UID a device chose for an item it created over CalDAV
-- (ADR-0004). Items made in Homebase have none and use a generated UID.
-- Archived items free their UID so a device can reuse it.
ALTER TABLE events ADD COLUMN IF NOT EXISTS calendar_uid TEXT;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS calendar_uid TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_events_household_calendar_uid
    ON events(household_id, calendar_uid)
    WHERE calendar_uid IS NOT NULL AND status <> 'archived';
CREATE UNIQUE INDEX IF NOT EXISTS idx_tasks_household_calendar_uid
    ON tasks(household_id, calendar_uid)
    WHERE calendar_uid IS NOT NULL AND status <> 'archived';

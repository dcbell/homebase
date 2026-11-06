-- A Routine's or Maintenance Schedule's Reminder Times use the form that
-- matches its next due date, as a Task's do, but 030 filled every existing
-- one with the date-only defaults, which don't fit a timed next due date. As
-- for Tasks (027), NULL now means the built-in defaults for the kind of next
-- due date, worked out when read as the kind depends on APP_TIMEZONE, so the
-- lists 030 filled in become NULL. Its Generated Tasks copy the NULL and so
-- get the defaults for their due date, as before Reminder Times. This runs
-- only while the column is still NOT NULL, so lists set later are kept.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = current_schema() AND table_name = 'routines'
            AND column_name = 'reminder_times' AND is_nullable = 'NO'
    ) THEN
        ALTER TABLE routines ALTER COLUMN reminder_times DROP NOT NULL;
        UPDATE routines SET reminder_times = NULL
        WHERE reminder_times = '[{"days": -1, "at": "08:00"}, {"days": 0, "at": "08:00"}, {"days": 1, "at": "08:00"}]'::jsonb;
    END IF;

    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = current_schema() AND table_name = 'asset_maintenance_items'
            AND column_name = 'reminder_times' AND is_nullable = 'NO'
    ) THEN
        ALTER TABLE asset_maintenance_items ALTER COLUMN reminder_times DROP NOT NULL;
        UPDATE asset_maintenance_items SET reminder_times = NULL
        WHERE reminder_times = '[{"days": -1, "at": "08:00"}, {"days": 0, "at": "08:00"}, {"days": 1, "at": "08:00"}]'::jsonb;
    END IF;
END $$;

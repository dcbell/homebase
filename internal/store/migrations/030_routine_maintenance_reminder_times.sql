-- Routines and Maintenance Schedules carry the Reminder Times copied onto each
-- Task they generate, always in the date-only form ({"days": -1, "at":
-- "08:00"}) to match their date-only due dates. Existing ones get the
-- built-in date-only Task defaults; new ones are always given a list by the
-- API, so the default is dropped after the fill.
ALTER TABLE routines
    ADD COLUMN IF NOT EXISTS reminder_times JSONB NOT NULL
        DEFAULT '[{"days": -1, "at": "08:00"}, {"days": 0, "at": "08:00"}, {"days": 1, "at": "08:00"}]';

ALTER TABLE routines ALTER COLUMN reminder_times DROP DEFAULT;

ALTER TABLE asset_maintenance_items
    ADD COLUMN IF NOT EXISTS reminder_times JSONB NOT NULL
        DEFAULT '[{"days": -1, "at": "08:00"}, {"days": 0, "at": "08:00"}, {"days": 1, "at": "08:00"}]';

ALTER TABLE asset_maintenance_items ALTER COLUMN reminder_times DROP DEFAULT;

-- Each Appointment carries its own Reminder Times, as a JSON list of timed
-- Reminder Times ({"minutes": -60}). Existing Appointments get the built-in
-- defaults (24 hours before, an hour before, at the start); new ones are
-- always given a list by the API, so the default is dropped after the fill.
ALTER TABLE events
    ADD COLUMN IF NOT EXISTS reminder_times JSONB NOT NULL
        DEFAULT '[{"minutes": -1440}, {"minutes": -60}, {"minutes": 0}]';

ALTER TABLE events ALTER COLUMN reminder_times DROP DEFAULT;

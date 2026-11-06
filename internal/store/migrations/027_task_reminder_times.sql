-- Each Task carries its own Reminder Times, as a JSON list of {"minutes": n}
-- and {"days": n, "at": "HH:MM"}. NULL means the built-in defaults for its
-- kind of due date, which depends on APP_TIMEZONE and so is worked out when
-- the Task is read: Tasks from before this change get the defaults that way.
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS reminder_times JSONB;

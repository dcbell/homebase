-- Scheduled Reminders are keyed by Reminder Time instead of stage, so several
-- Reminders of the same stage can exist for one recipient, subject and due
-- time. A Reminder Time is either timed (offset_minutes) or date-only
-- (offset_days at the local time of day at_time); the stage column is kept,
-- worked out from the Reminder Time's sign.
ALTER TABLE scheduled_reminders
    ADD COLUMN IF NOT EXISTS offset_minutes INTEGER,
    ADD COLUMN IF NOT EXISTS offset_days INTEGER,
    ADD COLUMN IF NOT EXISTS at_time TIME;

DROP INDEX IF EXISTS ux_scheduled_reminders_key;

-- Carry over rows scheduled from the fixed stages, which have no Reminder
-- Time yet, with the Reminder Time that now gives the same send time. Only
-- 08:00 was ever used for the date-only form, so Task Reminders sent neither
-- 24 hours before nor at the due time were date-only, which needs no
-- timezone to tell. hour_before becomes a Due Soon an hour before.
UPDATE scheduled_reminders
SET offset_days = CASE stage WHEN 'due_soon' THEN -1 WHEN 'due_now' THEN 0 ELSE 1 END,
    at_time = TIME '08:00'
WHERE offset_minutes IS NULL AND offset_days IS NULL
    AND subject_type = 'task'
    AND (stage = 'overdue' OR send_at - due_at NOT IN (INTERVAL '-24 hours', INTERVAL '0'));

UPDATE scheduled_reminders
SET offset_minutes = (EXTRACT(EPOCH FROM send_at - due_at) / 60)::INTEGER,
    stage = CASE stage WHEN 'hour_before' THEN 'due_soon' ELSE stage END
WHERE offset_minutes IS NULL AND offset_days IS NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'scheduled_reminders_reminder_time_form'
            AND conrelid = 'scheduled_reminders'::regclass
    ) THEN
        ALTER TABLE scheduled_reminders ADD CONSTRAINT scheduled_reminders_reminder_time_form CHECK (
            (offset_minutes IS NOT NULL AND offset_days IS NULL AND at_time IS NULL)
            OR (offset_minutes IS NULL AND offset_days IS NOT NULL AND at_time IS NOT NULL)
        );
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS ux_scheduled_reminders_reminder_time
    ON scheduled_reminders(recipient_id, subject_type, subject_id, offset_minutes, offset_days, at_time, due_at)
    NULLS NOT DISTINCT;

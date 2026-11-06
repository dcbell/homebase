-- Scheduled Reminders: one row per (recipient, subject, stage, due time).
-- The Notifier schedules them ahead; cancelling or rescheduling is a row update.
-- The Reminder runner claims due rows atomically by setting sent_at.
CREATE TABLE IF NOT EXISTS scheduled_reminders (
    id BIGSERIAL PRIMARY KEY,
    household_id BIGINT NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    recipient_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subject_type TEXT NOT NULL CHECK (subject_type IN ('task', 'appointment')),
    subject_id BIGINT NOT NULL,
    stage TEXT NOT NULL CHECK (stage IN ('due_soon', 'hour_before', 'due_now', 'overdue')),
    due_at TIMESTAMPTZ NOT NULL,
    send_at TIMESTAMPTZ NOT NULL,
    sent_at TIMESTAMPTZ,
    cancelled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 026 replaces this key with one per Reminder Time and drops it. Migrations
-- run again on every start, so only create it before that has happened: once
-- several Reminders share a stage it can no longer be built.
DO $$
BEGIN
    IF to_regclass('ux_scheduled_reminders_reminder_time') IS NULL THEN
        CREATE UNIQUE INDEX IF NOT EXISTS ux_scheduled_reminders_key
            ON scheduled_reminders(recipient_id, subject_type, subject_id, stage, due_at);
    END IF;
END $$;
CREATE INDEX IF NOT EXISTS idx_scheduled_reminders_subject
    ON scheduled_reminders(subject_type, subject_id);
CREATE INDEX IF NOT EXISTS idx_scheduled_reminders_pending
    ON scheduled_reminders(send_at) WHERE sent_at IS NULL AND cancelled_at IS NULL;

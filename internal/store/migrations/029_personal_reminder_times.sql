-- A Member's Personal Reminder Times on one Task or Appointment, as a JSON
-- list in the same form as the subject's own reminder_times. They replace
-- its Reminder Times for that Member alone. No row means the Member uses the
-- shared Reminder Times; an empty list means no Reminders.
CREATE TABLE IF NOT EXISTS personal_reminder_times (
    household_id BIGINT NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    subject_type TEXT NOT NULL CHECK (subject_type IN ('task', 'appointment')),
    subject_id BIGINT NOT NULL,
    member_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reminder_times JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (subject_type, subject_id, member_id)
);

CREATE INDEX IF NOT EXISTS idx_personal_reminder_times_member
    ON personal_reminder_times(household_id, member_id);

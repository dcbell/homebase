CREATE TABLE IF NOT EXISTS notifications (
    id BIGSERIAL PRIMARY KEY,
    household_id BIGINT NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    recipient_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('assignment', 'task_reminder', 'appointment_reminder')),
    stage TEXT CHECK (stage IN ('due_soon', 'hour_before', 'due_now', 'overdue')),
    subject_type TEXT NOT NULL CHECK (subject_type IN ('task', 'list_item', 'routine', 'appointment')),
    subject_id BIGINT NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    read_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_notifications_recipient ON notifications(household_id, recipient_id, id DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_unread ON notifications(household_id, recipient_id) WHERE read_at IS NULL;

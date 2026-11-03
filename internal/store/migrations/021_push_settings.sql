-- Which kinds of Notification each Member gets as Push. A missing row means
-- every kind is on. Turning a kind off stops the Push only: those
-- Notifications are still stored and count towards Unread.
CREATE TABLE IF NOT EXISTS push_settings (
    household_id BIGINT NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    assignment BOOLEAN NOT NULL DEFAULT TRUE,
    task_reminder BOOLEAN NOT NULL DEFAULT TRUE,
    appointment_reminder BOOLEAN NOT NULL DEFAULT TRUE,
    PRIMARY KEY (household_id, user_id)
);

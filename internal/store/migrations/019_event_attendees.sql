-- Attendees: the Members an Appointment (stored as an event) is for. An
-- Appointment with no rows here is for the whole Household, so existing
-- Appointments need no backfill.
CREATE TABLE IF NOT EXISTS event_attendees (
    event_id BIGINT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (event_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_event_attendees_user ON event_attendees(user_id);

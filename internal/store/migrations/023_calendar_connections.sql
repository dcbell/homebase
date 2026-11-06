-- Calendar Connections: one per device a Member connects to Homebase over
-- CalDAV (ADR-0004). Only a hash of each generated secret is kept.
CREATE TABLE IF NOT EXISTS calendar_connections (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    household_id BIGINT NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('ios', 'other')),
    secret_hash TEXT NOT NULL UNIQUE,
    secret_prefix TEXT NOT NULL,
    last_used_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_calendar_connections_user_household ON calendar_connections(user_id, household_id);

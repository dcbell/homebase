DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'users' AND column_name = 'google_sub'
    ) AND NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'users' AND column_name = 'oauth_subject'
    ) THEN
        ALTER TABLE users RENAME COLUMN google_sub TO oauth_subject;
    END IF;
END $$;

ALTER TABLE users ADD COLUMN IF NOT EXISTS oauth_subject TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS users_oauth_subject_key ON users(oauth_subject);
DROP TABLE IF EXISTS google_connections;

UPDATE events SET source = 'homebase' WHERE source <> 'homebase';
ALTER TABLE events DROP CONSTRAINT IF EXISTS events_source_check;
ALTER TABLE events ADD CONSTRAINT events_source_check CHECK (source IN ('homebase'));

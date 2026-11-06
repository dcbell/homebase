-- iPhone setup profiles (#37): an ios Calendar Connection is created with a
-- single-use, short-lived download link. Only a hash of the link's token is
-- kept; downloading the profile issues the connection's real secret and
-- clears the token.
ALTER TABLE calendar_connections ADD COLUMN IF NOT EXISTS profile_token_hash TEXT UNIQUE;
ALTER TABLE calendar_connections ADD COLUMN IF NOT EXISTS profile_token_expires_at TIMESTAMPTZ;

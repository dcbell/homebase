CREATE TABLE IF NOT EXISTS dashboard_tiles (
    household_id BIGINT NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    tile_key TEXT NOT NULL,
    position INTEGER NOT NULL,
    PRIMARY KEY (household_id, tile_key)
);

CREATE INDEX IF NOT EXISTS idx_dashboard_tiles_household_position ON dashboard_tiles(household_id, position);

package store

import (
	"context"
	"sort"
)

var defaultTileOrder = []string{
	"calendar",
	"tasks",
	"projects",
	"appointments",
	"list",
}

var dashboardTileNames = map[string]string{
	"calendar":     "Calendar",
	"tasks":        "Tasks",
	"projects":     "Projects",
	"appointments": "Appointments",
	"list":         "List",
}

func DashboardTiles() []DashboardTile {
	tiles := make([]DashboardTile, 0, len(defaultTileOrder))
	for _, key := range defaultTileOrder {
		tiles = append(tiles, DashboardTile{Key: key, Name: dashboardTileNames[key]})
	}
	return tiles
}

func (s *Store) DashboardTileOrder(ctx context.Context, householdID int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT tile_key
		FROM dashboard_tiles
		WHERE household_id = $1
		ORDER BY position, tile_key
	`, householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	order := []string{}
	seen := map[string]bool{}
	hasSavedRows := false
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		hasSavedRows = true
		if validTile(key) && !seen[key] {
			order = append(order, key)
			seen[key] = true
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if hasSavedRows {
		for _, key := range defaultTileOrder {
			if !seen[key] {
				order = append(order, key)
				seen[key] = true
			}
		}
		return order, nil
	}

	for _, key := range defaultTileOrder {
		if !seen[key] {
			order = append(order, key)
		}
	}
	return order, nil
}

func (s *Store) MoveDashboardTile(ctx context.Context, householdID int64, tileKey, direction string) error {
	if !validTile(tileKey) {
		return ErrNotFound
	}
	order, err := s.DashboardTileOrder(ctx, householdID)
	if err != nil {
		return err
	}

	index := -1
	for i, key := range order {
		if key == tileKey {
			index = i
			break
		}
	}
	if index == -1 {
		return ErrNotFound
	}

	switch direction {
	case "up":
		if index > 0 {
			order[index], order[index-1] = order[index-1], order[index]
		}
	case "down":
		if index < len(order)-1 {
			order[index], order[index+1] = order[index+1], order[index]
		}
	default:
		return ErrNotFound
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for i, key := range order {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO dashboard_tiles (household_id, tile_key, position)
			VALUES ($1, $2, $3)
			ON CONFLICT (household_id, tile_key) DO UPDATE SET position = EXCLUDED.position
		`, householdID, key, i); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) SetDashboardTileOrder(ctx context.Context, householdID int64, tiles []string) error {
	seen := map[string]bool{}
	order := []string{}
	for _, key := range tiles {
		if validTile(key) && !seen[key] {
			order = append(order, key)
			seen[key] = true
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM dashboard_tiles
		WHERE household_id = $1
	`, householdID); err != nil {
		return err
	}

	for i, key := range order {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO dashboard_tiles (household_id, tile_key, position)
			VALUES ($1, $2, $3)
		`, householdID, key, i); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func validTile(key string) bool {
	i := sort.SearchStrings(sortedTileKeys(), key)
	keys := sortedTileKeys()
	return i < len(keys) && keys[i] == key
}

func sortedTileKeys() []string {
	keys := append([]string{}, defaultTileOrder...)
	sort.Strings(keys)
	return keys
}

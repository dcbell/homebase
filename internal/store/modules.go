package store

import "context"

func (s *Store) ListModuleItems(ctx context.Context, householdID int64, table string) ([]ModuleItem, error) {
	var query string
	switch table {
	case "routines":
		query = `SELECT id, title, '' AS name, '' AS kind, cadence, next_due_at, created_at FROM routines WHERE household_id = $1 AND status <> 'archived' ORDER BY next_due_at NULLS LAST, created_at DESC`
	case "lists":
		query = `SELECT id, title, '' AS name, kind, '' AS cadence, NULL::timestamptz AS next_due_at, created_at FROM household_lists WHERE household_id = $1 AND status <> 'archived' ORDER BY updated_at DESC, created_at DESC`
	case "contacts":
		query = `SELECT id, '' AS title, name, kind, '' AS cadence, NULL::timestamptz AS next_due_at, created_at FROM contacts WHERE household_id = $1 AND status <> 'archived' ORDER BY name`
	case "assets":
		query = `SELECT id, '' AS title, name, kind, '' AS cadence, NULL::timestamptz AS next_due_at, created_at FROM assets WHERE household_id = $1 AND status <> 'archived' ORDER BY name`
	case "documents":
		query = `SELECT id, title, '' AS name, kind, '' AS cadence, NULL::timestamptz AS next_due_at, created_at FROM documents WHERE household_id = $1 AND status <> 'archived' ORDER BY updated_at DESC, created_at DESC`
	default:
		return nil, ErrNotFound
	}

	rows, err := s.db.QueryContext(ctx, query, householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []ModuleItem{}
	for rows.Next() {
		var item ModuleItem
		if err := rows.Scan(&item.ID, &item.Title, &item.Name, &item.Kind, &item.Cadence, &item.NextDueAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

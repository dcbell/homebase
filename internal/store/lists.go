package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

func (s *Store) ListHouseholdLists(ctx context.Context, householdID int64) ([]HouseholdList, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, household_id, title, description, kind, status, created_by, created_at, updated_at
		FROM household_lists
		WHERE household_id = $1 AND status <> 'archived'
		ORDER BY updated_at DESC, created_at DESC
	`, householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	lists := []HouseholdList{}
	for rows.Next() {
		var list HouseholdList
		if err := scanHouseholdList(rows, &list); err != nil {
			return nil, err
		}
		lists = append(lists, list)
	}
	return lists, rows.Err()
}

func (s *Store) CreateHouseholdList(ctx context.Context, householdID, userID int64, input HouseholdListInput) (HouseholdList, error) {
	if err := validateHouseholdListInput(&input); err != nil {
		return HouseholdList{}, err
	}

	var list HouseholdList
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO household_lists (household_id, title, description, kind, status, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, household_id, title, description, kind, status, created_by, created_at, updated_at
	`, householdID, input.Title, input.Description, input.Kind, input.Status, userID).Scan(
		&list.ID, &list.HouseholdID, &list.Title, &list.Description, &list.Kind, &list.Status, &list.CreatedBy, &list.CreatedAt, &list.UpdatedAt,
	)
	return list, err
}

func (s *Store) GetHouseholdList(ctx context.Context, householdID, id int64) (HouseholdList, error) {
	var list HouseholdList
	err := s.db.QueryRowContext(ctx, `
		SELECT id, household_id, title, description, kind, status, created_by, created_at, updated_at
		FROM household_lists
		WHERE household_id = $1 AND id = $2
	`, householdID, id).Scan(
		&list.ID, &list.HouseholdID, &list.Title, &list.Description, &list.Kind, &list.Status, &list.CreatedBy, &list.CreatedAt, &list.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return HouseholdList{}, ErrNotFound
	}
	return list, err
}

func (s *Store) UpdateHouseholdList(ctx context.Context, householdID, id int64, input HouseholdListInput) (HouseholdList, error) {
	if err := validateHouseholdListInput(&input); err != nil {
		return HouseholdList{}, err
	}

	var list HouseholdList
	err := s.db.QueryRowContext(ctx, `
		UPDATE household_lists
		SET title = $3,
			description = $4,
			kind = $5,
			status = $6,
			updated_at = now()
		WHERE household_id = $1 AND id = $2
		RETURNING id, household_id, title, description, kind, status, created_by, created_at, updated_at
	`, householdID, id, input.Title, input.Description, input.Kind, input.Status).Scan(
		&list.ID, &list.HouseholdID, &list.Title, &list.Description, &list.Kind, &list.Status, &list.CreatedBy, &list.CreatedAt, &list.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return HouseholdList{}, ErrNotFound
	}
	return list, err
}

func (s *Store) ArchiveHouseholdList(ctx context.Context, householdID, id int64) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE household_lists
		SET status = 'archived', updated_at = now()
		WHERE household_id = $1 AND id = $2
	`, householdID, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListListItems(ctx context.Context, householdID, listID int64) ([]ListItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT li.id, li.household_id, li.list_id, li.assigned_to, COALESCE(assigned.name, ''), li.title, li.notes, li.status, li.due_at, li.completed_at, li.created_by, li.created_at, li.updated_at
		FROM list_items li
		LEFT JOIN users assigned ON assigned.id = li.assigned_to
		WHERE li.household_id = $1 AND li.list_id = $2 AND li.status <> 'archived'
		ORDER BY
			CASE li.status WHEN 'open' THEN 1 ELSE 2 END,
			COALESCE(li.due_at, '9999-12-31'::timestamptz),
			li.created_at DESC
	`, householdID, listID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []ListItem{}
	for rows.Next() {
		var item ListItem
		if err := scanListItem(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) CreateListItem(ctx context.Context, householdID, userID, listID int64, input ListItemInput) (ListItem, error) {
	if err := s.validateListItemInput(ctx, householdID, listID, &input); err != nil {
		return ListItem{}, err
	}

	var item ListItem
	err := s.db.QueryRowContext(ctx, `
		WITH inserted AS (
			INSERT INTO list_items (household_id, list_id, assigned_to, title, notes, status, due_at, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING id, household_id, list_id, assigned_to, title, notes, status, due_at, completed_at, created_by, created_at, updated_at
		)
		SELECT inserted.id, inserted.household_id, inserted.list_id, inserted.assigned_to, COALESCE(assigned.name, ''), inserted.title, inserted.notes, inserted.status, inserted.due_at, inserted.completed_at, inserted.created_by, inserted.created_at, inserted.updated_at
		FROM inserted
		LEFT JOIN users assigned ON assigned.id = inserted.assigned_to
	`, householdID, listID, input.AssignedTo, input.Title, input.Notes, input.Status, input.DueAt, userID).Scan(
		&item.ID, &item.HouseholdID, &item.ListID, &item.AssignedTo, &item.AssignedName, &item.Title, &item.Notes, &item.Status, &item.DueAt, &item.CompletedAt, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt,
	)
	return item, err
}

func (s *Store) UpdateListItem(ctx context.Context, householdID, id int64, input ListItemInput) (ListItem, error) {
	listID, err := s.listIDForItem(ctx, householdID, id)
	if errors.Is(err, sql.ErrNoRows) {
		return ListItem{}, ErrNotFound
	}
	if err != nil {
		return ListItem{}, err
	}
	if err := s.validateListItemInput(ctx, householdID, listID, &input); err != nil {
		return ListItem{}, err
	}

	var item ListItem
	err = s.db.QueryRowContext(ctx, `
		WITH updated AS (
			UPDATE list_items
			SET assigned_to = $3,
				title = $4,
				notes = $5,
				status = $6,
				due_at = $7,
				completed_at = CASE
					WHEN $6 = 'done' AND completed_at IS NULL THEN now()
					WHEN $6 <> 'done' THEN NULL
					ELSE completed_at
				END,
				updated_at = now()
			WHERE household_id = $1 AND id = $2
			RETURNING id, household_id, list_id, assigned_to, title, notes, status, due_at, completed_at, created_by, created_at, updated_at
		)
		SELECT updated.id, updated.household_id, updated.list_id, updated.assigned_to, COALESCE(assigned.name, ''), updated.title, updated.notes, updated.status, updated.due_at, updated.completed_at, updated.created_by, updated.created_at, updated.updated_at
		FROM updated
		LEFT JOIN users assigned ON assigned.id = updated.assigned_to
	`, householdID, id, input.AssignedTo, input.Title, input.Notes, input.Status, input.DueAt).Scan(
		&item.ID, &item.HouseholdID, &item.ListID, &item.AssignedTo, &item.AssignedName, &item.Title, &item.Notes, &item.Status, &item.DueAt, &item.CompletedAt, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ListItem{}, ErrNotFound
	}
	return item, err
}

func (s *Store) CompleteListItem(ctx context.Context, householdID, id int64) (ListItem, error) {
	var item ListItem
	err := s.db.QueryRowContext(ctx, `
		WITH updated AS (
			UPDATE list_items
			SET status = 'done', completed_at = now(), updated_at = now()
			WHERE household_id = $1 AND id = $2
			RETURNING id, household_id, list_id, assigned_to, title, notes, status, due_at, completed_at, created_by, created_at, updated_at
		)
		SELECT updated.id, updated.household_id, updated.list_id, updated.assigned_to, COALESCE(assigned.name, ''), updated.title, updated.notes, updated.status, updated.due_at, updated.completed_at, updated.created_by, updated.created_at, updated.updated_at
		FROM updated
		LEFT JOIN users assigned ON assigned.id = updated.assigned_to
	`, householdID, id).Scan(
		&item.ID, &item.HouseholdID, &item.ListID, &item.AssignedTo, &item.AssignedName, &item.Title, &item.Notes, &item.Status, &item.DueAt, &item.CompletedAt, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ListItem{}, ErrNotFound
	}
	return item, err
}

func (s *Store) ReopenListItem(ctx context.Context, householdID, id int64) (ListItem, error) {
	var item ListItem
	err := s.db.QueryRowContext(ctx, `
		WITH updated AS (
			UPDATE list_items
			SET status = 'open', completed_at = NULL, updated_at = now()
			WHERE household_id = $1 AND id = $2
			RETURNING id, household_id, list_id, assigned_to, title, notes, status, due_at, completed_at, created_by, created_at, updated_at
		)
		SELECT updated.id, updated.household_id, updated.list_id, updated.assigned_to, COALESCE(assigned.name, ''), updated.title, updated.notes, updated.status, updated.due_at, updated.completed_at, updated.created_by, updated.created_at, updated.updated_at
		FROM updated
		LEFT JOIN users assigned ON assigned.id = updated.assigned_to
	`, householdID, id).Scan(
		&item.ID, &item.HouseholdID, &item.ListID, &item.AssignedTo, &item.AssignedName, &item.Title, &item.Notes, &item.Status, &item.DueAt, &item.CompletedAt, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ListItem{}, ErrNotFound
	}
	return item, err
}

func (s *Store) ArchiveListItem(ctx context.Context, householdID, id int64) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE list_items
		SET status = 'archived', updated_at = now()
		WHERE household_id = $1 AND id = $2
	`, householdID, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func validateHouseholdListInput(input *HouseholdListInput) error {
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		return errors.New("title is required")
	}
	if input.Kind == "" {
		input.Kind = "general"
	}
	if input.Status == "" {
		input.Status = "active"
	}
	if input.Status != "active" && input.Status != "archived" {
		return errors.New("status must be active or archived")
	}
	return nil
}

func (s *Store) validateListItemInput(ctx context.Context, householdID, listID int64, input *ListItemInput) error {
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		return errors.New("title is required")
	}
	if input.Status == "" {
		input.Status = "open"
	}
	if input.Status != "open" && input.Status != "done" && input.Status != "archived" {
		return errors.New("status must be open, done, or archived")
	}
	ok, err := s.listInHousehold(ctx, householdID, listID)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("list must belong to this household")
	}
	if input.AssignedTo != nil {
		ok, err := s.userInHousehold(ctx, householdID, *input.AssignedTo)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("assigned user must belong to this household")
		}
	}
	return nil
}

func (s *Store) listInHousehold(ctx context.Context, householdID, listID int64) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM household_lists
			WHERE household_id = $1 AND id = $2 AND status <> 'archived'
		)
	`, householdID, listID).Scan(&exists)
	return exists, err
}

func (s *Store) listIDForItem(ctx context.Context, householdID, itemID int64) (int64, error) {
	var listID int64
	err := s.db.QueryRowContext(ctx, `
		SELECT list_id
		FROM list_items
		WHERE household_id = $1 AND id = $2 AND status <> 'archived'
	`, householdID, itemID).Scan(&listID)
	return listID, err
}

type householdListScanner interface {
	Scan(dest ...any) error
}

func scanHouseholdList(scanner householdListScanner, list *HouseholdList) error {
	return scanner.Scan(&list.ID, &list.HouseholdID, &list.Title, &list.Description, &list.Kind, &list.Status, &list.CreatedBy, &list.CreatedAt, &list.UpdatedAt)
}

type listItemScanner interface {
	Scan(dest ...any) error
}

func scanListItem(scanner listItemScanner, item *ListItem) error {
	return scanner.Scan(&item.ID, &item.HouseholdID, &item.ListID, &item.AssignedTo, &item.AssignedName, &item.Title, &item.Notes, &item.Status, &item.DueAt, &item.CompletedAt, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt)
}

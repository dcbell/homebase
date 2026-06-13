package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (s *Store) ListRoutines(ctx context.Context, householdID int64) ([]Routine, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.id, r.household_id, r.assigned_to, COALESCE(assigned.name, ''), r.title, r.notes, r.cadence, r.status, r.next_due_at, r.last_completed_at, r.created_by, r.created_at, r.updated_at
		FROM routines r
		LEFT JOIN users assigned ON assigned.id = r.assigned_to
		WHERE r.household_id = $1 AND r.status <> 'archived'
		ORDER BY r.next_due_at NULLS LAST, r.created_at DESC
	`, householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	routines := []Routine{}
	for rows.Next() {
		var routine Routine
		if err := scanRoutine(rows, &routine); err != nil {
			return nil, err
		}
		routines = append(routines, routine)
	}
	return routines, rows.Err()
}

func (s *Store) GetRoutine(ctx context.Context, householdID, id int64) (Routine, error) {
	var routine Routine
	err := s.db.QueryRowContext(ctx, `
		SELECT r.id, r.household_id, r.assigned_to, COALESCE(assigned.name, ''), r.title, r.notes, r.cadence, r.status, r.next_due_at, r.last_completed_at, r.created_by, r.created_at, r.updated_at
		FROM routines r
		LEFT JOIN users assigned ON assigned.id = r.assigned_to
		WHERE r.household_id = $1 AND r.id = $2
	`, householdID, id).Scan(
		&routine.ID, &routine.HouseholdID, &routine.AssignedTo, &routine.AssignedName, &routine.Title, &routine.Notes, &routine.Cadence, &routine.Status, &routine.NextDueAt, &routine.LastCompletedAt, &routine.CreatedBy, &routine.CreatedAt, &routine.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Routine{}, ErrNotFound
	}
	return routine, err
}

func (s *Store) CreateRoutine(ctx context.Context, householdID, userID int64, input RoutineInput) (Routine, error) {
	if err := s.validateRoutineInput(ctx, householdID, &input); err != nil {
		return Routine{}, err
	}

	var routine Routine
	err := s.db.QueryRowContext(ctx, `
		WITH inserted AS (
			INSERT INTO routines (household_id, assigned_to, title, notes, cadence, status, next_due_at, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING id, household_id, assigned_to, title, notes, cadence, status, next_due_at, last_completed_at, created_by, created_at, updated_at
		)
		SELECT inserted.id, inserted.household_id, inserted.assigned_to, COALESCE(assigned.name, ''), inserted.title, inserted.notes, inserted.cadence, inserted.status, inserted.next_due_at, inserted.last_completed_at, inserted.created_by, inserted.created_at, inserted.updated_at
		FROM inserted
		LEFT JOIN users assigned ON assigned.id = inserted.assigned_to
	`, householdID, input.AssignedTo, input.Title, input.Notes, input.Cadence, input.Status, input.NextDueAt, userID).Scan(
		&routine.ID, &routine.HouseholdID, &routine.AssignedTo, &routine.AssignedName, &routine.Title, &routine.Notes, &routine.Cadence, &routine.Status, &routine.NextDueAt, &routine.LastCompletedAt, &routine.CreatedBy, &routine.CreatedAt, &routine.UpdatedAt,
	)
	return routine, err
}

func (s *Store) UpdateRoutine(ctx context.Context, householdID, id int64, input RoutineInput) (Routine, error) {
	if err := s.validateRoutineInput(ctx, householdID, &input); err != nil {
		return Routine{}, err
	}

	var routine Routine
	err := s.db.QueryRowContext(ctx, `
		WITH updated AS (
			UPDATE routines
			SET assigned_to = $3,
				title = $4,
				notes = $5,
				cadence = $6,
				status = $7,
				next_due_at = $8,
				updated_at = now()
			WHERE household_id = $1 AND id = $2
			RETURNING id, household_id, assigned_to, title, notes, cadence, status, next_due_at, last_completed_at, created_by, created_at, updated_at
		)
		SELECT updated.id, updated.household_id, updated.assigned_to, COALESCE(assigned.name, ''), updated.title, updated.notes, updated.cadence, updated.status, updated.next_due_at, updated.last_completed_at, updated.created_by, updated.created_at, updated.updated_at
		FROM updated
		LEFT JOIN users assigned ON assigned.id = updated.assigned_to
	`, householdID, id, input.AssignedTo, input.Title, input.Notes, input.Cadence, input.Status, input.NextDueAt).Scan(
		&routine.ID, &routine.HouseholdID, &routine.AssignedTo, &routine.AssignedName, &routine.Title, &routine.Notes, &routine.Cadence, &routine.Status, &routine.NextDueAt, &routine.LastCompletedAt, &routine.CreatedBy, &routine.CreatedAt, &routine.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Routine{}, ErrNotFound
	}
	return routine, err
}

func (s *Store) ArchiveRoutine(ctx context.Context, householdID, id int64) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE routines
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

func (s *Store) GenerateRoutineTask(ctx context.Context, householdID, userID, id int64) (Task, error) {
	routine, err := s.GetRoutine(ctx, householdID, id)
	if err != nil {
		return Task{}, err
	}
	if routine.Status == "archived" {
		return Task{}, errors.New("routine is archived")
	}
	if routine.NextDueAt != nil {
		task, err := s.getOpenRoutineTaskForDue(ctx, routine.ID, *routine.NextDueAt)
		if err == nil {
			return task, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return Task{}, err
		}
	}

	input := TaskInput{
		RoutineID:  &routine.ID,
		AssignedTo: routine.AssignedTo,
		Title:      routine.Title,
		Notes:      routine.Notes,
		Priority:   "normal",
		DueAt:      routine.NextDueAt,
	}
	return s.CreateTask(ctx, householdID, userID, input)
}

func (s *Store) getOpenRoutineTaskForDue(ctx context.Context, routineID int64, dueAt time.Time) (Task, error) {
	var task Task
	err := s.db.QueryRowContext(ctx, `
		SELECT t.id, t.household_id, t.project_id, t.project_folder_id, t.routine_id, t.asset_id, t.asset_maintenance_item_id, t.assigned_to, COALESCE(assigned.name, ''), t.title, t.notes, t.status, t.priority, t.due_at, t.completed_at, t.created_by, t.created_at, t.updated_at
		FROM tasks t
		LEFT JOIN users assigned ON assigned.id = t.assigned_to
		WHERE t.routine_id = $1
			AND t.status = 'open'
			AND t.due_at IS NOT DISTINCT FROM $2
		LIMIT 1
	`, routineID, dueAt).Scan(
		&task.ID, &task.HouseholdID, &task.ProjectID, &task.ProjectFolderID, &task.RoutineID, &task.AssetID, &task.AssetMaintenanceItemID, &task.AssignedTo, &task.AssignedName, &task.Title, &task.Notes, &task.Status, &task.Priority, &task.DueAt, &task.CompletedAt, &task.CreatedBy, &task.CreatedAt, &task.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	return task, err
}

func (s *Store) GenerateDueRoutineTasks(ctx context.Context, now time.Time) (int64, error) {
	rows, err := s.db.QueryContext(ctx, `
		INSERT INTO tasks (household_id, routine_id, assigned_to, title, notes, priority, due_at, created_by)
		SELECT r.household_id, r.id, r.assigned_to, r.title, r.notes, 'normal', r.next_due_at, r.created_by
		FROM routines r
		WHERE r.status = 'active'
			AND r.next_due_at IS NOT NULL
			AND r.next_due_at <= $1
			AND NOT EXISTS (
				SELECT 1
				FROM tasks t
				WHERE t.routine_id = r.id
					AND t.status = 'open'
					AND t.due_at IS NOT DISTINCT FROM r.next_due_at
			)
		ON CONFLICT DO NOTHING
		RETURNING id
	`, now)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var count int64
	for rows.Next() {
		count++
	}
	return count, rows.Err()
}

func (s *Store) CompleteRoutine(ctx context.Context, householdID, id int64) error {
	routine, err := s.GetRoutine(ctx, householdID, id)
	if err != nil {
		return err
	}

	nextDue, err := nextDueForCadence(routine.NextDueAt, routine.Cadence)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, `
		UPDATE routines
		SET last_completed_at = now(),
			next_due_at = $3,
			updated_at = now()
		WHERE household_id = $1 AND id = $2
	`, householdID, id, nextDue)
	return err
}

func (s *Store) RoutineNotices(ctx context.Context, householdID int64) ([]RoutineNotice, error) {
	routines, err := s.ListRoutines(ctx, householdID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	soon := now.Add(7 * 24 * time.Hour)
	notices := []RoutineNotice{}
	for _, routine := range routines {
		if routine.NextDueAt == nil {
			continue
		}
		if routine.NextDueAt.Before(now) {
			notices = append(notices, RoutineNotice{
				Routine: routine,
				Kind:    "overdue",
				Message: fmt.Sprintf("%s is overdue", routine.Title),
			})
			continue
		}
		if routine.NextDueAt.Before(soon) {
			notices = append(notices, RoutineNotice{
				Routine: routine,
				Kind:    "upcoming",
				Message: fmt.Sprintf("%s is due soon", routine.Title),
			})
		}
	}
	return notices, nil
}

func (s *Store) validateRoutineInput(ctx context.Context, householdID int64, input *RoutineInput) error {
	input.Title = strings.TrimSpace(input.Title)
	input.Cadence = strings.TrimSpace(input.Cadence)
	if input.Title == "" {
		return errors.New("title is required")
	}
	if input.Cadence == "" {
		input.Cadence = "monthly"
	}
	if !validCadence(input.Cadence) {
		return errors.New("cadence must be daily, weekly, monthly, quarterly, or yearly")
	}
	if input.Status == "" {
		input.Status = "active"
	}
	if input.Status != "active" && input.Status != "archived" {
		return errors.New("status must be active or archived")
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

func validCadence(cadence string) bool {
	switch cadence {
	case "daily", "weekly", "monthly", "quarterly", "yearly":
		return true
	default:
		return false
	}
}

func nextDueForCadence(current *time.Time, cadence string) (time.Time, error) {
	base := time.Now()
	if current != nil && !current.IsZero() {
		base = *current
	}

	var next time.Time
	switch cadence {
	case "daily":
		next = base.AddDate(0, 0, 1)
	case "weekly":
		next = base.AddDate(0, 0, 7)
	case "monthly":
		next = base.AddDate(0, 1, 0)
	case "quarterly":
		next = base.AddDate(0, 3, 0)
	case "yearly":
		next = base.AddDate(1, 0, 0)
	default:
		return time.Time{}, errors.New("unsupported cadence")
	}

	now := time.Now()
	for next.Before(now) {
		base = next
		switch cadence {
		case "daily":
			next = base.AddDate(0, 0, 1)
		case "weekly":
			next = base.AddDate(0, 0, 7)
		case "monthly":
			next = base.AddDate(0, 1, 0)
		case "quarterly":
			next = base.AddDate(0, 3, 0)
		case "yearly":
			next = base.AddDate(1, 0, 0)
		}
	}
	return next, nil
}

func (s *Store) routineInHousehold(ctx context.Context, householdID, routineID int64) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM routines
			WHERE household_id = $1 AND id = $2
		)
	`, householdID, routineID).Scan(&exists)
	return exists, err
}

type routineScanner interface {
	Scan(dest ...any) error
}

func scanRoutine(scanner routineScanner, routine *Routine) error {
	return scanner.Scan(&routine.ID, &routine.HouseholdID, &routine.AssignedTo, &routine.AssignedName, &routine.Title, &routine.Notes, &routine.Cadence, &routine.Status, &routine.NextDueAt, &routine.LastCompletedAt, &routine.CreatedBy, &routine.CreatedAt, &routine.UpdatedAt)
}

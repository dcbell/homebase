package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (s *Store) ListRoutines(ctx context.Context, householdID int64) ([]Routine, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.id, r.household_id, r.assigned_to, COALESCE(assigned.name, ''), r.title, r.notes, r.cadence, r.status, r.next_due_at, r.last_completed_at, r.created_by, r.created_at, r.updated_at, r.reminder_times
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
		SELECT r.id, r.household_id, r.assigned_to, COALESCE(assigned.name, ''), r.title, r.notes, r.cadence, r.status, r.next_due_at, r.last_completed_at, r.created_by, r.created_at, r.updated_at, r.reminder_times
		FROM routines r
		LEFT JOIN users assigned ON assigned.id = r.assigned_to
		WHERE r.household_id = $1 AND r.id = $2
	`, householdID, id).Scan(
		&routine.ID, &routine.HouseholdID, &routine.AssignedTo, &routine.AssignedName, &routine.Title, &routine.Notes, &routine.Cadence, &routine.Status, &routine.NextDueAt, &routine.LastCompletedAt, &routine.CreatedBy, &routine.CreatedAt, &routine.UpdatedAt, sourceReminderTimes{&routine.ReminderTimes, &routine.NextDueAt},
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
	reminderTimes, err := newReminderTimes(input.NextDueAt, input.ReminderTimes)
	if err != nil {
		return Routine{}, err
	}

	var routine Routine
	err = s.db.QueryRowContext(ctx, `
		WITH inserted AS (
			INSERT INTO routines (household_id, assigned_to, title, notes, cadence, status, next_due_at, created_by, reminder_times)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			RETURNING id, household_id, assigned_to, title, notes, cadence, status, next_due_at, last_completed_at, created_by, created_at, updated_at, reminder_times
		)
		SELECT inserted.id, inserted.household_id, inserted.assigned_to, COALESCE(assigned.name, ''), inserted.title, inserted.notes, inserted.cadence, inserted.status, inserted.next_due_at, inserted.last_completed_at, inserted.created_by, inserted.created_at, inserted.updated_at, inserted.reminder_times
		FROM inserted
		LEFT JOIN users assigned ON assigned.id = inserted.assigned_to
	`, householdID, input.AssignedTo, input.Title, input.Notes, input.Cadence, input.Status, input.NextDueAt, userID, reminderTimes).Scan(
		&routine.ID, &routine.HouseholdID, &routine.AssignedTo, &routine.AssignedName, &routine.Title, &routine.Notes, &routine.Cadence, &routine.Status, &routine.NextDueAt, &routine.LastCompletedAt, &routine.CreatedBy, &routine.CreatedAt, &routine.UpdatedAt, sourceReminderTimes{&routine.ReminderTimes, &routine.NextDueAt},
	)
	return routine, err
}

// UpdateRoutine replaces a Routine's fields. It also returns the Routine as it
// was just before the update, so callers can tell what changed.
func (s *Store) UpdateRoutine(ctx context.Context, householdID, id int64, input RoutineInput) (updated Routine, previous Routine, err error) {
	if err := s.validateRoutineInput(ctx, householdID, &input); err != nil {
		return Routine{}, Routine{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Routine{}, Routine{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var previousDue *time.Time
	err = tx.QueryRowContext(ctx, `
		SELECT next_due_at FROM routines WHERE household_id = $1 AND id = $2 FOR UPDATE
	`, householdID, id).Scan(&previousDue)
	if errors.Is(err, sql.ErrNoRows) {
		return Routine{}, Routine{}, ErrNotFound
	}
	if err != nil {
		return Routine{}, Routine{}, err
	}
	reminderTimes, err := updatedReminderTimes(previousDue, input.NextDueAt, input.ReminderTimes)
	if err != nil {
		return Routine{}, Routine{}, err
	}

	var routine, prev Routine
	err = tx.QueryRowContext(ctx, `
		WITH previous AS (
			SELECT id, household_id, assigned_to, title, notes, cadence, status, next_due_at, last_completed_at, created_by, created_at, updated_at, reminder_times
			FROM routines
			WHERE household_id = $1 AND id = $2
		), updated AS (
			UPDATE routines
			SET assigned_to = $3,
				title = $4,
				notes = $5,
				cadence = $6,
				status = $7,
				next_due_at = $8,
				reminder_times = COALESCE($9::jsonb, reminder_times),
				updated_at = now()
			WHERE household_id = $1 AND id = $2
			RETURNING id, household_id, assigned_to, title, notes, cadence, status, next_due_at, last_completed_at, created_by, created_at, updated_at, reminder_times
		)
		SELECT updated.id, updated.household_id, updated.assigned_to, COALESCE(assigned.name, ''), updated.title, updated.notes, updated.cadence, updated.status, updated.next_due_at, updated.last_completed_at, updated.created_by, updated.created_at, updated.updated_at, updated.reminder_times,
			previous.id, previous.household_id, previous.assigned_to, COALESCE(previous_assigned.name, ''), previous.title, previous.notes, previous.cadence, previous.status, previous.next_due_at, previous.last_completed_at, previous.created_by, previous.created_at, previous.updated_at, previous.reminder_times
		FROM updated
		JOIN previous ON previous.id = updated.id
		LEFT JOIN users assigned ON assigned.id = updated.assigned_to
		LEFT JOIN users previous_assigned ON previous_assigned.id = previous.assigned_to
	`, householdID, id, input.AssignedTo, input.Title, input.Notes, input.Cadence, input.Status, input.NextDueAt, reminderTimes).Scan(
		&routine.ID, &routine.HouseholdID, &routine.AssignedTo, &routine.AssignedName, &routine.Title, &routine.Notes, &routine.Cadence, &routine.Status, &routine.NextDueAt, &routine.LastCompletedAt, &routine.CreatedBy, &routine.CreatedAt, &routine.UpdatedAt, sourceReminderTimes{&routine.ReminderTimes, &routine.NextDueAt},
		&prev.ID, &prev.HouseholdID, &prev.AssignedTo, &prev.AssignedName, &prev.Title, &prev.Notes, &prev.Cadence, &prev.Status, &prev.NextDueAt, &prev.LastCompletedAt, &prev.CreatedBy, &prev.CreatedAt, &prev.UpdatedAt, sourceReminderTimes{&prev.ReminderTimes, &prev.NextDueAt},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Routine{}, Routine{}, ErrNotFound
	}
	if err != nil {
		return Routine{}, Routine{}, err
	}
	return routine, prev, tx.Commit()
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

// GenerateRoutineTask creates the Task for a Routine's next due date. It is
// idempotent: when an open Task for that due date already exists it is
// returned with created false.
func (s *Store) GenerateRoutineTask(ctx context.Context, householdID, userID, id int64) (task Task, created bool, err error) {
	routine, err := s.GetRoutine(ctx, householdID, id)
	if err != nil {
		return Task{}, false, err
	}
	if routine.Status == "archived" {
		return Task{}, false, errors.New("routine is archived")
	}
	if routine.NextDueAt != nil {
		task, err := s.getOpenRoutineTaskForDue(ctx, routine.ID, *routine.NextDueAt)
		if err == nil {
			return task, false, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return Task{}, false, err
		}
	}

	input := TaskInput{
		RoutineID:  &routine.ID,
		AssignedTo: routine.AssignedTo,
		Title:      routine.Title,
		Notes:      routine.Notes,
		Priority:   "normal",
		DueAt:      routine.NextDueAt,
		// A copy, so later edits to the Routine don't reach this Task.
		ReminderTimes: routine.ReminderTimes,
	}
	task, err = s.CreateTask(ctx, householdID, userID, input)
	if err != nil {
		return Task{}, false, err
	}
	return task, true, nil
}

func (s *Store) getOpenRoutineTaskForDue(ctx context.Context, routineID int64, dueAt time.Time) (Task, error) {
	var task Task
	err := s.db.QueryRowContext(ctx, `
		SELECT t.id, t.household_id, t.project_id, t.project_folder_id, t.routine_id, t.asset_id, t.asset_maintenance_item_id, t.assigned_to, COALESCE(assigned.name, ''), t.title, t.notes, t.status, t.priority, t.due_at, t.completed_at, t.created_by, t.created_at, t.updated_at, COALESCE(t.calendar_uid, ''), t.reminder_times
		FROM tasks t
		LEFT JOIN users assigned ON assigned.id = t.assigned_to
		WHERE t.routine_id = $1
			AND t.status = 'open'
			AND t.due_at IS NOT DISTINCT FROM $2
		LIMIT 1
	`, routineID, dueAt).Scan(
		&task.ID, &task.HouseholdID, &task.ProjectID, &task.ProjectFolderID, &task.RoutineID, &task.AssetID, &task.AssetMaintenanceItemID, &task.AssignedTo, &task.AssignedName, &task.Title, &task.Notes, &task.Status, &task.Priority, &task.DueAt, &task.CompletedAt, &task.CreatedBy, &task.CreatedAt, &task.UpdatedAt, &task.CalendarUID, taskReminderTimes{&task},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	return task, err
}

// GenerateDueRoutineTasks creates the Task for every active Routine due as of
// now that has no open Task for that due date yet, and returns the Tasks it
// created.
func (s *Store) GenerateDueRoutineTasks(ctx context.Context, now time.Time) ([]Task, error) {
	rows, err := s.db.QueryContext(ctx, `
		WITH inserted AS (
			INSERT INTO tasks (household_id, routine_id, assigned_to, title, notes, priority, due_at, created_by, reminder_times)
			SELECT r.household_id, r.id, r.assigned_to, r.title, r.notes, 'normal', r.next_due_at, r.created_by, r.reminder_times
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
			RETURNING id, household_id, project_id, project_folder_id, routine_id, asset_id, asset_maintenance_item_id, assigned_to, title, notes, status, priority, due_at, completed_at, created_by, created_at, updated_at, calendar_uid, reminder_times
		)
		SELECT inserted.id, inserted.household_id, inserted.project_id, inserted.project_folder_id, inserted.routine_id, inserted.asset_id, inserted.asset_maintenance_item_id, inserted.assigned_to, COALESCE(assigned.name, ''), inserted.title, inserted.notes, inserted.status, inserted.priority, inserted.due_at, inserted.completed_at, inserted.created_by, inserted.created_at, inserted.updated_at, COALESCE(inserted.calendar_uid, ''), inserted.reminder_times
		FROM inserted
		LEFT JOIN users assigned ON assigned.id = inserted.assigned_to
		ORDER BY inserted.id
	`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := []Task{}
	for rows.Next() {
		var task Task
		if err := scanTask(rows, &task); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
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
	return scanner.Scan(&routine.ID, &routine.HouseholdID, &routine.AssignedTo, &routine.AssignedName, &routine.Title, &routine.Notes, &routine.Cadence, &routine.Status, &routine.NextDueAt, &routine.LastCompletedAt, &routine.CreatedBy, &routine.CreatedAt, &routine.UpdatedAt, sourceReminderTimes{&routine.ReminderTimes, &routine.NextDueAt})
}

// sourceReminderTimes scans the reminder_times column of a Routine or
// Maintenance Schedule into its ReminderTimes. NULL means the built-in
// defaults for its next due date, so it must come after next_due_at in the
// Scan, which fills columns in order.
type sourceReminderTimes struct {
	times   *[]ReminderTime
	nextDue **time.Time
}

func (s sourceReminderTimes) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*s.times = defaultTaskReminderTimes(*s.nextDue)
		return nil
	case string:
		return json.Unmarshal([]byte(v), s.times)
	case []byte:
		return json.Unmarshal(v, s.times)
	}
	return fmt.Errorf("scan reminder_times: unexpected %T", src)
}

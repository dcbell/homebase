package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

type TaskInput struct {
	ProjectID              *int64     `json:"project_id"`
	ProjectFolderID        *int64     `json:"project_folder_id"`
	RoutineID              *int64     `json:"routine_id"`
	AssetID                *int64     `json:"asset_id"`
	AssetMaintenanceItemID *int64     `json:"asset_maintenance_item_id"`
	AssignedTo             *int64     `json:"assigned_to"`
	Title                  string     `json:"title"`
	Notes                  string     `json:"notes"`
	Status                 string     `json:"status"`
	Priority               string     `json:"priority"`
	DueAt                  *time.Time `json:"due_at"`
	// CalendarUID is the UID a device chose when creating the Task over
	// CalDAV. It is only read on create.
	CalendarUID string `json:"-"`
}

func (s *Store) ListTasks(ctx context.Context, householdID int64) ([]Task, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.id, t.household_id, t.project_id, t.project_folder_id, t.routine_id, t.asset_id, t.asset_maintenance_item_id, t.assigned_to, COALESCE(assigned.name, ''), t.title, t.notes, t.status, t.priority, t.due_at, t.completed_at, t.created_by, t.created_at, t.updated_at, COALESCE(t.calendar_uid, '')
		FROM tasks t
		LEFT JOIN users assigned ON assigned.id = t.assigned_to
		WHERE t.household_id = $1 AND t.status <> 'archived'
		ORDER BY
			CASE t.status WHEN 'open' THEN 1 ELSE 2 END,
			COALESCE(t.due_at, '9999-12-31'::timestamptz),
			CASE t.priority WHEN 'high' THEN 1 WHEN 'normal' THEN 2 ELSE 3 END,
			t.created_at DESC
	`, householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := []Task{}
	for rows.Next() {
		var t Task
		if err := scanTask(rows, &t); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}

	return tasks, rows.Err()
}

// ListProjectTasks returns every Task of a Project, whatever its status.
func (s *Store) ListProjectTasks(ctx context.Context, householdID, projectID int64) ([]Task, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.id, t.household_id, t.project_id, t.project_folder_id, t.routine_id, t.asset_id, t.asset_maintenance_item_id, t.assigned_to, COALESCE(assigned.name, ''), t.title, t.notes, t.status, t.priority, t.due_at, t.completed_at, t.created_by, t.created_at, t.updated_at, COALESCE(t.calendar_uid, '')
		FROM tasks t
		LEFT JOIN users assigned ON assigned.id = t.assigned_to
		WHERE t.household_id = $1 AND t.project_id = $2
		ORDER BY t.id
	`, householdID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := []Task{}
	for rows.Next() {
		var t Task
		if err := scanTask(rows, &t); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// ListOpenTasksConcerning returns the Household's open Tasks that concern a
// Member, meaning they are assigned to the Member or to nobody, and that are
// either undated or due within [from, to).
func (s *Store) ListOpenTasksConcerning(ctx context.Context, householdID, userID int64, from, to time.Time) ([]Task, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.id, t.household_id, t.project_id, t.project_folder_id, t.routine_id, t.asset_id, t.asset_maintenance_item_id, t.assigned_to, COALESCE(assigned.name, ''), t.title, t.notes, t.status, t.priority, t.due_at, t.completed_at, t.created_by, t.created_at, t.updated_at, COALESCE(t.calendar_uid, '')
		FROM tasks t
		LEFT JOIN users assigned ON assigned.id = t.assigned_to
		WHERE t.household_id = $1
		  AND t.status = 'open'
		  AND (t.assigned_to = $2 OR t.assigned_to IS NULL)
		  AND (t.due_at IS NULL OR (t.due_at >= $3 AND t.due_at < $4))
		ORDER BY t.id
	`, householdID, userID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := []Task{}
	for rows.Next() {
		var t Task
		if err := scanTask(rows, &t); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (s *Store) CreateTask(ctx context.Context, householdID, userID int64, input TaskInput) (Task, error) {
	if err := s.validateTaskInput(ctx, householdID, &input); err != nil {
		return Task{}, err
	}

	var t Task
	err := s.db.QueryRowContext(ctx, `
		WITH inserted AS (
			INSERT INTO tasks (household_id, project_id, project_folder_id, routine_id, asset_id, asset_maintenance_item_id, assigned_to, title, notes, priority, due_at, created_by, calendar_uid)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NULLIF($13, ''))
			RETURNING id, household_id, project_id, project_folder_id, routine_id, asset_id, asset_maintenance_item_id, assigned_to, title, notes, status, priority, due_at, completed_at, created_by, created_at, updated_at, calendar_uid
		)
		SELECT inserted.id, inserted.household_id, inserted.project_id, inserted.project_folder_id, inserted.routine_id, inserted.asset_id, inserted.asset_maintenance_item_id, inserted.assigned_to, COALESCE(assigned.name, ''), inserted.title, inserted.notes, inserted.status, inserted.priority, inserted.due_at, inserted.completed_at, inserted.created_by, inserted.created_at, inserted.updated_at, COALESCE(inserted.calendar_uid, '')
		FROM inserted
		LEFT JOIN users assigned ON assigned.id = inserted.assigned_to
	`, householdID, input.ProjectID, input.ProjectFolderID, input.RoutineID, input.AssetID, input.AssetMaintenanceItemID, input.AssignedTo, input.Title, input.Notes, input.Priority, input.DueAt, userID, input.CalendarUID).Scan(
		&t.ID, &t.HouseholdID, &t.ProjectID, &t.ProjectFolderID, &t.RoutineID, &t.AssetID, &t.AssetMaintenanceItemID, &t.AssignedTo, &t.AssignedName, &t.Title, &t.Notes, &t.Status, &t.Priority, &t.DueAt, &t.CompletedAt, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt, &t.CalendarUID,
	)
	return t, err
}

func (s *Store) GetTask(ctx context.Context, householdID, id int64) (Task, error) {
	var t Task
	err := s.db.QueryRowContext(ctx, `
		SELECT t.id, t.household_id, t.project_id, t.project_folder_id, t.routine_id, t.asset_id, t.asset_maintenance_item_id, t.assigned_to, COALESCE(assigned.name, ''), t.title, t.notes, t.status, t.priority, t.due_at, t.completed_at, t.created_by, t.created_at, t.updated_at, COALESCE(t.calendar_uid, '')
		FROM tasks t
		LEFT JOIN users assigned ON assigned.id = t.assigned_to
		WHERE t.household_id = $1 AND t.id = $2
	`, householdID, id).Scan(
		&t.ID, &t.HouseholdID, &t.ProjectID, &t.ProjectFolderID, &t.RoutineID, &t.AssetID, &t.AssetMaintenanceItemID, &t.AssignedTo, &t.AssignedName, &t.Title, &t.Notes, &t.Status, &t.Priority, &t.DueAt, &t.CompletedAt, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt, &t.CalendarUID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	return t, err
}

// FindTaskByCalendarUID returns the Household's active Task that a device
// created with the given UID, or ErrNotFound.
func (s *Store) FindTaskByCalendarUID(ctx context.Context, householdID int64, uid string) (Task, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `
		SELECT id FROM tasks
		WHERE household_id = $1 AND calendar_uid = $2 AND status <> 'archived'
	`, householdID, uid).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	if err != nil {
		return Task{}, err
	}
	return s.GetTask(ctx, householdID, id)
}

// UpdateTask replaces a Task's fields. It also returns the Task as it was
// just before the update, so callers can tell what changed (a new assignee,
// due date or status).
func (s *Store) UpdateTask(ctx context.Context, householdID, id int64, input TaskInput) (updated Task, previous Task, err error) {
	if err := s.validateTaskInput(ctx, householdID, &input); err != nil {
		return Task{}, Task{}, err
	}
	if input.Status == "" {
		input.Status = "open"
	}
	if input.Status != "open" && input.Status != "done" && input.Status != "archived" {
		return Task{}, Task{}, errors.New("status must be open, done, or archived")
	}

	var t, p Task
	err = s.db.QueryRowContext(ctx, `
		WITH previous AS (
			SELECT id, household_id, project_id, project_folder_id, routine_id, asset_id, asset_maintenance_item_id, assigned_to, title, notes, status, priority, due_at, completed_at, created_by, created_at, updated_at, calendar_uid
			FROM tasks
			WHERE household_id = $1 AND id = $2
		), updated AS (
			UPDATE tasks
			SET project_id = $3,
				project_folder_id = $4,
				routine_id = $5,
				asset_id = $6,
				asset_maintenance_item_id = $7,
				assigned_to = $8,
				title = $9,
				notes = $10,
				status = $11,
				priority = $12,
				due_at = $13,
				completed_at = CASE
					WHEN $11 = 'done' AND completed_at IS NULL THEN now()
					WHEN $11 <> 'done' THEN NULL
					ELSE completed_at
				END,
				updated_at = now()
			WHERE household_id = $1 AND id = $2
			RETURNING id, household_id, project_id, project_folder_id, routine_id, asset_id, asset_maintenance_item_id, assigned_to, title, notes, status, priority, due_at, completed_at, created_by, created_at, updated_at, calendar_uid
		)
		SELECT updated.id, updated.household_id, updated.project_id, updated.project_folder_id, updated.routine_id, updated.asset_id, updated.asset_maintenance_item_id, updated.assigned_to, COALESCE(assigned.name, ''), updated.title, updated.notes, updated.status, updated.priority, updated.due_at, updated.completed_at, updated.created_by, updated.created_at, updated.updated_at, COALESCE(updated.calendar_uid, ''),
			previous.id, previous.household_id, previous.project_id, previous.project_folder_id, previous.routine_id, previous.asset_id, previous.asset_maintenance_item_id, previous.assigned_to, COALESCE(previous_assigned.name, ''), previous.title, previous.notes, previous.status, previous.priority, previous.due_at, previous.completed_at, previous.created_by, previous.created_at, previous.updated_at, COALESCE(previous.calendar_uid, '')
		FROM updated
		JOIN previous ON previous.id = updated.id
		LEFT JOIN users assigned ON assigned.id = updated.assigned_to
		LEFT JOIN users previous_assigned ON previous_assigned.id = previous.assigned_to
	`, householdID, id, input.ProjectID, input.ProjectFolderID, input.RoutineID, input.AssetID, input.AssetMaintenanceItemID, input.AssignedTo, input.Title, input.Notes, input.Status, input.Priority, input.DueAt).Scan(
		&t.ID, &t.HouseholdID, &t.ProjectID, &t.ProjectFolderID, &t.RoutineID, &t.AssetID, &t.AssetMaintenanceItemID, &t.AssignedTo, &t.AssignedName, &t.Title, &t.Notes, &t.Status, &t.Priority, &t.DueAt, &t.CompletedAt, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt, &t.CalendarUID,
		&p.ID, &p.HouseholdID, &p.ProjectID, &p.ProjectFolderID, &p.RoutineID, &p.AssetID, &p.AssetMaintenanceItemID, &p.AssignedTo, &p.AssignedName, &p.Title, &p.Notes, &p.Status, &p.Priority, &p.DueAt, &p.CompletedAt, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt, &p.CalendarUID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, Task{}, ErrNotFound
	}
	if err != nil {
		return Task{}, Task{}, err
	}
	// Only the open→done transition advances the Task's source; saving a Task
	// that was already done must not advance it again.
	completed := t.Status == "done" && p.Status != "done"
	if t.RoutineID != nil && completed {
		if advanceErr := s.CompleteRoutine(ctx, householdID, *t.RoutineID); advanceErr != nil {
			return Task{}, Task{}, advanceErr
		}
	}
	if t.AssetMaintenanceItemID != nil && completed {
		if advanceErr := s.CompleteAssetMaintenance(ctx, householdID, *t.AssetMaintenanceItemID); advanceErr != nil {
			return Task{}, Task{}, advanceErr
		}
	}
	return t, p, nil
}

func (s *Store) ArchiveTask(ctx context.Context, householdID, id int64) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE tasks
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

func (s *Store) ReopenTask(ctx context.Context, householdID, id int64) (Task, error) {
	var t Task
	err := s.db.QueryRowContext(ctx, `
		WITH updated AS (
			UPDATE tasks
			SET status = 'open', completed_at = NULL, updated_at = now()
			WHERE household_id = $1 AND id = $2
			RETURNING id, household_id, project_id, project_folder_id, routine_id, asset_id, asset_maintenance_item_id, assigned_to, title, notes, status, priority, due_at, completed_at, created_by, created_at, updated_at, calendar_uid
		)
		SELECT updated.id, updated.household_id, updated.project_id, updated.project_folder_id, updated.routine_id, updated.asset_id, updated.asset_maintenance_item_id, updated.assigned_to, COALESCE(assigned.name, ''), updated.title, updated.notes, updated.status, updated.priority, updated.due_at, updated.completed_at, updated.created_by, updated.created_at, updated.updated_at, COALESCE(updated.calendar_uid, '')
		FROM updated
		LEFT JOIN users assigned ON assigned.id = updated.assigned_to
	`, householdID, id).Scan(
		&t.ID, &t.HouseholdID, &t.ProjectID, &t.ProjectFolderID, &t.RoutineID, &t.AssetID, &t.AssetMaintenanceItemID, &t.AssignedTo, &t.AssignedName, &t.Title, &t.Notes, &t.Status, &t.Priority, &t.DueAt, &t.CompletedAt, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt, &t.CalendarUID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	return t, err
}

func (s *Store) CompleteTask(ctx context.Context, householdID, id int64) (Task, error) {
	var t Task
	err := s.db.QueryRowContext(ctx, `
		WITH updated AS (
			UPDATE tasks
			SET status = 'done', completed_at = now(), updated_at = now()
			WHERE household_id = $1 AND id = $2
			RETURNING id, household_id, project_id, project_folder_id, routine_id, asset_id, asset_maintenance_item_id, assigned_to, title, notes, status, priority, due_at, completed_at, created_by, created_at, updated_at, calendar_uid
		)
		SELECT updated.id, updated.household_id, updated.project_id, updated.project_folder_id, updated.routine_id, updated.asset_id, updated.asset_maintenance_item_id, updated.assigned_to, COALESCE(assigned.name, ''), updated.title, updated.notes, updated.status, updated.priority, updated.due_at, updated.completed_at, updated.created_by, updated.created_at, updated.updated_at, COALESCE(updated.calendar_uid, '')
		FROM updated
		LEFT JOIN users assigned ON assigned.id = updated.assigned_to
	`, householdID, id).Scan(
		&t.ID, &t.HouseholdID, &t.ProjectID, &t.ProjectFolderID, &t.RoutineID, &t.AssetID, &t.AssetMaintenanceItemID, &t.AssignedTo, &t.AssignedName, &t.Title, &t.Notes, &t.Status, &t.Priority, &t.DueAt, &t.CompletedAt, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt, &t.CalendarUID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	if err == nil && t.RoutineID != nil {
		if advanceErr := s.CompleteRoutine(ctx, householdID, *t.RoutineID); advanceErr != nil {
			return Task{}, advanceErr
		}
	}
	if err == nil && t.AssetMaintenanceItemID != nil {
		if advanceErr := s.CompleteAssetMaintenance(ctx, householdID, *t.AssetMaintenanceItemID); advanceErr != nil {
			return Task{}, advanceErr
		}
	}
	return t, err
}

func (s *Store) validateTaskInput(ctx context.Context, householdID int64, input *TaskInput) error {
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		return errors.New("title is required")
	}
	if input.Priority == "" {
		input.Priority = "normal"
	}
	if input.ProjectID != nil {
		ok, err := s.projectInHousehold(ctx, householdID, *input.ProjectID)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("project must belong to this household")
		}
	}
	if input.ProjectFolderID != nil {
		if input.ProjectID == nil {
			return errors.New("project folder requires a project")
		}
		ok, err := s.projectFolderInProject(ctx, householdID, *input.ProjectID, *input.ProjectFolderID)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("project folder must belong to this project")
		}
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
	if input.RoutineID != nil {
		ok, err := s.routineInHousehold(ctx, householdID, *input.RoutineID)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("routine must belong to this household")
		}
	}
	if input.AssetID != nil {
		ok, err := s.assetInHousehold(ctx, householdID, *input.AssetID)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("asset must belong to this household")
		}
	}
	if input.AssetMaintenanceItemID != nil {
		ok, err := s.assetMaintenanceItemInHousehold(ctx, householdID, *input.AssetMaintenanceItemID)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("asset maintenance item must belong to this household")
		}
	}
	return nil
}

func (s *Store) projectInHousehold(ctx context.Context, householdID, projectID int64) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM projects
			WHERE household_id = $1 AND id = $2
		)
	`, householdID, projectID).Scan(&exists)
	return exists, err
}

type taskScanner interface {
	Scan(dest ...any) error
}

func scanTask(scanner taskScanner, t *Task) error {
	return scanner.Scan(&t.ID, &t.HouseholdID, &t.ProjectID, &t.ProjectFolderID, &t.RoutineID, &t.AssetID, &t.AssetMaintenanceItemID, &t.AssignedTo, &t.AssignedName, &t.Title, &t.Notes, &t.Status, &t.Priority, &t.DueAt, &t.CompletedAt, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt, &t.CalendarUID)
}

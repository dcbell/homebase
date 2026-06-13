package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

func (s *Store) ListProjectFolders(ctx context.Context, householdID, projectID int64) ([]ProjectFolder, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, household_id, project_id, title, status, sort_order, created_by, created_at, updated_at
		FROM project_folders
		WHERE household_id = $1 AND project_id = $2 AND status <> 'archived'
		ORDER BY sort_order, title
	`, householdID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	folders := []ProjectFolder{}
	for rows.Next() {
		var folder ProjectFolder
		if err := scanProjectFolder(rows, &folder); err != nil {
			return nil, err
		}
		folders = append(folders, folder)
	}

	return folders, rows.Err()
}

func (s *Store) CreateProjectFolder(ctx context.Context, householdID, userID, projectID int64, input ProjectFolderInput) (ProjectFolder, error) {
	if err := s.validateProjectFolderInput(ctx, householdID, projectID, &input); err != nil {
		return ProjectFolder{}, err
	}

	var folder ProjectFolder
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO project_folders (household_id, project_id, title, sort_order, created_by)
		VALUES (
			$1,
			$2,
			$3,
			COALESCE(NULLIF($4, 0), (SELECT COALESCE(MAX(sort_order), 0) + 1 FROM project_folders WHERE household_id = $1 AND project_id = $2)),
			$5
		)
		RETURNING id, household_id, project_id, title, status, sort_order, created_by, created_at, updated_at
	`, householdID, projectID, input.Title, input.SortOrder, userID).Scan(
		&folder.ID, &folder.HouseholdID, &folder.ProjectID, &folder.Title, &folder.Status, &folder.SortOrder, &folder.CreatedBy, &folder.CreatedAt, &folder.UpdatedAt,
	)
	return folder, err
}

func (s *Store) UpdateProjectFolder(ctx context.Context, householdID, id int64, input ProjectFolderInput) (ProjectFolder, error) {
	projectID, err := s.projectIDForFolder(ctx, householdID, id)
	if errors.Is(err, sql.ErrNoRows) {
		return ProjectFolder{}, ErrNotFound
	}
	if err != nil {
		return ProjectFolder{}, err
	}
	if err := s.validateProjectFolderInput(ctx, householdID, projectID, &input); err != nil {
		return ProjectFolder{}, err
	}

	var folder ProjectFolder
	err = s.db.QueryRowContext(ctx, `
		UPDATE project_folders
		SET title = $3,
			sort_order = COALESCE(NULLIF($4, 0), sort_order),
			updated_at = now()
		WHERE household_id = $1 AND id = $2
		RETURNING id, household_id, project_id, title, status, sort_order, created_by, created_at, updated_at
	`, householdID, id, input.Title, input.SortOrder).Scan(
		&folder.ID, &folder.HouseholdID, &folder.ProjectID, &folder.Title, &folder.Status, &folder.SortOrder, &folder.CreatedBy, &folder.CreatedAt, &folder.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ProjectFolder{}, ErrNotFound
	}
	return folder, err
}

func (s *Store) ArchiveProjectFolder(ctx context.Context, householdID, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx, `
		UPDATE project_folders
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
	if _, err := tx.ExecContext(ctx, `
		UPDATE tasks
		SET project_folder_id = NULL, updated_at = now()
		WHERE household_id = $1 AND project_folder_id = $2
	`, householdID, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) validateProjectFolderInput(ctx context.Context, householdID, projectID int64, input *ProjectFolderInput) error {
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		return errors.New("title is required")
	}
	ok, err := s.projectInHousehold(ctx, householdID, projectID)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("project must belong to this household")
	}
	return nil
}

func (s *Store) projectIDForFolder(ctx context.Context, householdID, folderID int64) (int64, error) {
	var projectID int64
	err := s.db.QueryRowContext(ctx, `
		SELECT project_id
		FROM project_folders
		WHERE household_id = $1 AND id = $2 AND status <> 'archived'
	`, householdID, folderID).Scan(&projectID)
	return projectID, err
}

func (s *Store) projectFolderInProject(ctx context.Context, householdID, projectID, folderID int64) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM project_folders
			WHERE household_id = $1
				AND project_id = $2
				AND id = $3
				AND status <> 'archived'
		)
	`, householdID, projectID, folderID).Scan(&exists)
	return exists, err
}

type projectFolderScanner interface {
	Scan(dest ...any) error
}

func scanProjectFolder(scanner projectFolderScanner, folder *ProjectFolder) error {
	return scanner.Scan(&folder.ID, &folder.HouseholdID, &folder.ProjectID, &folder.Title, &folder.Status, &folder.SortOrder, &folder.CreatedBy, &folder.CreatedAt, &folder.UpdatedAt)
}

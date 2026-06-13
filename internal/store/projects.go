package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

type ProjectInput struct {
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	Priority    string     `json:"priority"`
	DueDate     *time.Time `json:"due_date"`
}

func (s *Store) ListProjects(ctx context.Context, householdID int64) ([]Project, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, household_id, title, description, status, priority, due_date, created_by, created_at, updated_at
		FROM projects
		WHERE household_id = $1 AND status <> 'archived'
		ORDER BY
			CASE priority WHEN 'high' THEN 1 WHEN 'normal' THEN 2 ELSE 3 END,
			COALESCE(due_date, '9999-12-31'::date),
			created_at DESC
	`, householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	projects := []Project{}
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.HouseholdID, &p.Title, &p.Description, &p.Status, &p.Priority, &p.DueDate, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}

	return projects, rows.Err()
}

func (s *Store) CreateProject(ctx context.Context, householdID, userID int64, input ProjectInput) (Project, error) {
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		return Project{}, errors.New("title is required")
	}
	if input.Status == "" {
		input.Status = "active"
	}
	if input.Priority == "" {
		input.Priority = "normal"
	}

	var p Project
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO projects (household_id, title, description, status, priority, due_date, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, household_id, title, description, status, priority, due_date, created_by, created_at, updated_at
	`, householdID, input.Title, input.Description, input.Status, input.Priority, input.DueDate, userID).Scan(
		&p.ID, &p.HouseholdID, &p.Title, &p.Description, &p.Status, &p.Priority, &p.DueDate, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt,
	)
	return p, err
}

func (s *Store) GetProject(ctx context.Context, householdID, id int64) (Project, error) {
	var p Project
	err := s.db.QueryRowContext(ctx, `
		SELECT id, household_id, title, description, status, priority, due_date, created_by, created_at, updated_at
		FROM projects
		WHERE household_id = $1 AND id = $2
	`, householdID, id).Scan(&p.ID, &p.HouseholdID, &p.Title, &p.Description, &p.Status, &p.Priority, &p.DueDate, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Project{}, ErrNotFound
	}
	return p, err
}

func (s *Store) UpdateProject(ctx context.Context, householdID, id int64, input ProjectInput) (Project, error) {
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		return Project{}, errors.New("title is required")
	}
	if input.Status == "" {
		input.Status = "active"
	}
	if input.Priority == "" {
		input.Priority = "normal"
	}

	var p Project
	err := s.db.QueryRowContext(ctx, `
		UPDATE projects
		SET title = $3,
			description = $4,
			status = $5,
			priority = $6,
			due_date = $7,
			updated_at = now()
		WHERE household_id = $1 AND id = $2
		RETURNING id, household_id, title, description, status, priority, due_date, created_by, created_at, updated_at
	`, householdID, id, input.Title, input.Description, input.Status, input.Priority, input.DueDate).Scan(
		&p.ID, &p.HouseholdID, &p.Title, &p.Description, &p.Status, &p.Priority, &p.DueDate, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Project{}, ErrNotFound
	}
	return p, err
}

func (s *Store) ArchiveProject(ctx context.Context, householdID, id int64) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE projects
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

package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

type EventInput struct {
	Title       string    `json:"title"`
	Description string    `json:"description"`
	StartsAt    time.Time `json:"starts_at"`
	EndsAt      time.Time `json:"ends_at"`
	Location    string    `json:"location"`
}

func (s *Store) ListEvents(ctx context.Context, householdID int64) ([]Event, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, household_id, title, description, starts_at, ends_at, location, source, external_id, sync_status, created_by, created_at, updated_at
		FROM events
		WHERE household_id = $1
		  AND starts_at >= now() - interval '1 day'
		ORDER BY starts_at
		LIMIT 50
	`, householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []Event{}
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.HouseholdID, &e.Title, &e.Description, &e.StartsAt, &e.EndsAt, &e.Location, &e.Source, &e.ExternalID, &e.SyncStatus, &e.CreatedBy, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}

	return events, rows.Err()
}

func (s *Store) CreateEvent(ctx context.Context, householdID, userID int64, input EventInput) (Event, error) {
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		return Event{}, errors.New("title is required")
	}
	if input.StartsAt.IsZero() || input.EndsAt.IsZero() || !input.EndsAt.After(input.StartsAt) {
		return Event{}, errors.New("valid starts_at and ends_at are required")
	}

	var e Event
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO events (household_id, title, description, starts_at, ends_at, location, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, household_id, title, description, starts_at, ends_at, location, source, external_id, sync_status, created_by, created_at, updated_at
	`, householdID, input.Title, input.Description, input.StartsAt, input.EndsAt, input.Location, userID).Scan(
		&e.ID, &e.HouseholdID, &e.Title, &e.Description, &e.StartsAt, &e.EndsAt, &e.Location, &e.Source, &e.ExternalID, &e.SyncStatus, &e.CreatedBy, &e.CreatedAt, &e.UpdatedAt,
	)
	return e, err
}

func (s *Store) GetEvent(ctx context.Context, householdID, id int64) (Event, error) {
	var e Event
	err := s.db.QueryRowContext(ctx, `
		SELECT id, household_id, title, description, starts_at, ends_at, location, source, external_id, sync_status, created_by, created_at, updated_at
		FROM events
		WHERE household_id = $1 AND id = $2
	`, householdID, id).Scan(&e.ID, &e.HouseholdID, &e.Title, &e.Description, &e.StartsAt, &e.EndsAt, &e.Location, &e.Source, &e.ExternalID, &e.SyncStatus, &e.CreatedBy, &e.CreatedAt, &e.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Event{}, ErrNotFound
	}
	return e, err
}

func (s *Store) UpdateEvent(ctx context.Context, householdID, id int64, input EventInput) (Event, error) {
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		return Event{}, errors.New("title is required")
	}
	if input.StartsAt.IsZero() || input.EndsAt.IsZero() || !input.EndsAt.After(input.StartsAt) {
		return Event{}, errors.New("valid starts_at and ends_at are required")
	}

	var e Event
	err := s.db.QueryRowContext(ctx, `
		UPDATE events
		SET title = $3,
			description = $4,
			starts_at = $5,
			ends_at = $6,
			location = $7,
			sync_status = 'local',
			updated_at = now()
		WHERE household_id = $1 AND id = $2
		RETURNING id, household_id, title, description, starts_at, ends_at, location, source, external_id, sync_status, created_by, created_at, updated_at
	`, householdID, id, input.Title, input.Description, input.StartsAt, input.EndsAt, input.Location).Scan(
		&e.ID, &e.HouseholdID, &e.Title, &e.Description, &e.StartsAt, &e.EndsAt, &e.Location, &e.Source, &e.ExternalID, &e.SyncStatus, &e.CreatedBy, &e.CreatedAt, &e.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Event{}, ErrNotFound
	}
	return e, err
}

func (s *Store) DeleteEvent(ctx context.Context, householdID, id int64) error {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM events
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

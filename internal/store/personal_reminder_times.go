package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// PersonalReminderTimes are a Member's own Reminder Times on one Task or
// Appointment, as the API shows them to that Member.
type PersonalReminderTimes struct {
	// UsingShared is true when the Member has none of their own, so the
	// subject's shared Reminder Times apply to them.
	UsingShared bool `json:"using_shared"`
	// ReminderTimes are the Member's own, and empty when UsingShared. An
	// empty list when not UsingShared means no Reminders.
	ReminderTimes []ReminderTime `json:"reminder_times"`
}

// PersonalReminderTimesOf returns a Member's Personal Reminder Times on a
// subject, or UsingShared when they have none.
func (s *Store) PersonalReminderTimesOf(ctx context.Context, subject ReminderSubject, memberID int64) (PersonalReminderTimes, error) {
	var encoded []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT reminder_times FROM personal_reminder_times
		WHERE household_id = $1 AND subject_type = $2 AND subject_id = $3 AND member_id = $4
	`, subject.HouseholdID, subject.Type, subject.ID, memberID).Scan(&encoded)
	if errors.Is(err, sql.ErrNoRows) {
		return PersonalReminderTimes{UsingShared: true, ReminderTimes: []ReminderTime{}}, nil
	}
	if err != nil {
		return PersonalReminderTimes{}, err
	}
	times := []ReminderTime{}
	if err := json.Unmarshal(encoded, &times); err != nil {
		return PersonalReminderTimes{}, err
	}
	return PersonalReminderTimes{ReminderTimes: times}, nil
}

// ListPersonalReminderTimes returns every Member's Personal Reminder Times on
// a subject, by Member id. Members missing from it use the shared ones.
func (s *Store) ListPersonalReminderTimes(ctx context.Context, subject ReminderSubject) (map[int64][]ReminderTime, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT member_id, reminder_times FROM personal_reminder_times
		WHERE household_id = $1 AND subject_type = $2 AND subject_id = $3
	`, subject.HouseholdID, subject.Type, subject.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	personal := map[int64][]ReminderTime{}
	for rows.Next() {
		var memberID int64
		var encoded []byte
		if err := rows.Scan(&memberID, &encoded); err != nil {
			return nil, err
		}
		times := []ReminderTime{}
		if err := json.Unmarshal(encoded, &times); err != nil {
			return nil, err
		}
		personal[memberID] = times
	}
	return personal, rows.Err()
}

// SetTaskPersonalReminderTimes sets a Member's Personal Reminder Times on a
// Task, replacing any they had; an empty list means no Reminders for them.
// Only the Task's assignee may: anyone else gets ErrForbidden. The Reminder
// Times must fit the Task's kind of due date, as its own do.
func (s *Store) SetTaskPersonalReminderTimes(ctx context.Context, householdID, taskID, memberID int64, times []ReminderTime) error {
	if times == nil {
		times = []ReminderTime{}
	}
	return s.changeTaskPersonalReminderTimes(ctx, householdID, taskID, memberID, func(tx *sql.Tx, due *time.Time) error {
		if err := ValidateReminderTimes(times, SubjectTask, taskDateOnly(due)); err != nil {
			return err
		}
		encoded, err := json.Marshal(times)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO personal_reminder_times (household_id, subject_type, subject_id, member_id, reminder_times)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (subject_type, subject_id, member_id)
			DO UPDATE SET reminder_times = EXCLUDED.reminder_times, updated_at = now()
		`, householdID, SubjectTask, taskID, memberID, string(encoded))
		return err
	})
}

// RemoveTaskPersonalReminderTimes removes a Member's Personal Reminder Times
// on a Task, so its shared Reminder Times apply to them again. Only the
// Task's assignee may: anyone else gets ErrForbidden.
func (s *Store) RemoveTaskPersonalReminderTimes(ctx context.Context, householdID, taskID, memberID int64) error {
	return s.changeTaskPersonalReminderTimes(ctx, householdID, taskID, memberID, func(tx *sql.Tx, _ *time.Time) error {
		_, err := tx.ExecContext(ctx, `
			DELETE FROM personal_reminder_times
			WHERE household_id = $1 AND subject_type = $2 AND subject_id = $3 AND member_id = $4
		`, householdID, SubjectTask, taskID, memberID)
		return err
	})
}

// changeTaskPersonalReminderTimes runs change for the Task's assignee with
// the Task locked, so a concurrent reassignment or due date change, which
// drops Personal Reminder Times, can't leave them behind for someone else.
func (s *Store) changeTaskPersonalReminderTimes(ctx context.Context, householdID, taskID, memberID int64, change func(tx *sql.Tx, due *time.Time) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var assignedTo *int64
	var due *time.Time
	err = tx.QueryRowContext(ctx, `
		SELECT assigned_to, due_at FROM tasks WHERE household_id = $1 AND id = $2 FOR UPDATE
	`, householdID, taskID).Scan(&assignedTo, &due)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if assignedTo == nil || *assignedTo != memberID {
		return ErrForbidden
	}
	if err := change(tx, due); err != nil {
		return err
	}
	return tx.Commit()
}

// dropTaskPersonalReminderTimes deletes the Personal Reminder Times on a Task
// that no longer apply after an update: every Member's when it switched
// between a timed and a date-only due date, as they no longer fit, and
// otherwise those of anyone but its assignee, who alone may have them.
func dropTaskPersonalReminderTimes(ctx context.Context, tx *sql.Tx, householdID, taskID int64, assignedTo *int64, kindSwitched bool) error {
	_, err := tx.ExecContext(ctx, `
		DELETE FROM personal_reminder_times
		WHERE household_id = $1 AND subject_type = $2 AND subject_id = $3
			AND ($4 OR member_id IS DISTINCT FROM $5)
	`, householdID, SubjectTask, taskID, kindSwitched, assignedTo)
	return err
}

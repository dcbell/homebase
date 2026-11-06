package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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

// SetPersonalReminderTimes sets a Member's Personal Reminder Times on a Task
// or Appointment, replacing any they had; an empty list means no Reminders
// for them. Only a Task's assignee may, or a Member an Appointment is for:
// anyone else gets ErrForbidden. The Reminder Times must fit the subject, as
// its own do.
func (s *Store) SetPersonalReminderTimes(ctx context.Context, subject ReminderSubject, memberID int64, times []ReminderTime) error {
	if times == nil {
		times = []ReminderTime{}
	}
	return s.changePersonalReminderTimes(ctx, subject, memberID, func(tx *sql.Tx, validate func([]ReminderTime) error) error {
		if err := validate(times); err != nil {
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
		`, subject.HouseholdID, subject.Type, subject.ID, memberID, string(encoded))
		return err
	})
}

// RemovePersonalReminderTimes removes a Member's Personal Reminder Times on
// a Task or Appointment, so its shared Reminder Times apply to them again.
// Only a Member who may set them may: anyone else gets ErrForbidden.
func (s *Store) RemovePersonalReminderTimes(ctx context.Context, subject ReminderSubject, memberID int64) error {
	return s.changePersonalReminderTimes(ctx, subject, memberID, func(tx *sql.Tx, _ func([]ReminderTime) error) error {
		_, err := tx.ExecContext(ctx, `
			DELETE FROM personal_reminder_times
			WHERE household_id = $1 AND subject_type = $2 AND subject_id = $3 AND member_id = $4
		`, subject.HouseholdID, subject.Type, subject.ID, memberID)
		return err
	})
}

// changePersonalReminderTimes runs change once the Member is authorized,
// with the subject locked, so a concurrent change that drops Personal
// Reminder Times, such as a reassignment, can't leave them behind for
// someone who may no longer have them. change gets the subject's validation
// for Reminder Times.
func (s *Store) changePersonalReminderTimes(ctx context.Context, subject ReminderSubject, memberID int64, change func(tx *sql.Tx, validate func([]ReminderTime) error) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var validate func([]ReminderTime) error
	switch subject.Type {
	case SubjectTask:
		validate, err = authorizeTaskPersonalReminderTimes(ctx, tx, subject, memberID)
	case SubjectAppointment:
		validate, err = authorizeAppointmentPersonalReminderTimes(ctx, tx, subject, memberID)
	default:
		err = fmt.Errorf("personal reminder times on a %q", subject.Type)
	}
	if err != nil {
		return err
	}
	if err := change(tx, validate); err != nil {
		return err
	}
	return tx.Commit()
}

// authorizeTaskPersonalReminderTimes locks a Task and checks the Member is
// its assignee, who alone may have Personal Reminder Times on it. It returns
// the validation for them: they must fit its kind of due date.
func authorizeTaskPersonalReminderTimes(ctx context.Context, tx *sql.Tx, subject ReminderSubject, memberID int64) (func([]ReminderTime) error, error) {
	var assignedTo *int64
	var due *time.Time
	err := tx.QueryRowContext(ctx, `
		SELECT assigned_to, due_at FROM tasks WHERE household_id = $1 AND id = $2 FOR UPDATE
	`, subject.HouseholdID, subject.ID).Scan(&assignedTo, &due)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if assignedTo == nil || *assignedTo != memberID {
		return nil, ErrForbidden
	}
	return func(times []ReminderTime) error {
		return ValidateReminderTimes(times, SubjectTask, taskDateOnly(due))
	}, nil
}

// authorizeAppointmentPersonalReminderTimes locks an Appointment and checks
// it is for the Member: they are an Attendee, or it has none. It returns the
// validation for their Personal Reminder Times, as for its own.
func authorizeAppointmentPersonalReminderTimes(ctx context.Context, tx *sql.Tx, subject ReminderSubject, memberID int64) (func([]ReminderTime) error, error) {
	appointment, err := getEvent(ctx, tx, subject.HouseholdID, subject.ID, true)
	if err != nil {
		return nil, err
	}
	if !AppointmentIsFor(appointment, memberID) {
		return nil, ErrForbidden
	}
	return func(times []ReminderTime) error {
		return ValidateReminderTimes(times, SubjectAppointment, false)
	}, nil
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

// AppointmentIsFor reports whether an Appointment is for a Member of its
// Household: they are an Attendee, or it has none.
func AppointmentIsFor(appointment Event, memberID int64) bool {
	if len(appointment.AttendeeIDs) == 0 {
		return true
	}
	for _, id := range appointment.AttendeeIDs {
		if id == memberID {
			return true
		}
	}
	return false
}

// dropAppointmentPersonalReminderTimes deletes the Personal Reminder Times on
// an Appointment of anyone it is no longer for after its Attendees changed.
// With no Attendees it is for every Member, so all are kept.
func dropAppointmentPersonalReminderTimes(ctx context.Context, tx *sql.Tx, householdID, eventID int64) error {
	_, err := tx.ExecContext(ctx, `
		DELETE FROM personal_reminder_times p
		WHERE p.household_id = $1 AND p.subject_type = $2 AND p.subject_id = $3
			AND EXISTS (SELECT 1 FROM event_attendees ea WHERE ea.event_id = $3)
			AND NOT EXISTS (SELECT 1 FROM event_attendees ea WHERE ea.event_id = $3 AND ea.user_id = p.member_id)
	`, householdID, SubjectAppointment, eventID)
	return err
}

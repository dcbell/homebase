package store

import (
	"context"
	"database/sql"
	"errors"
)

// PushSettings says which kinds of Notification a Member gets as Push. Every
// kind is on until the Member turns it off. A kind that is off still stores
// its Notifications in the bell.
type PushSettings struct {
	Assignment          bool `json:"assignment"`
	TaskReminder        bool `json:"task_reminder"`
	AppointmentReminder bool `json:"appointment_reminder"`
}

// PushSettingsInput changes some of a Member's Push settings; nil fields keep
// their current value.
type PushSettingsInput struct {
	Assignment          *bool `json:"assignment"`
	TaskReminder        *bool `json:"task_reminder"`
	AppointmentReminder *bool `json:"appointment_reminder"`
}

// Allows reports whether Notifications of kind are pushed. Unknown kinds are
// pushed.
func (p PushSettings) Allows(kind string) bool {
	switch kind {
	case NotificationAssignment:
		return p.Assignment
	case NotificationTaskReminder:
		return p.TaskReminder
	case NotificationAppointmentReminder:
		return p.AppointmentReminder
	default:
		return true
	}
}

// PushSettings returns a Member's Push settings, all on if never changed.
func (s *Store) PushSettings(ctx context.Context, householdID, userID int64) (PushSettings, error) {
	settings := PushSettings{Assignment: true, TaskReminder: true, AppointmentReminder: true}
	err := s.db.QueryRowContext(ctx, `
		SELECT assignment, task_reminder, appointment_reminder
		FROM push_settings
		WHERE household_id = $1 AND user_id = $2
	`, householdID, userID).Scan(&settings.Assignment, &settings.TaskReminder, &settings.AppointmentReminder)
	if errors.Is(err, sql.ErrNoRows) {
		return settings, nil
	}
	return settings, err
}

// UpdatePushSettings changes the given kinds of a Member's Push settings and
// returns the result.
func (s *Store) UpdatePushSettings(ctx context.Context, householdID, userID int64, input PushSettingsInput) (PushSettings, error) {
	var settings PushSettings
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO push_settings (household_id, user_id, assignment, task_reminder, appointment_reminder)
		VALUES ($1, $2, COALESCE($3, TRUE), COALESCE($4, TRUE), COALESCE($5, TRUE))
		ON CONFLICT (household_id, user_id) DO UPDATE
		SET assignment = COALESCE($3, push_settings.assignment),
		    task_reminder = COALESCE($4, push_settings.task_reminder),
		    appointment_reminder = COALESCE($5, push_settings.appointment_reminder)
		RETURNING assignment, task_reminder, appointment_reminder
	`, householdID, userID, input.Assignment, input.TaskReminder, input.AppointmentReminder).Scan(
		&settings.Assignment, &settings.TaskReminder, &settings.AppointmentReminder)
	return settings, err
}

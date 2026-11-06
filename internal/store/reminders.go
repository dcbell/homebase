package store

import (
	"context"
	"database/sql"
	"time"
)

// Reminder stages. A Reminder's stage is worked out from its Reminder Time;
// ReminderHourBefore remains only on Notifications sent before Reminder Times
// replaced the fixed stages.
const (
	ReminderDueSoon    = "due_soon"
	ReminderHourBefore = "hour_before"
	ReminderDueNow     = "due_now"
	ReminderOverdue    = "overdue"
)

// ReminderSubject is the thing Reminders are about, such as one Task or one
// Appointment.
type ReminderSubject struct {
	HouseholdID int64
	Type        string
	ID          int64
}

// ReminderInput is one Reminder the subject should have: who receives it, at
// which Reminder Time, and for which due time. It is sent at
// ReminderTime.SendAt(DueAt).
type ReminderInput struct {
	RecipientID  int64
	ReminderTime ReminderTime
	DueAt        time.Time
}

// ScheduledReminder is a Reminder claimed by the runner for sending.
type ScheduledReminder struct {
	ID           int64
	HouseholdID  int64
	RecipientID  int64
	SubjectType  string
	SubjectID    int64
	ReminderTime ReminderTime
	DueAt        time.Time
	SendAt       time.Time
}

type reminderKey struct {
	recipientID  int64
	reminderTime ReminderTime
	dueAt        int64
}

func keyOf(recipientID int64, reminderTime ReminderTime, dueAt time.Time) reminderKey {
	return reminderKey{recipientID: recipientID, reminderTime: reminderTime, dueAt: dueAt.UnixMicro()}
}

// reminderTimeColumns are a Reminder Time as the offset_minutes, offset_days
// and at_time columns store it: the timed form sets only offset_minutes, the
// date-only form only offset_days and at_time.
func reminderTimeColumns(r ReminderTime) (minutes, days sql.NullInt64, at sql.NullString) {
	if r.IsDateOnly() {
		return minutes, sql.NullInt64{Int64: int64(r.Days), Valid: true}, sql.NullString{String: r.At, Valid: true}
	}
	return sql.NullInt64{Int64: int64(r.Minutes), Valid: true}, days, at
}

// reminderTimeFromColumns reverses reminderTimeColumns. at must be selected
// as HH:MM.
func reminderTimeFromColumns(minutes, days sql.NullInt64, at sql.NullString) ReminderTime {
	if at.Valid {
		return ReminderTime{Days: int(days.Int64), At: at.String}
	}
	return ReminderTime{Minutes: int(minutes.Int64)}
}

// ScheduleReminders makes want the complete set of Reminders for subject.
//
// Pending Reminders not in want are cancelled. Reminders in want are
// scheduled only when their send time is after now, so a Reminder whose time
// has already passed is skipped rather than sent late; a previously
// cancelled one comes back when it is still ahead. A Reminder already sent is
// never sent again, and a pending one that is still wanted stays pending even
// if its time has just passed. An empty want cancels every pending Reminder.
func (s *Store) ScheduleReminders(ctx context.Context, subject ReminderSubject, want []ReminderInput, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	wanted := map[reminderKey]bool{}
	for _, r := range want {
		wanted[keyOf(r.RecipientID, r.ReminderTime, r.DueAt)] = true
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT id, recipient_id, offset_minutes, offset_days, to_char(at_time, 'HH24:MI'), due_at
		FROM scheduled_reminders
		WHERE subject_type = $1 AND subject_id = $2 AND sent_at IS NULL AND cancelled_at IS NULL
		FOR UPDATE
	`, subject.Type, subject.ID)
	if err != nil {
		return err
	}
	var cancel []int64
	for rows.Next() {
		var id, recipientID int64
		var minutes, days sql.NullInt64
		var at sql.NullString
		var dueAt time.Time
		if err := rows.Scan(&id, &recipientID, &minutes, &days, &at, &dueAt); err != nil {
			_ = rows.Close()
			return err
		}
		if !wanted[keyOf(recipientID, reminderTimeFromColumns(minutes, days, at), dueAt)] {
			cancel = append(cancel, id)
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, id := range cancel {
		if _, err := tx.ExecContext(ctx, `UPDATE scheduled_reminders SET cancelled_at = $2 WHERE id = $1`, id, now); err != nil {
			return err
		}
	}

	for _, r := range want {
		sendAt := r.ReminderTime.SendAt(r.DueAt)
		if !sendAt.After(now) {
			continue
		}
		minutes, days, at := reminderTimeColumns(r.ReminderTime)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO scheduled_reminders (household_id, recipient_id, subject_type, subject_id, stage, offset_minutes, offset_days, at_time, due_at, send_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8::time, $9, $10)
			ON CONFLICT (recipient_id, subject_type, subject_id, offset_minutes, offset_days, at_time, due_at)
			DO UPDATE SET send_at = EXCLUDED.send_at, stage = EXCLUDED.stage, cancelled_at = NULL, household_id = EXCLUDED.household_id
			WHERE scheduled_reminders.sent_at IS NULL
		`, subject.HouseholdID, r.RecipientID, subject.Type, subject.ID, r.ReminderTime.Stage(r.DueAt), minutes, days, at, r.DueAt, sendAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ReleaseReminders returns claimed Reminders to pending, so the next
// ClaimDueReminders picks them up again. It is for Reminders whose
// Notification could not be created.
func (s *Store) ReleaseReminders(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE scheduled_reminders
		SET sent_at = NULL
		WHERE id = ANY($1) AND sent_at IS NOT NULL
	`, ids)
	return err
}

// ClaimDueReminders atomically marks up to limit pending Reminders due as of
// asOf as sent and returns them. Rows locked by a concurrent claim are
// skipped, so two runners never claim the same Reminder.
func (s *Store) ClaimDueReminders(ctx context.Context, asOf time.Time, limit int) ([]ScheduledReminder, error) {
	rows, err := s.db.QueryContext(ctx, `
		UPDATE scheduled_reminders
		SET sent_at = $1
		WHERE id IN (
			SELECT id FROM scheduled_reminders
			WHERE send_at <= $1 AND sent_at IS NULL AND cancelled_at IS NULL
			ORDER BY send_at, id
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, household_id, recipient_id, subject_type, subject_id,
			offset_minutes, offset_days, to_char(at_time, 'HH24:MI'), due_at, send_at
	`, asOf, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	claimed := []ScheduledReminder{}
	for rows.Next() {
		var r ScheduledReminder
		var minutes, days sql.NullInt64
		var at sql.NullString
		if err := rows.Scan(&r.ID, &r.HouseholdID, &r.RecipientID, &r.SubjectType, &r.SubjectID, &minutes, &days, &at, &r.DueAt, &r.SendAt); err != nil {
			return nil, err
		}
		r.ReminderTime = reminderTimeFromColumns(minutes, days, at)
		claimed = append(claimed, r)
	}
	return claimed, rows.Err()
}

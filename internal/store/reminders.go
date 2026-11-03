package store

import (
	"context"
	"time"
)

// Reminder stages.
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
// which stage, for which due time, and when to send it.
type ReminderInput struct {
	RecipientID int64
	Stage       string
	DueAt       time.Time
	SendAt      time.Time
}

// ScheduledReminder is a Reminder claimed by the runner for sending.
type ScheduledReminder struct {
	ID          int64
	HouseholdID int64
	RecipientID int64
	SubjectType string
	SubjectID   int64
	Stage       string
	DueAt       time.Time
	SendAt      time.Time
}

type reminderKey struct {
	recipientID int64
	stage       string
	dueAt       int64
}

func keyOf(recipientID int64, stage string, dueAt time.Time) reminderKey {
	return reminderKey{recipientID: recipientID, stage: stage, dueAt: dueAt.UnixMicro()}
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
		wanted[keyOf(r.RecipientID, r.Stage, r.DueAt)] = true
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT id, recipient_id, stage, due_at
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
		var stage string
		var dueAt time.Time
		if err := rows.Scan(&id, &recipientID, &stage, &dueAt); err != nil {
			_ = rows.Close()
			return err
		}
		if !wanted[keyOf(recipientID, stage, dueAt)] {
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
		if !r.SendAt.After(now) {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO scheduled_reminders (household_id, recipient_id, subject_type, subject_id, stage, due_at, send_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (recipient_id, subject_type, subject_id, stage, due_at)
			DO UPDATE SET send_at = EXCLUDED.send_at, cancelled_at = NULL, household_id = EXCLUDED.household_id
			WHERE scheduled_reminders.sent_at IS NULL
		`, subject.HouseholdID, r.RecipientID, subject.Type, subject.ID, r.Stage, r.DueAt, r.SendAt); err != nil {
			return err
		}
	}
	return tx.Commit()
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
		RETURNING id, household_id, recipient_id, subject_type, subject_id, stage, due_at, send_at
	`, asOf, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	claimed := []ScheduledReminder{}
	for rows.Next() {
		var r ScheduledReminder
		if err := rows.Scan(&r.ID, &r.HouseholdID, &r.RecipientID, &r.SubjectType, &r.SubjectID, &r.Stage, &r.DueAt, &r.SendAt); err != nil {
			return nil, err
		}
		claimed = append(claimed, r)
	}
	return claimed, rows.Err()
}

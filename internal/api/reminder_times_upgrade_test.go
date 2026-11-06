package api_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"testing"
	"time"

	"homebase/internal/apitest"
	"homebase/internal/store"
	"homebase/internal/store/storetest"
)

// insertID runs an INSERT ... RETURNING id against a schema not yet fully
// migrated, where the store's own methods can't be relied on.
func insertID(t *testing.T, db *sql.DB, query string, args ...any) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRowContext(context.Background(), query, args...).Scan(&id); err != nil {
		t.Fatalf("seed before upgrade: %v", err)
	}
	return id
}

// scheduleLegacyReminders inserts pending Reminders the way they were
// scheduled before Reminder Times: one row per fixed stage.
func scheduleLegacyReminders(t *testing.T, db *sql.DB, householdID, recipientID int64, subjectType string, subjectID int64, due time.Time, sendAt map[string]time.Time) {
	t.Helper()
	for _, stage := range []string{"due_soon", "hour_before", "due_now", "overdue"} {
		at, ok := sendAt[stage]
		if !ok {
			continue
		}
		if _, err := db.ExecContext(context.Background(), `
			INSERT INTO scheduled_reminders (household_id, recipient_id, subject_type, subject_id, stage, due_at, send_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, householdID, recipientID, subjectType, subjectID, stage, due, at); err != nil {
			t.Fatalf("schedule legacy %s Reminder: %v", stage, err)
		}
	}
}

func TestTasksFromBeforeReminderTimesGetTheDefaults(t *testing.T) {
	t.Parallel()
	st, db := storetest.OpenBefore(t, "027_task_reminder_times.sql")

	householdID := insertID(t, db, `INSERT INTO households (name) VALUES ('Home') RETURNING id`)
	memberID := insertID(t, db, `INSERT INTO users (email, name) VALUES ('member@example.test', 'Member') RETURNING id`)
	if _, err := db.Exec(`INSERT INTO household_members (household_id, user_id, role) VALUES ($1, $2, 'owner')`, householdID, memberID); err != nil {
		t.Fatalf("seed Member: %v", err)
	}
	insertTask := `INSERT INTO tasks (household_id, assigned_to, title, due_at, created_by) VALUES ($1, $2, $3, $4, $2) RETURNING id`
	timed := insertID(t, db, insertTask, householdID, memberID, "Pay the water bill", at(17, 18, 0))
	dated := insertID(t, db, insertTask, householdID, memberID, "Renew the passport", localAt(20, 0, 0))

	if err := storetest.Migrate(t, st); err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	h := apitest.NewWithStore(t, st)
	member := h.SignIn(store.User{ID: memberID, Name: "Member"}, householdID)

	if got := getTask(t, member, timed); fmt.Sprint(got.ReminderTimes) != fmt.Sprint([]store.ReminderTime{{Minutes: -24 * 60}, {Minutes: 0}, {Days: 1, At: "08:00"}}) {
		t.Errorf("timed Task Reminder Times = %v, want the timed defaults", got.ReminderTimes)
	}
	if got := getTask(t, member, dated); fmt.Sprint(got.ReminderTimes) != fmt.Sprint([]store.ReminderTime{{Days: -1, At: "08:00"}, {Days: 0, At: "08:00"}, {Days: 1, At: "08:00"}}) {
		t.Errorf("date-only Task Reminder Times = %v, want the date-only defaults", got.ReminderTimes)
	}

	// Saving each, as anything that touches a Task does, schedules today's
	// Reminders.
	for _, id := range []int64{timed, dated} {
		task := getTask(t, member, id)
		updateTask(t, member, task, map[string]any{"notes": "Upgraded."})
	}
	sendRemindersAt(t, h, at(28, 8, 0))
	expectTaskReminders(t, member,
		"due_soon: Task due in 24 hours", "due_now: Task due now", "overdue: Task overdue by 1 day",
		"due_soon: Task due in 1 day", "due_now: Task due today", "overdue: Task overdue by 1 day")
}

func TestRemindersPendingAtUpgradeAreSentOnceAtTheirOriginalTimes(t *testing.T) {
	t.Parallel()
	st, db := storetest.OpenBefore(t, "026_scheduled_reminder_times.sql")

	householdID := insertID(t, db, `INSERT INTO households (name) VALUES ('Home') RETURNING id`)
	ownerID := insertID(t, db, `INSERT INTO users (email, name) VALUES ('owner@example.test', 'Owner') RETURNING id`)
	memberID := insertID(t, db, `INSERT INTO users (email, name) VALUES ('member@example.test', 'Member') RETURNING id`)
	if _, err := db.Exec(`INSERT INTO household_members (household_id, user_id, role) VALUES ($1, $2, 'owner'), ($1, $3, 'member')`, householdID, ownerID, memberID); err != nil {
		t.Fatalf("seed Members: %v", err)
	}
	insertTask := `INSERT INTO tasks (household_id, assigned_to, title, due_at, created_by) VALUES ($1, $2, $3, $4, $5) RETURNING id`
	timedDue, dateDue, starts := at(17, 18, 0), localAt(17, 0, 0), at(17, 9, 0)
	timed := insertID(t, db, insertTask, householdID, memberID, "Pay the water bill", timedDue, ownerID)
	dated := insertID(t, db, insertTask, householdID, memberID, "Renew the passport", dateDue, ownerID)
	appt := insertID(t, db, `INSERT INTO events (household_id, title, starts_at, ends_at, created_by) VALUES ($1, 'Dentist', $2, $3, $4) RETURNING id`,
		householdID, starts, starts.Add(time.Hour), ownerID)
	if _, err := db.Exec(`INSERT INTO event_attendees (event_id, user_id) VALUES ($1, $2)`, appt, memberID); err != nil {
		t.Fatalf("seed Attendee: %v", err)
	}
	scheduleLegacyReminders(t, db, householdID, memberID, "task", timed, timedDue, map[string]time.Time{
		"due_soon": at(16, 18, 0), "due_now": at(17, 18, 0), "overdue": localAt(18, 8, 0),
	})
	scheduleLegacyReminders(t, db, householdID, memberID, "task", dated, dateDue, map[string]time.Time{
		"due_soon": localAt(16, 8, 0), "due_now": localAt(17, 8, 0), "overdue": localAt(18, 8, 0),
	})
	scheduleLegacyReminders(t, db, householdID, memberID, "appointment", appt, starts, map[string]time.Time{
		"due_soon": at(16, 9, 0), "hour_before": at(17, 8, 0), "due_now": at(17, 9, 0),
	})

	if err := storetest.Migrate(t, st); err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	h := apitest.NewWithStore(t, st)
	member := h.SignIn(store.User{ID: memberID, Name: "Member"}, householdID)

	// Each subject is saved again just as one of its Reminders falls due,
	// before the runner gets to it, so the Notifier reschedules it against
	// the carried-over Reminders: a match keeps the due one, a mismatch would
	// cancel it and skip its replacement as already passed.
	resaveTask := func(id int64, title string, due time.Time) {
		member.Patch(fmt.Sprintf("/api/v1/tasks/%d", id), map[string]any{
			"title": title, "assigned_to": memberID, "due_at": due.Format(time.RFC3339),
		}).Expect(http.StatusOK)
	}
	h.Clock.Set(localAt(16, 8, 0))
	resaveTask(dated, "Renew the passport", dateDue)
	sendRemindersAt(t, h, localAt(16, 8, 0))
	sendRemindersAt(t, h, at(16, 9, 0))
	h.Clock.Set(at(16, 18, 0))
	resaveTask(timed, "Pay the water bill", timedDue)
	sendRemindersAt(t, h, at(16, 18, 0))
	h.Clock.Set(at(17, 8, 0))
	editAppointment(t, member, appt, starts, []int64{memberID})
	sendRemindersAt(t, h, at(17, 8, 0))
	sendRemindersAt(t, h, at(17, 9, 0))
	sendRemindersAt(t, h, at(17, 18, 0))
	sendRemindersAt(t, h, localAt(18, 8, 0))
	sendRemindersAt(t, h, at(25, 8, 0))

	expectReminders(t, member,
		fmt.Sprintf("due_soon %d", dated), fmt.Sprintf("due_soon %d", timed),
		fmt.Sprintf("due_now %d", dated), fmt.Sprintf("due_now %d", timed),
		fmt.Sprintf("overdue %d", timed), fmt.Sprintf("overdue %d", dated))
	expectAppointmentReminders(t, member, stages(appt, "due_soon", "due_soon", "due_now")...)
	got := reminderTitlesOf(t, member, "appointment_reminder")
	if want := []string{"Appointment in 24 hours", "Appointment in 1 hour", "Appointment starting now"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("Appointment Reminder titles = %q, want %q", got, want)
	}
}

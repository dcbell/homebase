package api_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"homebase/internal/apitest"
	"homebase/internal/store"
)

// createTaskWithReminderTimes creates a Task assigned to assignee with the
// given Reminder Times, as the API's JSON takes them.
func createTaskWithReminderTimes(t *testing.T, c *apitest.Client, assignee *apitest.Client, due time.Time, reminderTimes []map[string]any) store.Task {
	t.Helper()
	return apitest.Decode[store.Task](t, c.Post("/api/v1/tasks", map[string]any{
		"title":          "Pay the water bill",
		"due_at":         due.Format(time.RFC3339),
		"assigned_to":    assignee.User.ID,
		"reminder_times": reminderTimes,
	}).Expect(http.StatusCreated))
}

// taskRemindersOf returns the Task Reminders a Member has received, oldest
// first, as "stage: title" strings.
func taskRemindersOf(t *testing.T, c *apitest.Client) []string {
	t.Helper()
	got := []string{}
	list := notificationsOf(t, c)
	for i := len(list.Notifications) - 1; i >= 0; i-- {
		if n := list.Notifications[i]; n.Kind == "task_reminder" {
			got = append(got, n.Stage+": "+n.Title)
		}
	}
	return got
}

func expectTaskReminders(t *testing.T, c *apitest.Client, want ...string) {
	t.Helper()
	got := taskRemindersOf(t, c)
	if len(want) == 0 {
		want = []string{}
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("%s Reminders = %q, want %q", c.User.Name, got, want)
	}
}

func TestTaskWithCustomReminderTimesSendsExactlyThoseReminders(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	createTaskWithReminderTimes(t, household.Owner, household.Member, at(20, 18, 0), []map[string]any{
		{"days": -3, "at": "09:00"},
		{"minutes": -120},
		{"minutes": 30},
	})

	sendRemindersAt(t, h, at(17, 8, 59))
	expectTaskReminders(t, household.Member)

	sendRemindersAt(t, h, at(17, 9, 0))
	expectTaskReminders(t, household.Member, "due_soon: Task due in 3 days")

	sendRemindersAt(t, h, at(20, 16, 0))
	expectTaskReminders(t, household.Member, "due_soon: Task due in 3 days", "due_soon: Task due in 2 hours")

	sendRemindersAt(t, h, at(20, 18, 0))
	expectTaskReminders(t, household.Member, "due_soon: Task due in 3 days", "due_soon: Task due in 2 hours")

	sendRemindersAt(t, h, at(28, 8, 0))
	expectTaskReminders(t, household.Member,
		"due_soon: Task due in 3 days", "due_soon: Task due in 2 hours", "overdue: Task overdue by 30 minutes")
}

func TestTaskCreatedWithoutReminderTimesGetsTheDefaults(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	timed := createDueTask(t, household.Owner, household.Member, at(17, 18, 0))
	dated := createDueTask(t, household.Owner, household.Member, localAt(17, 0, 0))

	got := apitest.Decode[store.Task](t, household.Owner.Get(fmt.Sprintf("/api/v1/tasks/%d", timed.ID)).Expect(http.StatusOK))
	if want := []store.ReminderTime{{Minutes: -24 * 60}, {Minutes: 0}, {Days: 1, At: "08:00"}}; fmt.Sprint(got.ReminderTimes) != fmt.Sprint(want) {
		t.Errorf("timed Task Reminder Times = %v, want %v", got.ReminderTimes, want)
	}
	if want := []store.ReminderTime{{Days: -1, At: "08:00"}, {Days: 0, At: "08:00"}, {Days: 1, At: "08:00"}}; fmt.Sprint(dated.ReminderTimes) != fmt.Sprint(want) {
		t.Errorf("date-only Task Reminder Times = %v, want %v", dated.ReminderTimes, want)
	}

	sendRemindersAt(t, h, at(28, 8, 0))
	expectTaskReminders(t, household.Member,
		"due_soon: Task due in 1 day", "due_soon: Task due in 24 hours",
		"due_now: Task due today", "due_now: Task due now",
		"overdue: Task overdue by 1 day", "overdue: Task overdue by 1 day")
}

func TestTaskWithNoReminderTimesSendsNoReminders(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	createTaskWithReminderTimes(t, household.Owner, household.Member, at(17, 18, 0), []map[string]any{})

	sendRemindersAt(t, h, at(28, 8, 0))
	expectTaskReminders(t, household.Member)
}

func TestSeveralOverdueReminderTimesAreSentInTurnUntilTheTaskIsDone(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	task := createTaskWithReminderTimes(t, household.Owner, household.Member, localAt(17, 0, 0), []map[string]any{
		{"days": 1, "at": "08:00"},
		{"days": 3, "at": "08:00"},
		{"days": 7, "at": "08:00"},
	})

	sendRemindersAt(t, h, localAt(18, 8, 0))
	expectTaskReminders(t, household.Member, "overdue: Task overdue by 1 day")

	sendRemindersAt(t, h, localAt(20, 8, 0))
	expectTaskReminders(t, household.Member, "overdue: Task overdue by 1 day", "overdue: Task overdue by 3 days")

	household.Member.Patch(fmt.Sprintf("/api/v1/tasks/%d/complete", task.ID), nil).Expect(http.StatusOK)
	sendRemindersAt(t, h, localAt(28, 8, 0))
	expectTaskReminders(t, household.Member, "overdue: Task overdue by 1 day", "overdue: Task overdue by 3 days")
}

func TestInvalidTaskReminderTimesAreRejected(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	timedDue, dateDue := at(17, 18, 0).Format(time.RFC3339), localAt(17, 0, 0).Format(time.RFC3339)
	cases := []struct {
		name          string
		due           string
		reminderTimes []map[string]any
	}{
		{"more than five", timedDue, []map[string]any{{"minutes": -60}, {"minutes": -120}, {"minutes": -180}, {"minutes": -240}, {"minutes": -300}, {"minutes": 0}}},
		{"a repeat", timedDue, []map[string]any{{"minutes": -60}, {"minutes": -60}}},
		{"beyond 30 days", timedDue, []map[string]any{{"days": -31, "at": "08:00"}}},
		{"a time of day that isn't HH:MM", timedDue, []map[string]any{{"days": -1, "at": "8am"}}},
		{"minutes on a date-only Task", dateDue, []map[string]any{{"minutes": -60}}},
	}
	task := createDueTask(t, household.Owner, household.Member, at(17, 18, 0))
	for _, c := range cases {
		body := map[string]any{"title": "Pay the water bill", "due_at": c.due, "reminder_times": c.reminderTimes}
		if r := household.Owner.Post("/api/v1/tasks", body); r.Status != http.StatusBadRequest {
			t.Errorf("create with %s: status = %d, want %d", c.name, r.Status, http.StatusBadRequest)
		}
		if c.due != timedDue {
			continue // Switching the Task to date-only would reset its Reminder Times.
		}
		if r := household.Owner.Patch(fmt.Sprintf("/api/v1/tasks/%d", task.ID), body); r.Status != http.StatusBadRequest {
			t.Errorf("update with %s: status = %d, want %d", c.name, r.Status, http.StatusBadRequest)
		}
	}
}

// updateTask saves a Task with the given fields, as a client resending the
// whole Task does.
func updateTask(t *testing.T, c *apitest.Client, task store.Task, fields map[string]any) store.Task {
	t.Helper()
	body := map[string]any{"title": task.Title, "assigned_to": task.AssignedTo, "due_at": task.DueAt}
	for k, v := range fields {
		body[k] = v
	}
	return apitest.Decode[store.Task](t, c.Patch(fmt.Sprintf("/api/v1/tasks/%d", task.ID), body).Expect(http.StatusOK))
}

func TestEditingReminderTimesReschedulesAndSkipsThoseAlreadyPassed(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	task := createDueTask(t, household.Owner, household.Member, at(17, 18, 0))

	// Now is the 16th at 20:00: Due Soon 24 hours before has been sent.
	sendRemindersAt(t, h, at(16, 20, 0))
	expectTaskReminders(t, household.Member, "due_soon: Task due in 24 hours")

	// 2 days before has already passed, so it is skipped rather than sent
	// late; 2 hours before is still ahead, and Due Now and Overdue are gone.
	updateTask(t, household.Owner, task, map[string]any{"reminder_times": []map[string]any{{"minutes": -2 * 24 * 60}, {"minutes": -120}}})

	sendRemindersAt(t, h, at(28, 8, 0))
	expectTaskReminders(t, household.Member, "due_soon: Task due in 24 hours", "due_soon: Task due in 2 hours")
}

func TestSavingATaskWithoutReminderTimesKeepsThem(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	task := createTaskWithReminderTimes(t, household.Owner, household.Member, at(17, 18, 0), []map[string]any{{"minutes": -120}})

	saved := updateTask(t, household.Owner, task, map[string]any{"title": "Pay the gas bill", "due_at": at(18, 18, 0)})
	if want := []store.ReminderTime{{Minutes: -120}}; fmt.Sprint(saved.ReminderTimes) != fmt.Sprint(want) {
		t.Errorf("Reminder Times = %v, want %v", saved.ReminderTimes, want)
	}

	sendRemindersAt(t, h, at(28, 8, 0))
	expectTaskReminders(t, household.Member, "due_soon: Task due in 2 hours")
}

func TestSwitchingBetweenTimedAndDateOnlyResetsReminderTimesToTheNewDefaults(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	task := createTaskWithReminderTimes(t, household.Owner, household.Member, at(17, 18, 0), []map[string]any{{"minutes": -120}})

	// The old Reminder Times are sent along, as a client resending the
	// whole Task does, but no longer fit a date with no time of day.
	dated := updateTask(t, household.Owner, task, map[string]any{"due_at": localAt(20, 0, 0), "reminder_times": task.ReminderTimes})
	if want := []store.ReminderTime{{Days: -1, At: "08:00"}, {Days: 0, At: "08:00"}, {Days: 1, At: "08:00"}}; fmt.Sprint(dated.ReminderTimes) != fmt.Sprint(want) {
		t.Errorf("Reminder Times after switching to date-only = %v, want %v", dated.ReminderTimes, want)
	}
	sendRemindersAt(t, h, localAt(19, 8, 0))
	expectTaskReminders(t, household.Member, "due_soon: Task due in 1 day")

	updateTask(t, household.Owner, dated, map[string]any{"reminder_times": []map[string]any{{"days": -2, "at": "07:00"}}})
	timed := updateTask(t, household.Owner, dated, map[string]any{"due_at": at(25, 18, 0)})
	if want := []store.ReminderTime{{Minutes: -24 * 60}, {Minutes: 0}, {Days: 1, At: "08:00"}}; fmt.Sprint(timed.ReminderTimes) != fmt.Sprint(want) {
		t.Errorf("Reminder Times after switching to timed = %v, want %v", timed.ReminderTimes, want)
	}
	sendRemindersAt(t, h, at(28, 8, 0))
	expectTaskReminders(t, household.Member,
		"due_soon: Task due in 1 day", "due_soon: Task due in 24 hours", "due_now: Task due now", "overdue: Task overdue by 1 day")
}

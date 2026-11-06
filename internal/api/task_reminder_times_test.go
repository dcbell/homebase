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

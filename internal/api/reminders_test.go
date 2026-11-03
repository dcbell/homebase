package api_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"homebase/internal/apitest"
	"homebase/internal/store"
)

// sendRemindersAt moves the Clock to at and runs the Reminder runner.
func sendRemindersAt(t *testing.T, h *apitest.Harness, at time.Time) {
	t.Helper()
	h.Clock.Set(at)
	if _, err := h.Server.SendDueReminders(context.Background(), at); err != nil {
		t.Fatalf("send due Reminders: %v", err)
	}
	h.Server.WaitForPush()
}

// remindersOf returns the Task Reminders a Member has received, oldest first,
// as "stage task-id" strings.
func remindersOf(t *testing.T, c *apitest.Client) []string {
	t.Helper()
	got := []string{}
	list := notificationsOf(t, c)
	for i := len(list.Notifications) - 1; i >= 0; i-- {
		n := list.Notifications[i]
		if n.Kind == "task_reminder" {
			got = append(got, fmt.Sprintf("%s %d", n.Stage, n.SubjectID))
		}
	}
	return got
}

func expectReminders(t *testing.T, c *apitest.Client, want ...string) {
	t.Helper()
	got := remindersOf(t, c)
	if len(want) == 0 {
		want = []string{}
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("%s Reminders = %v, want %v", c.User.Name, got, want)
	}
}

func at(day, hour, minute int) time.Time {
	return time.Date(2030, time.January, day, hour, minute, 0, 0, time.UTC)
}

func createDueTask(t *testing.T, c *apitest.Client, assignee *apitest.Client, due time.Time) store.Task {
	t.Helper()
	body := map[string]any{"title": "Pay the water bill", "due_at": due.Format(time.RFC3339)}
	if assignee != nil {
		body["assigned_to"] = assignee.User.ID
	}
	return apitest.Decode[store.Task](t, c.Post("/api/v1/tasks", body).Expect(http.StatusCreated))
}

func TestAssignedTaskGetsDueSoonDueNowAndOverdueReminders(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	task := createDueTask(t, household.Owner, household.Member, at(17, 18, 0))
	dueSoon, dueNow, overdue := fmt.Sprintf("due_soon %d", task.ID), fmt.Sprintf("due_now %d", task.ID), fmt.Sprintf("overdue %d", task.ID)

	sendRemindersAt(t, h, at(16, 17, 59))
	expectReminders(t, household.Member)

	sendRemindersAt(t, h, at(16, 18, 0))
	expectReminders(t, household.Member, dueSoon)

	sendRemindersAt(t, h, at(17, 17, 59))
	expectReminders(t, household.Member, dueSoon)

	sendRemindersAt(t, h, at(17, 18, 0))
	expectReminders(t, household.Member, dueSoon, dueNow)

	sendRemindersAt(t, h, at(18, 7, 59))
	expectReminders(t, household.Member, dueSoon, dueNow)

	sendRemindersAt(t, h, at(18, 8, 0))
	expectReminders(t, household.Member, dueSoon, dueNow, overdue)

	sendRemindersAt(t, h, at(25, 8, 0))
	expectReminders(t, household.Member, dueSoon, dueNow, overdue)
	expectReminders(t, household.Owner)
}

func TestRemindersWhoseTimeHasPassedWhenScheduledAreSkipped(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	// Now is the 15th at 12:00, so Due Soon (15th 06:00) has already passed.
	task := createDueTask(t, household.Owner, household.Member, at(16, 6, 0))

	sendRemindersAt(t, h, at(15, 12, 0))
	expectReminders(t, household.Member)

	sendRemindersAt(t, h, at(16, 6, 0))
	expectReminders(t, household.Member, fmt.Sprintf("due_now %d", task.ID))
}

func TestOverdueIsSentTheFirstMorningAfterTheDueDate(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	// Due just after midnight: Overdue waits for the next day's 08:00, not
	// the same morning.
	task := createDueTask(t, household.Owner, household.Member, at(17, 0, 30))

	sendRemindersAt(t, h, at(17, 8, 0))
	expectReminders(t, household.Member, fmt.Sprintf("due_soon %d", task.ID), fmt.Sprintf("due_now %d", task.ID))

	sendRemindersAt(t, h, at(18, 8, 0))
	expectReminders(t, household.Member, fmt.Sprintf("due_soon %d", task.ID), fmt.Sprintf("due_now %d", task.ID), fmt.Sprintf("overdue %d", task.ID))
}

func TestCompletedTaskGetsNoOverdue(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	task := createDueTask(t, household.Owner, household.Member, at(15, 18, 0))

	sendRemindersAt(t, h, at(15, 18, 0))
	household.Member.Patch(fmt.Sprintf("/api/v1/tasks/%d/complete", task.ID), nil).Expect(http.StatusOK)

	sendRemindersAt(t, h, at(16, 8, 0))
	expectReminders(t, household.Member, fmt.Sprintf("due_now %d", task.ID))
}

func TestSelfAssignedTaskGetsReminders(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	task := createDueTask(t, household.Member, household.Member, at(15, 18, 0))

	sendRemindersAt(t, h, at(15, 18, 0))
	expectReminders(t, household.Member, fmt.Sprintf("due_now %d", task.ID))
}

func TestUnassignedOrUndatedTasksGetNoReminders(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	createDueTask(t, household.Owner, nil, at(16, 18, 0))
	household.Owner.Post("/api/v1/tasks", map[string]any{
		"title":       "Someday: paint the shed",
		"assigned_to": household.Member.User.ID,
	}).Expect(http.StatusCreated)

	sendRemindersAt(t, h, at(30, 8, 0))
	expectReminders(t, household.Owner)
	expectReminders(t, household.Member)
}

func TestChangingDueDateReschedulesReminders(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	task := createDueTask(t, household.Owner, household.Member, at(16, 18, 0))

	household.Owner.Patch(fmt.Sprintf("/api/v1/tasks/%d", task.ID), map[string]any{
		"title":       task.Title,
		"assigned_to": household.Member.User.ID,
		"due_at":      at(20, 9, 0).Format(time.RFC3339),
	}).Expect(http.StatusOK)

	sendRemindersAt(t, h, at(19, 8, 59))
	expectReminders(t, household.Member)

	sendRemindersAt(t, h, at(20, 9, 0))
	expectReminders(t, household.Member, fmt.Sprintf("due_soon %d", task.ID), fmt.Sprintf("due_now %d", task.ID))
}

func TestRemovingDueDateCancelsReminders(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	task := createDueTask(t, household.Owner, household.Member, at(16, 18, 0))

	household.Owner.Patch(fmt.Sprintf("/api/v1/tasks/%d", task.ID), map[string]any{
		"title":       task.Title,
		"assigned_to": household.Member.User.ID,
	}).Expect(http.StatusOK)

	sendRemindersAt(t, h, at(30, 8, 0))
	expectReminders(t, household.Member)
}

func TestReassigningTaskMovesRemindersToTheNewAssignee(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	task := createDueTask(t, household.Owner, household.Member, at(16, 18, 0))

	sendRemindersAt(t, h, at(15, 18, 0))
	expectReminders(t, household.Member, fmt.Sprintf("due_soon %d", task.ID))

	household.Member.Patch(fmt.Sprintf("/api/v1/tasks/%d", task.ID), map[string]any{
		"title":       task.Title,
		"assigned_to": household.Owner.User.ID,
		"due_at":      at(16, 18, 0).Format(time.RFC3339),
	}).Expect(http.StatusOK)

	sendRemindersAt(t, h, at(17, 8, 0))
	expectReminders(t, household.Member, fmt.Sprintf("due_soon %d", task.ID))
	expectReminders(t, household.Owner, fmt.Sprintf("due_now %d", task.ID), fmt.Sprintf("overdue %d", task.ID))
}

func TestUnassigningTaskCancelsReminders(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	task := createDueTask(t, household.Owner, household.Member, at(16, 18, 0))

	household.Owner.Patch(fmt.Sprintf("/api/v1/tasks/%d", task.ID), map[string]any{
		"title":  task.Title,
		"due_at": at(16, 18, 0).Format(time.RFC3339),
	}).Expect(http.StatusOK)

	sendRemindersAt(t, h, at(30, 8, 0))
	expectReminders(t, household.Member)
	expectReminders(t, household.Owner)
}

func TestReminderRunnerIsIdempotentAndSafeToRunConcurrently(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	var tasks []store.Task
	for range 5 {
		tasks = append(tasks, createDueTask(t, household.Owner, household.Member, at(15, 18, 0)))
	}
	asOf := at(15, 18, 0)
	h.Clock.Set(asOf)

	const runners = 4
	sent := make(chan int, runners)
	errs := make(chan error, runners)
	for range runners {
		go func() {
			n, err := h.Server.SendDueReminders(context.Background(), asOf)
			sent <- n
			errs <- err
		}()
	}
	total := 0
	for range runners {
		total += <-sent
		if err := <-errs; err != nil {
			t.Errorf("runner: %v", err)
		}
	}
	sendRemindersAt(t, h, asOf)

	if total != len(tasks) {
		t.Errorf("runners sent %d Reminders in total, want %d", total, len(tasks))
	}
	if got := remindersOf(t, household.Member); len(got) != len(tasks) {
		t.Errorf("Member got %d Reminders, want one Due Now per Task: %v", len(got), got)
	}
}

func TestReminderNotificationDescribesTheTask(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	task := createDueTask(t, household.Owner, household.Member, at(15, 18, 0))

	sendRemindersAt(t, h, at(15, 18, 0))

	list := notificationsOf(t, household.Member)
	n := list.Notifications[0]
	if n.Kind != "task_reminder" || n.Stage != "due_now" {
		t.Fatalf("latest Notification = %+v, want a Due Now Task Reminder", n)
	}
	if n.Title != "Task due now" || n.Body != "Pay the water bill" {
		t.Errorf("title, body = %q, %q; want %q, %q", n.Title, n.Body, "Task due now", "Pay the water bill")
	}
	if want := fmt.Sprintf("/tasks/%d", task.ID); n.URL != want {
		t.Errorf("url = %q, want %q", n.URL, want)
	}
	if !n.CreatedAt.Equal(at(15, 18, 0)) {
		t.Errorf("created_at = %v, want the Clock's now", n.CreatedAt)
	}
	if list.UnreadCount != 2 {
		t.Errorf("unread_count = %d, want 2 (assignment and Reminder)", list.UnreadCount)
	}
}

func TestReopeningTaskWithFutureDueDateRestoresItsReminders(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	task := createDueTask(t, household.Owner, household.Member, at(17, 18, 0))

	household.Member.Patch(fmt.Sprintf("/api/v1/tasks/%d/complete", task.ID), nil).Expect(http.StatusOK)
	sendRemindersAt(t, h, at(16, 18, 0))
	expectReminders(t, household.Member)

	household.Member.Patch(fmt.Sprintf("/api/v1/tasks/%d/reopen", task.ID), nil).Expect(http.StatusOK)
	sendRemindersAt(t, h, at(17, 18, 0))
	expectReminders(t, household.Member, fmt.Sprintf("due_now %d", task.ID))
}

func TestReopeningTaskSkipsRemindersThatPassedWhileItWasDone(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	task := createDueTask(t, household.Owner, household.Member, at(17, 18, 0))

	household.Member.Patch(fmt.Sprintf("/api/v1/tasks/%d/complete", task.ID), nil).Expect(http.StatusOK)
	h.Clock.Set(at(16, 20, 0))
	household.Member.Patch(fmt.Sprintf("/api/v1/tasks/%d/reopen", task.ID), nil).Expect(http.StatusOK)

	sendRemindersAt(t, h, at(16, 20, 0))
	expectReminders(t, household.Member)

	sendRemindersAt(t, h, at(17, 18, 0))
	expectReminders(t, household.Member, fmt.Sprintf("due_now %d", task.ID))
}

func TestArchivingTaskCancelsItsReminders(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	task := createDueTask(t, household.Owner, household.Member, at(17, 18, 0))

	household.Owner.Post(fmt.Sprintf("/api/v1/tasks/%d/archive", task.ID), nil).Expect(http.StatusOK)

	sendRemindersAt(t, h, at(30, 8, 0))
	expectReminders(t, household.Member)
}

func TestCompletingTaskThroughEditCancelsItsReminders(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	task := createDueTask(t, household.Owner, household.Member, at(17, 18, 0))

	household.Member.Patch(fmt.Sprintf("/api/v1/tasks/%d", task.ID), map[string]any{
		"title":       task.Title,
		"assigned_to": household.Member.User.ID,
		"due_at":      at(17, 18, 0).Format(time.RFC3339),
		"status":      "done",
	}).Expect(http.StatusOK)

	sendRemindersAt(t, h, at(30, 8, 0))
	expectReminders(t, household.Member)
}

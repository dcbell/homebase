package api_test

import (
	"fmt"
	"testing"
	"time"

	"homebase/internal/apitest"
)

// localAt is a time on a January 2030 day in the application timezone.
func localAt(day, hour, minute int) time.Time {
	return time.Date(2030, time.January, day, hour, minute, 0, 0, time.Local)
}

func TestTaskDueOnADateGetsItsRemindersInTheMorning(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	// Due at local midnight: a due date with no time of day.
	task := createDueTask(t, household.Owner, household.Member, localAt(17, 0, 0))
	dueSoon, dueNow, overdue := fmt.Sprintf("due_soon %d", task.ID), fmt.Sprintf("due_now %d", task.ID), fmt.Sprintf("overdue %d", task.ID)

	sendRemindersAt(t, h, localAt(16, 7, 59))
	expectReminders(t, household.Member)

	sendRemindersAt(t, h, localAt(16, 8, 0))
	expectReminders(t, household.Member, dueSoon)

	sendRemindersAt(t, h, localAt(17, 7, 59))
	expectReminders(t, household.Member, dueSoon)

	sendRemindersAt(t, h, localAt(17, 8, 0))
	expectReminders(t, household.Member, dueSoon, dueNow)

	sendRemindersAt(t, h, localAt(18, 7, 59))
	expectReminders(t, household.Member, dueSoon, dueNow)

	sendRemindersAt(t, h, localAt(18, 8, 0))
	expectReminders(t, household.Member, dueSoon, dueNow, overdue)
}

func TestTaskDueOnADateHasRemindersTitledInDays(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	createDueTask(t, household.Owner, household.Member, localAt(17, 0, 0))

	sendRemindersAt(t, h, localAt(25, 8, 0))
	got := reminderTitlesOf(t, household.Member, "task_reminder")
	want := []string{"Task due in 1 day", "Task due today", "Task overdue by 1 day"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("Reminder titles = %q, want %q", got, want)
	}
}

package api_test

import (
	"fmt"
	"net/http"
	"testing"

	"homebase/internal/apitest"
	"homebase/internal/store"
)

// setPersonalReminderTimes sets the caller's Personal Reminder Times on a
// Task, as the API's JSON takes them.
func setPersonalReminderTimes(c *apitest.Client, taskID int64, reminderTimes []map[string]any) *apitest.Response {
	return c.Do(http.MethodPut, fmt.Sprintf("/api/v1/tasks/%d/personal-reminder-times", taskID), map[string]any{
		"reminder_times": reminderTimes,
	})
}

// useSharedReminderTimes removes the caller's Personal Reminder Times on a
// Task, so its shared Reminder Times apply to them again.
func useSharedReminderTimes(c *apitest.Client, taskID int64) *apitest.Response {
	return c.Delete(fmt.Sprintf("/api/v1/tasks/%d/personal-reminder-times", taskID))
}

// expectUsingShared checks that the Task, as c reads it, shows c is using its
// shared Reminder Times.
func expectUsingShared(t *testing.T, c *apitest.Client, taskID int64) {
	t.Helper()
	got := getTask(t, c, taskID).PersonalReminderTimes
	if got == nil || !got.UsingShared || len(got.ReminderTimes) != 0 {
		t.Errorf("%s Personal Reminder Times = %+v, want using shared", c.User.Name, got)
	}
}

func TestPersonalReminderTimesReplaceTheSharedOnesForTheAssigneeOnly(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	task := createTaskWithReminderTimes(t, household.Owner, household.Member, at(20, 18, 0), []map[string]any{{"minutes": -120}})

	saved := apitest.Decode[store.Task](t, setPersonalReminderTimes(household.Member, task.ID, []map[string]any{
		{"days": -3, "at": "09:00"},
		{"minutes": 30},
	}).Expect(http.StatusOK))
	want := store.PersonalReminderTimes{ReminderTimes: []store.ReminderTime{{Days: -3, At: "09:00"}, {Minutes: 30}}}
	if saved.PersonalReminderTimes == nil || fmt.Sprint(*saved.PersonalReminderTimes) != fmt.Sprint(want) {
		t.Errorf("Personal Reminder Times after setting = %+v, want %+v", saved.PersonalReminderTimes, want)
	}
	if got := getTask(t, household.Member, task.ID).PersonalReminderTimes; got == nil || fmt.Sprint(*got) != fmt.Sprint(want) {
		t.Errorf("Personal Reminder Times read back = %+v, want %+v", got, want)
	}

	// Later changes to the shared Reminder Times leave them alone.
	updateTask(t, household.Owner, task, map[string]any{"reminder_times": []map[string]any{{"minutes": -60}, {"minutes": 0}}})

	sendRemindersAt(t, h, at(28, 8, 0))
	expectTaskReminders(t, household.Member, "due_soon: Task due in 3 days", "overdue: Task overdue by 30 minutes")
}

func TestSavingATaskOrAppointmentShowsTheCallersOwnPersonalReminderTimes(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()

	created := apitest.Decode[store.Task](t, household.Member.Post("/api/v1/tasks", map[string]any{
		"title": "Pay the water bill", "due_at": at(20, 18, 0), "assigned_to": household.Member.User.ID,
	}).Expect(http.StatusCreated))
	if got := created.PersonalReminderTimes; got == nil || !got.UsingShared {
		t.Errorf("Personal Reminder Times on the created Task = %+v, want using shared", got)
	}
	setPersonalReminderTimes(household.Member, created.ID, []map[string]any{{"minutes": -30}}).Expect(http.StatusOK)
	updated := updateTask(t, household.Member, created, map[string]any{"title": "Pay the gas bill"})
	want := store.PersonalReminderTimes{ReminderTimes: []store.ReminderTime{{Minutes: -30}}}
	if got := updated.PersonalReminderTimes; got == nil || fmt.Sprint(*got) != fmt.Sprint(want) {
		t.Errorf("Personal Reminder Times on the updated Task = %+v, want %+v", got, want)
	}
	if got := updateTask(t, household.Owner, created, map[string]any{"title": "Pay the gas bill"}).PersonalReminderTimes; got != nil {
		t.Errorf("Personal Reminder Times shown to someone else = %+v, want none", got)
	}

	appointment := apitest.Decode[store.Event](t, household.Member.Post("/api/v1/events", map[string]any{
		"title": "Vet", "starts_at": at(20, 9, 0), "ends_at": at(20, 10, 0), "attendee_ids": []int64{household.Member.User.ID},
	}).Expect(http.StatusCreated))
	if got := appointment.PersonalReminderTimes; got == nil || !got.UsingShared {
		t.Errorf("Personal Reminder Times on the created Appointment = %+v, want using shared", got)
	}
	setPersonalAppointmentReminderTimes(household.Member, appointment.ID, []map[string]any{{"minutes": -30}}).Expect(http.StatusOK)
	saved := apitest.Decode[store.Event](t, household.Member.Patch(fmt.Sprintf("/api/v1/events/%d", appointment.ID), map[string]any{
		"title": "Vet check-up", "starts_at": at(20, 9, 0), "ends_at": at(20, 10, 0),
	}).Expect(http.StatusOK))
	if got := saved.PersonalReminderTimes; got == nil || fmt.Sprint(*got) != fmt.Sprint(want) {
		t.Errorf("Personal Reminder Times on the updated Appointment = %+v, want %+v", got, want)
	}
	other := apitest.Decode[store.Event](t, household.Owner.Patch(fmt.Sprintf("/api/v1/events/%d", appointment.ID), map[string]any{
		"title": "Vet check-up", "starts_at": at(20, 9, 0), "ends_at": at(20, 10, 0),
	}).Expect(http.StatusOK))
	if other.PersonalReminderTimes != nil {
		t.Errorf("Appointment Personal Reminder Times shown to a non-Attendee = %+v, want none", other.PersonalReminderTimes)
	}
}

func TestAnEmptyPersonalListMeansNoRemindersForTheAssignee(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	task := createDueTask(t, household.Owner, household.Member, at(17, 18, 0))

	saved := apitest.Decode[store.Task](t, setPersonalReminderTimes(household.Member, task.ID, []map[string]any{}).Expect(http.StatusOK))
	if got := saved.PersonalReminderTimes; got == nil || got.UsingShared || len(got.ReminderTimes) != 0 {
		t.Errorf("Personal Reminder Times = %+v, want an empty personal list", got)
	}

	sendRemindersAt(t, h, at(28, 8, 0))
	expectTaskReminders(t, household.Member)
}

func TestRemovingPersonalReminderTimesGoesBackToTheSharedOnes(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	task := createTaskWithReminderTimes(t, household.Owner, household.Member, at(17, 18, 0), []map[string]any{{"minutes": -120}})
	expectUsingShared(t, household.Member, task.ID)

	setPersonalReminderTimes(household.Member, task.ID, []map[string]any{}).Expect(http.StatusOK)
	saved := apitest.Decode[store.Task](t, useSharedReminderTimes(household.Member, task.ID).Expect(http.StatusOK))
	if got := saved.PersonalReminderTimes; got == nil || !got.UsingShared {
		t.Errorf("Personal Reminder Times after removing = %+v, want using shared", got)
	}
	expectUsingShared(t, household.Member, task.ID)

	sendRemindersAt(t, h, at(28, 8, 0))
	expectTaskReminders(t, household.Member, "due_soon: Task due in 2 hours")
}

func TestOnlyTheAssigneeCanReadOrSetPersonalReminderTimes(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	task := createDueTask(t, household.Owner, household.Member, at(17, 18, 0))
	setPersonalReminderTimes(household.Member, task.ID, []map[string]any{{"minutes": -30}}).Expect(http.StatusOK)

	if got := getTask(t, household.Owner, task.ID).PersonalReminderTimes; got != nil {
		t.Errorf("another Member read Personal Reminder Times %+v, want none", got)
	}
	setPersonalReminderTimes(household.Owner, task.ID, []map[string]any{}).Expect(http.StatusForbidden)
	useSharedReminderTimes(household.Owner, task.ID).Expect(http.StatusForbidden)

	unassigned := apitest.Decode[store.Task](t, household.Owner.Post("/api/v1/tasks", map[string]any{
		"title": "Sweep the porch", "due_at": at(17, 18, 0),
	}).Expect(http.StatusCreated))
	if got := getTask(t, household.Owner, unassigned.ID).PersonalReminderTimes; got != nil {
		t.Errorf("Personal Reminder Times on an unassigned Task = %+v, want none", got)
	}
	setPersonalReminderTimes(household.Owner, unassigned.ID, []map[string]any{}).Expect(http.StatusForbidden)
	setPersonalReminderTimes(household.Member, 999999, []map[string]any{}).Expect(http.StatusNotFound)

	sendRemindersAt(t, h, at(28, 8, 0))
	expectTaskReminders(t, household.Member, "due_soon: Task due in 30 minutes")
}

func TestInvalidPersonalReminderTimesAreRejected(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	timed := createDueTask(t, household.Owner, household.Member, at(17, 18, 0))
	dated := createDueTask(t, household.Owner, household.Member, localAt(17, 0, 0))

	setPersonalReminderTimes(household.Member, timed.ID, []map[string]any{{"minutes": -60}, {"minutes": -60}}).Expect(http.StatusBadRequest)
	setPersonalReminderTimes(household.Member, timed.ID, []map[string]any{{"days": -31, "at": "08:00"}}).Expect(http.StatusBadRequest)
	setPersonalReminderTimes(household.Member, dated.ID, []map[string]any{{"minutes": -60}}).Expect(http.StatusBadRequest)
	expectUsingShared(t, household.Member, timed.ID)
	expectUsingShared(t, household.Member, dated.ID)
}

func TestReassigningATaskThrowsAwayPersonalReminderTimes(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	alice, bob := household.Member, household.Owner
	task := createTaskWithReminderTimes(t, bob, alice, at(17, 18, 0), []map[string]any{{"minutes": -120}})
	setPersonalReminderTimes(alice, task.ID, []map[string]any{{"minutes": -30}}).Expect(http.StatusOK)

	toBob := updateTask(t, bob, task, map[string]any{"assigned_to": bob.User.ID})
	expectUsingShared(t, bob, task.ID)
	sendRemindersAt(t, h, at(17, 16, 0))
	expectTaskReminders(t, bob, "due_soon: Task due in 2 hours")

	updateTask(t, bob, toBob, map[string]any{"assigned_to": alice.User.ID, "due_at": at(18, 18, 0)})
	expectUsingShared(t, alice, task.ID)
	sendRemindersAt(t, h, at(28, 8, 0))
	expectTaskReminders(t, alice, "due_soon: Task due in 2 hours")
}

func TestSavingATaskForTheSameAssigneeKeepsPersonalReminderTimes(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	task := createDueTask(t, household.Owner, household.Member, at(17, 18, 0))
	setPersonalReminderTimes(household.Member, task.ID, []map[string]any{{"minutes": -30}}).Expect(http.StatusOK)

	updateTask(t, household.Owner, task, map[string]any{"title": "Pay the gas bill", "due_at": at(18, 18, 0)})

	sendRemindersAt(t, h, at(28, 8, 0))
	expectTaskReminders(t, household.Member, "due_soon: Task due in 30 minutes")
}

func TestSwitchingBetweenTimedAndDateOnlyDropsPersonalReminderTimes(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	task := createDueTask(t, household.Owner, household.Member, at(17, 18, 0))
	setPersonalReminderTimes(household.Member, task.ID, []map[string]any{{"minutes": -30}}).Expect(http.StatusOK)

	dated := updateTask(t, household.Owner, task, map[string]any{"due_at": localAt(20, 0, 0)})
	expectUsingShared(t, household.Member, task.ID)
	sendRemindersAt(t, h, localAt(20, 8, 0))
	expectTaskReminders(t, household.Member, "due_soon: Task due in 1 day", "due_now: Task due today")

	setPersonalReminderTimes(household.Member, task.ID, []map[string]any{{"days": -2, "at": "07:00"}}).Expect(http.StatusOK)
	updateTask(t, household.Owner, dated, map[string]any{"due_at": at(25, 18, 0)})
	expectUsingShared(t, household.Member, task.ID)
	sendRemindersAt(t, h, at(28, 8, 0))
	expectTaskReminders(t, household.Member,
		"due_soon: Task due in 1 day", "due_now: Task due today",
		"due_soon: Task due in 24 hours", "due_now: Task due now", "overdue: Task overdue by 1 day")
}

func TestAMemberWhoLeavesTheHouseholdLosesTheirPersonalReminderTimes(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	task := createTaskWithReminderTimes(t, household.Owner, household.Member, at(17, 18, 0), []map[string]any{{"minutes": -120}})
	setPersonalReminderTimes(household.Member, task.ID, []map[string]any{}).Expect(http.StatusOK)

	household.Owner.Delete(fmt.Sprintf("/api/v1/members/%d", household.Member.User.ID)).Expect(http.StatusOK)
	member := readd(t, h, household, household.Member.User)
	// The Task still names them; saving it schedules their Reminders again.
	updateTask(t, household.Owner, task, map[string]any{"title": "Pay the gas bill"})

	expectUsingShared(t, member, task.ID)
	sendRemindersAt(t, h, at(28, 8, 0))
	expectTaskReminders(t, member, "due_soon: Task due in 2 hours")
}

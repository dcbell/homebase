package api_test

import (
	"fmt"
	"net/http"
	"testing"

	"homebase/internal/apitest"
	"homebase/internal/store"
)

func pushSettingsOf(t *testing.T, c *apitest.Client) store.PushSettings {
	t.Helper()
	return apitest.Decode[store.PushSettings](t, c.Get("/api/v1/push-settings").Expect(http.StatusOK))
}

func TestPushIsOnForEveryKindByDefault(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()

	got := pushSettingsOf(t, household.Member)
	want := store.PushSettings{Assignment: true, TaskReminder: true, AppointmentReminder: true}
	if got != want {
		t.Errorf("default Push settings = %+v, want %+v", got, want)
	}
}

func TestTurningAKindOffChangesOnlyThatKind(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()

	updated := apitest.Decode[store.PushSettings](t, household.Member.Patch("/api/v1/push-settings", map[string]any{
		"task_reminder": false,
	}).Expect(http.StatusOK))
	want := store.PushSettings{Assignment: true, TaskReminder: false, AppointmentReminder: true}
	if updated != want {
		t.Errorf("updated Push settings = %+v, want %+v", updated, want)
	}
	if got := pushSettingsOf(t, household.Member); got != want {
		t.Errorf("Push settings after update = %+v, want %+v", got, want)
	}
	if got := pushSettingsOf(t, household.Owner); got != (store.PushSettings{Assignment: true, TaskReminder: true, AppointmentReminder: true}) {
		t.Errorf("another Member's Push settings = %+v, want all on", got)
	}
}

func TestAssignmentWithPushOffIsStoredButNotPushed(t *testing.T) {
	t.Parallel()
	h, recorder := newPushHarness(t)
	household := h.SeedHousehold()
	subscribe(t, household.Member, "https://push.example/phone")
	household.Member.Patch("/api/v1/push-settings", map[string]any{"assignment": false}).Expect(http.StatusOK)

	household.Owner.Post("/api/v1/tasks", map[string]any{
		"title":       "Replace furnace filter",
		"assigned_to": household.Member.User.ID,
	}).Expect(http.StatusCreated)

	list := notificationsOf(t, household.Member)
	if len(list.Notifications) != 1 || list.UnreadCount != 1 {
		t.Fatalf("Member has %d Notifications (%d Unread), want the Assignment stored and Unread", len(list.Notifications), list.UnreadCount)
	}
	if sent := recorder.Sent(); len(sent) != 0 {
		t.Errorf("sent %d Pushes with Assignment Push off, want none", len(sent))
	}

	household.Member.Patch("/api/v1/push-settings", map[string]any{"assignment": true}).Expect(http.StatusOK)
	household.Owner.Post("/api/v1/tasks", map[string]any{
		"title":       "Clean gutters",
		"assigned_to": household.Member.User.ID,
	}).Expect(http.StatusCreated)
	if sent := recorder.Sent(); len(sent) != 1 || sent[0].Payload.Body != "Clean gutters" {
		t.Errorf("sent %+v with Assignment Push back on, want one Push for the new Task", sent)
	}
}

func TestTaskReminderWithPushOffIsStoredButNotPushed(t *testing.T) {
	t.Parallel()
	h, recorder := newPushHarness(t)
	household := h.SeedHousehold()
	subscribe(t, household.Member, "https://push.example/phone")
	household.Member.Patch("/api/v1/push-settings", map[string]any{"task_reminder": false}).Expect(http.StatusOK)
	task := createDueTask(t, household.Owner, household.Member, at(17, 18, 0))
	assignmentPushes := len(recorder.Sent())

	sendRemindersAt(t, h, at(16, 18, 0))

	expectReminders(t, household.Member, fmt.Sprintf("due_soon %d", task.ID))
	if assignmentPushes != 1 {
		t.Errorf("sent %d Pushes for the Assignment, want 1 (Assignment Push is still on)", assignmentPushes)
	}
	if sent := recorder.Sent(); len(sent) != assignmentPushes {
		t.Errorf("sent %d Pushes for the Reminder with Task Reminder Push off, want none", len(sent)-assignmentPushes)
	}
}

func TestAppointmentReminderIsPushedOnlyWhileItsKindIsOn(t *testing.T) {
	t.Parallel()
	h, recorder := newPushHarness(t)
	household := h.SeedHousehold()
	subscribe(t, household.Member, "https://push.example/phone")
	household.Member.Patch("/api/v1/push-settings", map[string]any{"appointment_reminder": false}).Expect(http.StatusOK)
	appt := createAppointmentAt(t, household.Owner, at(17, 9, 0), household.Member.User.ID)
	before := len(recorder.Sent())

	sendRemindersAt(t, h, at(16, 9, 0))
	expectAppointmentReminders(t, household.Member, stages(appt.ID, "due_soon")...)
	if sent := recorder.Sent(); len(sent) != before {
		t.Errorf("sent %d Pushes for the Reminder with Appointment Reminder Push off, want none", len(sent)-before)
	}

	household.Member.Patch("/api/v1/push-settings", map[string]any{"appointment_reminder": true}).Expect(http.StatusOK)
	sendRemindersAt(t, h, at(17, 8, 0))
	expectAppointmentReminders(t, household.Member, stages(appt.ID, "due_soon", "due_soon")...)
	if sent := recorder.Sent(); len(sent) != before+1 {
		t.Errorf("sent %d Pushes for the hour-before Reminder with the kind back on, want 1", len(sent)-before)
	}
}

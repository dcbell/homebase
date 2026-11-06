package api_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"homebase/internal/apitest"
	"homebase/internal/store"
)

// These tests save through the Server's shared operations directly, the way a
// caller other than the JSON API (such as CalDAV) does, and check the same
// rules apply as through the JSON API.

func TestAppointmentSavedOutsideTheJSONAPINotifiesAndRemindsAttendees(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	ctx := context.Background()
	owner := household.Owner.User

	appt, err := h.Server.CreateAppointment(ctx, household.Household, owner, store.EventInput{
		Title:       "Dentist",
		StartsAt:    at(17, 9, 0),
		EndsAt:      at(17, 10, 0),
		AttendeeIDs: []int64{household.Member.User.ID},
	})
	if err != nil {
		t.Fatalf("CreateAppointment: %v", err)
	}
	if got := assignmentsOf(t, household.Member); fmt.Sprint(got) != fmt.Sprint([]string{fmt.Sprintf("appointment %d", appt.ID)}) {
		t.Errorf("Member Assignment Notifications = %v, want the Appointment", got)
	}

	if _, err := h.Server.UpdateAppointment(ctx, household.Household, owner, appt.ID, store.EventInput{
		Title:    "Dentist",
		StartsAt: at(18, 9, 0),
		EndsAt:   at(18, 10, 0),
	}); err != nil {
		t.Fatalf("UpdateAppointment: %v", err)
	}
	sendRemindersAt(t, h, at(17, 9, 0))
	expectAppointmentReminders(t, household.Member, stages(appt.ID, "due_soon")...)

	if err := h.Server.ArchiveAppointment(ctx, household.Household, owner, appt.ID); err != nil {
		t.Fatalf("ArchiveAppointment: %v", err)
	}
	sendRemindersAt(t, h, at(18, 9, 0))
	expectAppointmentReminders(t, household.Member, stages(appt.ID, "due_soon")...)

	if err := h.Server.ArchiveAppointment(ctx, household.Household, owner, appt.ID+1000); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("ArchiveAppointment of a missing Appointment = %v, want store.ErrNotFound", err)
	}
}

func TestTaskSavedOutsideTheJSONAPINotifiesRemindsAndAdvancesItsRoutine(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	ctx := context.Background()
	owner := household.Owner.User
	member := household.Member.User.ID
	due := at(17, 9, 0)

	task, err := h.Server.CreateTask(ctx, household.Household, owner, store.TaskInput{Title: "Pay the water bill", DueAt: &due})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	task, err = h.Server.UpdateTask(ctx, household.Household, owner, task.ID, store.TaskInput{
		Title: "Pay the water bill", DueAt: &due, AssignedTo: &member,
	})
	if err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}
	if got := assignmentsOf(t, household.Member); fmt.Sprint(got) != fmt.Sprint([]string{fmt.Sprintf("task %d", task.ID)}) {
		t.Errorf("Member Assignment Notifications = %v, want the Task", got)
	}

	if _, err := h.Server.CompleteTask(ctx, household.Household, owner, task.ID); err != nil {
		t.Fatalf("CompleteTask: %v", err)
	}
	sendRemindersAt(t, h, at(16, 9, 0))
	expectReminders(t, household.Member)

	if _, err := h.Server.ReopenTask(ctx, household.Household, owner, task.ID); err != nil {
		t.Fatalf("ReopenTask: %v", err)
	}
	sendRemindersAt(t, h, at(17, 9, 0))
	expectReminders(t, household.Member, fmt.Sprintf("due_now %d", task.ID))

	if err := h.Server.ArchiveTask(ctx, household.Household, owner, task.ID); err != nil {
		t.Fatalf("ArchiveTask: %v", err)
	}
	sendRemindersAt(t, h, at(20, 12, 0))
	expectReminders(t, household.Member, fmt.Sprintf("due_now %d", task.ID))

	routine := apitest.Decode[store.Routine](t, household.Owner.Post("/api/v1/routines", map[string]any{
		"title":       "Clean the gutters",
		"cadence":     "quarterly",
		"next_due_at": "2030-01-20T09:00:00Z",
	}).Expect(http.StatusCreated))
	generated := apitest.Decode[store.Task](t, household.Owner.Post(fmt.Sprintf("/api/v1/routines/%d/generate-task", routine.ID), nil).Expect(http.StatusCreated))
	if _, err := h.Server.CompleteTask(ctx, household.Household, owner, generated.ID); err != nil {
		t.Fatalf("CompleteTask of the Generated Task: %v", err)
	}
	advanced := apitest.Decode[store.Routine](t, household.Owner.Get(fmt.Sprintf("/api/v1/routines/%d", routine.ID)).Expect(http.StatusOK))
	if advanced.LastCompletedAt == nil {
		t.Errorf("Routine last_completed_at = nil, want the Routine advanced by completing its Generated Task")
	}
}

// assignmentsOf returns the Assignment Notifications a Member has received,
// oldest first, as "subject-type subject-id" strings.
func assignmentsOf(t *testing.T, c *apitest.Client) []string {
	t.Helper()
	got := []string{}
	list := notificationsOf(t, c)
	for i := len(list.Notifications) - 1; i >= 0; i-- {
		n := list.Notifications[i]
		if n.Kind == "assignment" {
			got = append(got, fmt.Sprintf("%s %d", n.SubjectType, n.SubjectID))
		}
	}
	return got
}

package api_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"homebase/internal/apitest"
	"homebase/internal/store"
)

// readd adds a removed Member back and signs them in, so anything sent to
// them while they were gone is observable.
func readd(t *testing.T, h *apitest.Harness, household apitest.Household, removed store.User) *apitest.Client {
	t.Helper()
	user := apitest.Decode[store.User](t, household.Owner.Post("/api/v1/members", map[string]any{
		"email": removed.Email,
		"name":  removed.Name,
		"role":  "member",
	}).Expect(http.StatusCreated))
	return h.SignIn(user, household.ID)
}

func TestRemovedMemberStillAssignedToATaskGetsNoReminders(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	task := createDueTask(t, household.Owner, household.Member, at(17, 18, 0))
	household.Owner.Patch(fmt.Sprintf("/api/v1/tasks/%d/complete", task.ID), nil).Expect(http.StatusOK)

	household.Owner.Delete(fmt.Sprintf("/api/v1/members/%d", household.Member.User.ID)).Expect(http.StatusOK)
	// The Task still names the removed Member; reopening it must not bring
	// their Reminders back.
	household.Owner.Patch(fmt.Sprintf("/api/v1/tasks/%d/reopen", task.ID), nil).Expect(http.StatusOK)
	sendRemindersAt(t, h, at(16, 18, 0))
	sendRemindersAt(t, h, at(17, 18, 0))
	sendRemindersAt(t, h, at(18, 8, 0))

	member := readd(t, h, household, household.Member.User)
	if got := notificationsOf(t, member); len(got.Notifications) != 0 {
		t.Errorf("removed Member got Notifications %+v, want none", got.Notifications)
	}
}

func TestRemovedMemberGetsNoAssignmentForTasksTheirRoutineGenerates(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	household.Owner.Post("/api/v1/routines", map[string]any{
		"title":       "Water the plants",
		"cadence":     "weekly",
		"next_due_at": "2030-01-15T08:00:00Z",
		"assigned_to": household.Member.User.ID,
	}).Expect(http.StatusCreated)

	household.Owner.Delete(fmt.Sprintf("/api/v1/members/%d", household.Member.User.ID)).Expect(http.StatusOK)
	if err := h.Server.GenerateDueTasks(context.Background()); err != nil {
		t.Fatalf("GenerateDueTasks: %v", err)
	}

	member := readd(t, h, household, household.Member.User)
	if got := notificationsOf(t, member); len(got.Notifications) != 0 {
		t.Errorf("removed Member got Notifications %+v, want none", got.Notifications)
	}
}

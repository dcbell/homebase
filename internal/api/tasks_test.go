package api_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"homebase/internal/apitest"
	"homebase/internal/store"
)

func TestOwnerCreatesTaskThatMemberCanFetch(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()

	created := apitest.Decode[store.Task](t, household.Owner.Post("/api/v1/tasks", map[string]any{
		"title":       "Replace furnace filter",
		"assigned_to": household.Member.User.ID,
	}).Expect(http.StatusCreated))

	got := apitest.Decode[store.Task](t, household.Member.Get(fmt.Sprintf("/api/v1/tasks/%d", created.ID)).Expect(http.StatusOK))
	if got.Title != "Replace furnace filter" {
		t.Errorf("title = %q, want %q", got.Title, "Replace furnace filter")
	}
	if got.AssignedTo == nil || *got.AssignedTo != household.Member.User.ID {
		t.Errorf("assigned_to = %v, want Member %d", got.AssignedTo, household.Member.User.ID)
	}
	if got.CreatedBy != household.Owner.User.ID {
		t.Errorf("created_by = %d, want Owner %d", got.CreatedBy, household.Owner.User.ID)
	}
}

func TestRequestsWithoutCredentialsAreUnauthorized(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)

	h.Anonymous().Get("/api/v1/tasks").Expect(http.StatusUnauthorized)
}

func TestMembersOfOtherHouseholdsCannotFetchTask(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	ours := h.SeedHousehold()
	theirs := h.SeedHousehold()

	created := apitest.Decode[store.Task](t, ours.Owner.Post("/api/v1/tasks", map[string]any{"title": "Ours"}).Expect(http.StatusCreated))

	theirs.Owner.Get(fmt.Sprintf("/api/v1/tasks/%d", created.ID)).Expect(http.StatusNotFound)
}

func TestReadOnlyTokenCanReadButNotWrite(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	reader := household.Member.WithToken("read")

	reader.Get("/api/v1/tasks").Expect(http.StatusOK)
	reader.Post("/api/v1/tasks", map[string]any{"title": "Nope"}).Expect(http.StatusForbidden)
	household.Member.WithToken("write").Post("/api/v1/tasks", map[string]any{"title": "Yes"}).Expect(http.StatusCreated)
}

func TestSavingAlreadyDoneGeneratedTaskDoesNotAdvanceRoutineAgain(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	routine := apitest.Decode[store.Routine](t, household.Owner.Post("/api/v1/routines", map[string]any{
		"title":       "Water the plants",
		"cadence":     "weekly",
		"next_due_at": "2030-01-20T09:00:00Z",
	}).Expect(http.StatusCreated))
	task := apitest.Decode[store.Task](t, household.Owner.Post(fmt.Sprintf("/api/v1/routines/%d/generate-task", routine.ID), nil).Expect(http.StatusCreated))
	done := map[string]any{"title": task.Title, "routine_id": routine.ID, "status": "done"}
	taskPath := fmt.Sprintf("/api/v1/tasks/%d", task.ID)

	household.Owner.Patch(taskPath, done).Expect(http.StatusOK)
	household.Owner.Patch(taskPath, done).Expect(http.StatusOK)

	got := apitest.Decode[store.Routine](t, household.Owner.Get(fmt.Sprintf("/api/v1/routines/%d", routine.ID)).Expect(http.StatusOK))
	if want := "2030-01-27T09:00:00Z"; got.NextDueAt == nil || got.NextDueAt.UTC().Format(time.RFC3339) != want {
		t.Errorf("next_due_at = %v, want %s (advanced once, by one week)", got.NextDueAt, want)
	}
}

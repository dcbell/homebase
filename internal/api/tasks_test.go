package api_test

import (
	"fmt"
	"net/http"
	"testing"

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

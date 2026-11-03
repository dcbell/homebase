package api_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"homebase/internal/apitest"
	"homebase/internal/store"
)

func createList(t *testing.T, c *apitest.Client) store.HouseholdList {
	t.Helper()
	return apitest.Decode[store.HouseholdList](t, c.Post("/api/v1/lists", map[string]any{"title": "Groceries"}).Expect(http.StatusCreated))
}

func expectNoNotifications(t *testing.T, c *apitest.Client) {
	t.Helper()
	if got := notificationsOf(t, c); got.UnreadCount != 0 || len(got.Notifications) != 0 {
		t.Errorf("%s got %+v, want no Notifications", c.User.Name, got)
	}
}

func expectOneNotification(t *testing.T, c *apitest.Client) store.Notification {
	t.Helper()
	got := notificationsOf(t, c)
	if got.UnreadCount != 1 || len(got.Notifications) != 1 {
		t.Fatalf("%s got %+v, want exactly one Unread Notification", c.User.Name, got)
	}
	return got.Notifications[0]
}

func TestAssigningListItemToAnotherMemberNotifiesThem(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	list := createList(t, household.Owner)

	item := apitest.Decode[store.ListItem](t, household.Owner.Post(fmt.Sprintf("/api/v1/lists/%d/items", list.ID), map[string]any{
		"title":       "Oat milk",
		"assigned_to": household.Member.User.ID,
	}).Expect(http.StatusCreated))

	n := expectOneNotification(t, household.Member)
	if n.Kind != "assignment" || n.SubjectType != "list_item" || n.SubjectID != item.ID {
		t.Errorf("Notification = %+v, want an assignment for List Item %d", n, item.ID)
	}
	if want := "Owner 1 assigned you a List Item"; n.Title != want {
		t.Errorf("title = %q, want %q", n.Title, want)
	}
	if n.Body != "Oat milk" {
		t.Errorf("body = %q, want the List Item title", n.Body)
	}
	if want := fmt.Sprintf("/lists/%d", list.ID); n.URL != want {
		t.Errorf("url = %q, want %q (List Items open their List)", n.URL, want)
	}
	expectNoNotifications(t, household.Owner)
}

func TestAssigningListItemToYourselfNotifiesNobody(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	list := createList(t, household.Member)

	household.Member.Post(fmt.Sprintf("/api/v1/lists/%d/items", list.ID), map[string]any{
		"title":       "Bread",
		"assigned_to": household.Member.User.ID,
	}).Expect(http.StatusCreated)

	expectNoNotifications(t, household.Member)
}

func TestReassigningListItemNotifiesOnlyTheNewAssignee(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	list := createList(t, household.Member)
	item := apitest.Decode[store.ListItem](t, household.Member.Post(fmt.Sprintf("/api/v1/lists/%d/items", list.ID), map[string]any{
		"title":       "Eggs",
		"assigned_to": household.Member.User.ID,
	}).Expect(http.StatusCreated))
	path := fmt.Sprintf("/api/v1/list-items/%d", item.ID)

	household.Member.Patch(path, map[string]any{"title": "Eggs", "assigned_to": household.Owner.User.ID}).Expect(http.StatusOK)
	household.Member.Patch(path, map[string]any{"title": "A dozen eggs", "assigned_to": household.Owner.User.ID}).Expect(http.StatusOK)

	n := expectOneNotification(t, household.Owner)
	if n.SubjectType != "list_item" || n.SubjectID != item.ID || n.Title != "Member 1 assigned you a List Item" {
		t.Errorf("Notification = %+v, want Member 1 assigning List Item %d", n, item.ID)
	}
	expectNoNotifications(t, household.Member)
}

func TestAssigningRoutineToAnotherMemberNotifiesThem(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()

	routine := apitest.Decode[store.Routine](t, household.Owner.Post("/api/v1/routines", map[string]any{
		"title":       "Take out the recycling",
		"cadence":     "weekly",
		"assigned_to": household.Member.User.ID,
	}).Expect(http.StatusCreated))

	n := expectOneNotification(t, household.Member)
	if n.Kind != "assignment" || n.SubjectType != "routine" || n.SubjectID != routine.ID {
		t.Errorf("Notification = %+v, want an assignment for Routine %d", n, routine.ID)
	}
	if want := "Owner 1 assigned you a Routine"; n.Title != want {
		t.Errorf("title = %q, want %q", n.Title, want)
	}
	if n.Body != "Take out the recycling" {
		t.Errorf("body = %q, want the Routine title", n.Body)
	}
	if want := fmt.Sprintf("/routines/%d", routine.ID); n.URL != want {
		t.Errorf("url = %q, want %q", n.URL, want)
	}
	expectNoNotifications(t, household.Owner)
}

func TestAssigningRoutineToYourselfNotifiesNobody(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()

	household.Owner.Post("/api/v1/routines", map[string]any{
		"title":       "Change the water filter",
		"assigned_to": household.Owner.User.ID,
	}).Expect(http.StatusCreated)

	expectNoNotifications(t, household.Owner)
}

func TestReassigningRoutineNotifiesOnlyTheNewAssignee(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	routine := apitest.Decode[store.Routine](t, household.Owner.Post("/api/v1/routines", map[string]any{
		"title":       "Vacuum the stairs",
		"assigned_to": household.Owner.User.ID,
	}).Expect(http.StatusCreated))
	path := fmt.Sprintf("/api/v1/routines/%d", routine.ID)

	household.Owner.Patch(path, map[string]any{"title": "Vacuum the stairs", "assigned_to": household.Member.User.ID}).Expect(http.StatusOK)
	household.Owner.Patch(path, map[string]any{"title": "Vacuum all stairs", "assigned_to": household.Member.User.ID}).Expect(http.StatusOK)

	n := expectOneNotification(t, household.Member)
	if n.SubjectType != "routine" || n.SubjectID != routine.ID || n.Title != "Owner 1 assigned you a Routine" {
		t.Errorf("Notification = %+v, want Owner 1 assigning Routine %d", n, routine.ID)
	}
	expectNoNotifications(t, household.Owner)
}

func expectGeneratedTaskNotification(t *testing.T, c *apitest.Client, task store.Task) {
	t.Helper()
	n := expectOneNotification(t, c)
	if n.Kind != "assignment" || n.SubjectType != "task" || n.SubjectID != task.ID {
		t.Errorf("Notification = %+v, want an assignment for Generated Task %d", n, task.ID)
	}
	if want := "You were assigned a Task"; n.Title != want {
		t.Errorf("title = %q, want %q (no Member performed the generation)", n.Title, want)
	}
	if n.Body != task.Title {
		t.Errorf("body = %q, want the Task title %q", n.Body, task.Title)
	}
	if want := fmt.Sprintf("/tasks/%d", task.ID); n.URL != want {
		t.Errorf("url = %q, want %q", n.URL, want)
	}
}

func TestGeneratingTaskFromRoutineNotifiesItsAssigneeEvenWhenTheyClicked(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	routine := apitest.Decode[store.Routine](t, household.Owner.Post("/api/v1/routines", map[string]any{
		"title":       "Clean the gutters",
		"cadence":     "quarterly",
		"next_due_at": "2030-01-20T09:00:00Z",
		"assigned_to": household.Owner.User.ID,
	}).Expect(http.StatusCreated))
	generate := fmt.Sprintf("/api/v1/routines/%d/generate-task", routine.ID)

	task := apitest.Decode[store.Task](t, household.Owner.Post(generate, nil).Expect(http.StatusCreated))
	household.Owner.Post(generate, nil).Expect(http.StatusCreated)

	expectGeneratedTaskNotification(t, household.Owner, task)
	expectNoNotifications(t, household.Member)
}

func TestGeneratingTaskFromUnassignedRoutineNotifiesNobody(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	routine := apitest.Decode[store.Routine](t, household.Owner.Post("/api/v1/routines", map[string]any{
		"title":       "Test the smoke alarms",
		"next_due_at": "2030-01-20T09:00:00Z",
	}).Expect(http.StatusCreated))

	household.Owner.Post(fmt.Sprintf("/api/v1/routines/%d/generate-task", routine.ID), nil).Expect(http.StatusCreated)

	expectNoNotifications(t, household.Owner)
	expectNoNotifications(t, household.Member)
}

func TestBackgroundCheckNotifiesTheAssigneeOfEachGeneratedTask(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	household.Member.Post("/api/v1/routines", map[string]any{
		"title":       "Feed the sourdough starter",
		"cadence":     "daily",
		"next_due_at": "2030-01-15T08:00:00Z",
		"assigned_to": household.Member.User.ID,
	}).Expect(http.StatusCreated)
	household.Owner.Post("/api/v1/routines", map[string]any{
		"title":       "Not due until next week",
		"cadence":     "weekly",
		"next_due_at": "2030-01-22T08:00:00Z",
		"assigned_to": household.Owner.User.ID,
	}).Expect(http.StatusCreated)
	household.Owner.Post("/api/v1/routines", map[string]any{
		"title":       "Nobody's job",
		"next_due_at": "2030-01-15T08:00:00Z",
	}).Expect(http.StatusCreated)

	if err := h.Server.GenerateDueTasks(context.Background()); err != nil {
		t.Fatalf("GenerateDueTasks: %v", err)
	}
	if err := h.Server.GenerateDueTasks(context.Background()); err != nil {
		t.Fatalf("GenerateDueTasks again: %v", err)
	}

	tasks := apitest.Decode[[]store.Task](t, household.Member.Get("/api/v1/tasks").Expect(http.StatusOK))
	var generated *store.Task
	for i := range tasks {
		if tasks[i].Title == "Feed the sourdough starter" {
			generated = &tasks[i]
		}
	}
	if generated == nil {
		t.Fatalf("no Generated Task in %+v", tasks)
	}
	expectGeneratedTaskNotification(t, household.Member, *generated)
	expectNoNotifications(t, household.Owner)
}

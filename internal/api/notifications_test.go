package api_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"homebase/internal/apitest"
	"homebase/internal/store"
)

func notificationsOf(t *testing.T, c *apitest.Client) store.NotificationList {
	t.Helper()
	return apitest.Decode[store.NotificationList](t, c.Get("/api/v1/notifications").Expect(http.StatusOK))
}

func TestAssigningTaskToAnotherMemberNotifiesThem(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()

	task := apitest.Decode[store.Task](t, household.Owner.Post("/api/v1/tasks", map[string]any{
		"title":       "Replace furnace filter",
		"assigned_to": household.Member.User.ID,
	}).Expect(http.StatusCreated))

	got := notificationsOf(t, household.Member)
	if got.UnreadCount != 1 {
		t.Errorf("unread_count = %d, want 1", got.UnreadCount)
	}
	if len(got.Notifications) != 1 {
		t.Fatalf("got %d Notifications, want 1: %+v", len(got.Notifications), got.Notifications)
	}
	n := got.Notifications[0]
	if n.Kind != "assignment" {
		t.Errorf("kind = %q, want assignment", n.Kind)
	}
	if n.SubjectType != "task" || n.SubjectID != task.ID {
		t.Errorf("subject = %s %d, want task %d", n.SubjectType, n.SubjectID, task.ID)
	}
	if want := fmt.Sprintf("/tasks/%d", task.ID); n.URL != want {
		t.Errorf("url = %q, want %q", n.URL, want)
	}
	if want := "Owner 1 assigned you a Task"; n.Title != want {
		t.Errorf("title = %q, want %q", n.Title, want)
	}
	if n.Body != "Replace furnace filter" {
		t.Errorf("body = %q, want the Task title", n.Body)
	}
	if n.ReadAt != nil {
		t.Errorf("read_at = %v, want unread", n.ReadAt)
	}
	if !n.CreatedAt.Equal(apitest.DefaultNow) {
		t.Errorf("created_at = %v, want the Clock's now %v", n.CreatedAt, apitest.DefaultNow)
	}

	if owner := notificationsOf(t, household.Owner); owner.UnreadCount != 0 || len(owner.Notifications) != 0 {
		t.Errorf("Owner got %+v, want no Notifications", owner)
	}
}

func TestAssigningTaskToYourselfNotifiesNobody(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()

	household.Member.Post("/api/v1/tasks", map[string]any{
		"title":       "Water the plants",
		"assigned_to": household.Member.User.ID,
	}).Expect(http.StatusCreated)

	if got := notificationsOf(t, household.Member); got.UnreadCount != 0 || len(got.Notifications) != 0 {
		t.Errorf("Member got %+v, want no Notifications", got)
	}
}

func TestCreatingUnassignedTaskNotifiesNobody(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()

	household.Owner.Post("/api/v1/tasks", map[string]any{"title": "Someone clean the gutters"}).Expect(http.StatusCreated)

	for _, c := range []*apitest.Client{household.Owner, household.Member} {
		if got := notificationsOf(t, c); got.UnreadCount != 0 || len(got.Notifications) != 0 {
			t.Errorf("%s got %+v, want no Notifications", c.User.Name, got)
		}
	}
}

func TestReassigningTaskNotifiesTheNewAssignee(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	task := apitest.Decode[store.Task](t, household.Member.Post("/api/v1/tasks", map[string]any{
		"title":       "Book the plumber",
		"assigned_to": household.Member.User.ID,
	}).Expect(http.StatusCreated))

	household.Member.Patch(fmt.Sprintf("/api/v1/tasks/%d", task.ID), map[string]any{
		"title":       "Book the plumber",
		"assigned_to": household.Owner.User.ID,
	}).Expect(http.StatusOK)

	got := notificationsOf(t, household.Owner)
	if len(got.Notifications) != 1 || got.UnreadCount != 1 {
		t.Fatalf("Owner got %+v, want one Unread Notification", got)
	}
	if n := got.Notifications[0]; n.SubjectID != task.ID || n.Title != "Member 1 assigned you a Task" {
		t.Errorf("Notification = %+v, want Member 1 assigning Task %d", n, task.ID)
	}
}

func TestEditingTaskWithoutChangingAssigneeDoesNotNotifyAgain(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	task := apitest.Decode[store.Task](t, household.Owner.Post("/api/v1/tasks", map[string]any{
		"title":       "Mow the lawn",
		"assigned_to": household.Member.User.ID,
	}).Expect(http.StatusCreated))

	household.Owner.Patch(fmt.Sprintf("/api/v1/tasks/%d", task.ID), map[string]any{
		"title":       "Mow the front lawn",
		"assigned_to": household.Member.User.ID,
	}).Expect(http.StatusOK)

	if got := notificationsOf(t, household.Member); got.UnreadCount != 1 || len(got.Notifications) != 1 {
		t.Errorf("Member got %+v, want only the original Notification", got)
	}
}

func TestAssigningProjectTaskNotifiesTheAssignee(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	project := apitest.Decode[store.Project](t, household.Owner.Post("/api/v1/projects", map[string]any{"title": "Kitchen remodel"}).Expect(http.StatusCreated))

	task := apitest.Decode[store.Task](t, household.Owner.Post("/api/v1/tasks", map[string]any{
		"title":       "Pick cabinet colour",
		"project_id":  project.ID,
		"assigned_to": household.Member.User.ID,
	}).Expect(http.StatusCreated))

	got := notificationsOf(t, household.Member)
	if len(got.Notifications) != 1 || got.Notifications[0].SubjectID != task.ID {
		t.Errorf("Member got %+v, want a Notification for Project Task %d", got, task.ID)
	}
}

func TestReadingNotificationsChangesUnreadCount(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	for _, title := range []string{"First", "Second", "Third"} {
		household.Owner.Post("/api/v1/tasks", map[string]any{"title": title, "assigned_to": household.Member.User.ID}).Expect(http.StatusCreated)
		h.Clock.Advance(time.Minute)
	}

	listed := notificationsOf(t, household.Member)
	if listed.UnreadCount != 3 {
		t.Fatalf("unread_count = %d, want 3", listed.UnreadCount)
	}
	var bodies []string
	for _, n := range listed.Notifications {
		bodies = append(bodies, n.Body)
	}
	if fmt.Sprint(bodies) != "[Third Second First]" {
		t.Errorf("Notifications = %v, want newest first", bodies)
	}
	if again := notificationsOf(t, household.Member); again.UnreadCount != 3 {
		t.Errorf("unread_count after listing = %d, want 3 (listing marks nothing read)", again.UnreadCount)
	}

	read := apitest.Decode[store.Notification](t, household.Member.Post(fmt.Sprintf("/api/v1/notifications/%d/read", listed.Notifications[0].ID), nil).Expect(http.StatusOK))
	if read.ReadAt == nil || !read.ReadAt.Equal(h.Clock.Now()) {
		t.Errorf("read_at = %v, want the Clock's now %v", read.ReadAt, h.Clock.Now())
	}
	if got := notificationsOf(t, household.Member).UnreadCount; got != 2 {
		t.Errorf("unread_count after reading one = %d, want 2", got)
	}

	household.Member.Post("/api/v1/notifications/read-all", nil).Expect(http.StatusOK)
	got := notificationsOf(t, household.Member)
	if got.UnreadCount != 0 {
		t.Errorf("unread_count after mark all read = %d, want 0", got.UnreadCount)
	}
	if len(got.Notifications) != 3 {
		t.Errorf("got %d Notifications after mark all read, want all 3 still listed", len(got.Notifications))
	}
}

func TestNotificationsArePaged(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	for _, title := range []string{"First", "Second", "Third"} {
		household.Owner.Post("/api/v1/tasks", map[string]any{"title": title, "assigned_to": household.Member.User.ID}).Expect(http.StatusCreated)
	}

	page := apitest.Decode[store.NotificationList](t, household.Member.Get("/api/v1/notifications?limit=2").Expect(http.StatusOK))
	if len(page.Notifications) != 2 || page.Notifications[0].Body != "Third" || page.UnreadCount != 3 {
		t.Fatalf("first page = %+v, want Third and Second with unread_count 3", page)
	}
	next := apitest.Decode[store.NotificationList](t, household.Member.Get(fmt.Sprintf("/api/v1/notifications?limit=2&before_id=%d", page.Notifications[1].ID)).Expect(http.StatusOK))
	if len(next.Notifications) != 1 || next.Notifications[0].Body != "First" {
		t.Errorf("second page = %+v, want only First", next)
	}
}

func TestMembersCannotReadOthersNotifications(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	household.Owner.Post("/api/v1/tasks", map[string]any{"title": "Yours", "assigned_to": household.Member.User.ID}).Expect(http.StatusCreated)
	n := notificationsOf(t, household.Member).Notifications[0]

	household.Owner.Post(fmt.Sprintf("/api/v1/notifications/%d/read", n.ID), nil).Expect(http.StatusNotFound)

	if got := notificationsOf(t, household.Member).UnreadCount; got != 1 {
		t.Errorf("Member unread_count = %d, want 1", got)
	}
}

func TestReadOnlyTokenCanListButNotMarkNotificationsRead(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	household.Owner.Post("/api/v1/tasks", map[string]any{"title": "Yours", "assigned_to": household.Member.User.ID}).Expect(http.StatusCreated)
	reader := household.Member.WithToken("read")

	listed := notificationsOf(t, reader)
	if listed.UnreadCount != 1 {
		t.Fatalf("unread_count via read token = %d, want 1", listed.UnreadCount)
	}
	reader.Post(fmt.Sprintf("/api/v1/notifications/%d/read", listed.Notifications[0].ID), nil).Expect(http.StatusForbidden)
	reader.Post("/api/v1/notifications/read-all", nil).Expect(http.StatusForbidden)

	writer := household.Member.WithToken("write")
	writer.Post(fmt.Sprintf("/api/v1/notifications/%d/read", listed.Notifications[0].ID), nil).Expect(http.StatusOK)
	if got := notificationsOf(t, reader).UnreadCount; got != 0 {
		t.Errorf("unread_count after write token marks read = %d, want 0", got)
	}
}

func TestDashboardCarriesStoredNotificationsInsteadOfRoutineNotices(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	household.Owner.Post("/api/v1/routines", map[string]any{
		"title":       "Change smoke alarm batteries",
		"cadence":     "monthly",
		"next_due_at": apitest.DefaultNow.Add(-48 * time.Hour),
	}).Expect(http.StatusCreated)
	household.Owner.Post("/api/v1/tasks", map[string]any{"title": "Yours", "assigned_to": household.Member.User.ID}).Expect(http.StatusCreated)

	raw := apitest.Decode[map[string]any](t, household.Member.Get("/api/v1/dashboard").Expect(http.StatusOK))
	if _, ok := raw["notices"]; ok {
		t.Errorf("dashboard still has routine notices: %v", raw["notices"])
	}
	dashboard := apitest.Decode[store.Dashboard](t, household.Member.Get("/api/v1/dashboard").Expect(http.StatusOK))
	if dashboard.Notifications.UnreadCount != 1 || len(dashboard.Notifications.Notifications) != 1 || dashboard.Notifications.Notifications[0].Body != "Yours" {
		t.Errorf("dashboard notifications = %+v, want the Member's one Unread Notification", dashboard.Notifications)
	}
}

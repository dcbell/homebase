package api_test

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"testing"
	"time"

	"homebase/internal/apitest"
	"homebase/internal/store"
)

// expireNotificationsAt moves the Clock to at and runs Notification retention.
func expireNotificationsAt(t *testing.T, h *apitest.Harness, at time.Time) {
	t.Helper()
	h.Clock.Set(at)
	if _, err := h.Server.ExpireNotifications(context.Background(), at); err != nil {
		t.Fatalf("expire Notifications: %v", err)
	}
}

func assignTask(t *testing.T, owner *apitest.Client, assignee *apitest.Client, title string) store.Task {
	t.Helper()
	return apitest.Decode[store.Task](t, owner.Post("/api/v1/tasks", map[string]any{
		"title":       title,
		"assigned_to": assignee.User.ID,
	}).Expect(http.StatusCreated))
}

// notificationBodies returns the bodies of a Member's Notifications, sorted.
func notificationBodies(t *testing.T, c *apitest.Client) []string {
	t.Helper()
	got := []string{}
	for _, n := range notificationsOf(t, c).Notifications {
		got = append(got, n.Body)
	}
	sort.Strings(got)
	return got
}

func TestReadNotificationsOlderThan90DaysAreDeleted(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	start := apitest.DefaultNow

	assignTask(t, household.Owner, household.Member, "Old and read")
	assignTask(t, household.Owner, household.Member, "Old and unread")
	household.Member.Post(fmt.Sprintf("/api/v1/notifications/%d/read", notificationIDByBody(t, household.Member, "Old and read")), nil).Expect(http.StatusOK)

	h.Clock.Set(start.Add(10 * 24 * time.Hour))
	assignTask(t, household.Owner, household.Member, "Recent and read")
	household.Member.Post(fmt.Sprintf("/api/v1/notifications/%d/read", notificationIDByBody(t, household.Member, "Recent and read")), nil).Expect(http.StatusOK)

	expireNotificationsAt(t, h, start.Add(90*24*time.Hour-time.Minute))
	if got, want := fmt.Sprint(notificationBodies(t, household.Member)), "[Old and read Old and unread Recent and read]"; got != want {
		t.Fatalf("before 90 days: Notifications = %s, want %s", got, want)
	}

	expireNotificationsAt(t, h, start.Add(91*24*time.Hour))
	if got, want := fmt.Sprint(notificationBodies(t, household.Member)), "[Old and unread Recent and read]"; got != want {
		t.Errorf("after 90 days: Notifications = %s, want %s", got, want)
	}
}

func notificationIDByBody(t *testing.T, c *apitest.Client, body string) int64 {
	t.Helper()
	for _, n := range notificationsOf(t, c).Notifications {
		if n.Body == body {
			return n.ID
		}
	}
	t.Fatalf("%s has no Notification %q", c.User.Name, body)
	return 0
}

func TestRemovingMemberDeletesTheirPushSubscriptionsNotificationsAndPendingReminders(t *testing.T) {
	t.Parallel()
	h, recorder := newPushHarness(t)
	household := h.SeedHousehold()
	subscribe(t, household.Member, "https://push.example/member-phone")
	subscribe(t, household.Owner, "https://push.example/owner-phone")

	createDueTask(t, household.Owner, household.Member, at(20, 12, 0))
	createDueTask(t, household.Member, household.Owner, at(20, 12, 0))
	if got := notificationsOf(t, household.Member); len(got.Notifications) != 1 {
		t.Fatalf("Member has %d Notifications before removal, want 1", len(got.Notifications))
	}

	household.Owner.Delete(fmt.Sprintf("/api/v1/members/%d", household.Member.User.ID)).Expect(http.StatusOK)

	// Add the same person back so what was left of them is observable.
	readded := apitest.Decode[store.User](t, household.Owner.Post("/api/v1/members", map[string]any{
		"email": household.Member.User.Email,
		"name":  household.Member.User.Name,
		"role":  "member",
	}).Expect(http.StatusCreated))
	if readded.ID != household.Member.User.ID {
		t.Fatalf("re-added Member has id %d, want the same user %d", readded.ID, household.Member.User.ID)
	}
	member := h.SignIn(readded, household.ID)

	if got := notificationsOf(t, member); len(got.Notifications) != 0 || got.UnreadCount != 0 {
		t.Errorf("removed Member's Notifications = %+v, want none", got)
	}
	if got := pushSubscriptionsOf(t, member); len(got) != 0 {
		t.Errorf("removed Member's Push Subscriptions = %+v, want none", got)
	}

	sentBefore := len(recorder.Sent())
	sendRemindersAt(t, h, at(22, 9, 0))
	expectReminders(t, member)
	for _, s := range recorder.Sent()[sentBefore:] {
		if s.Subscription.Endpoint == "https://push.example/member-phone" {
			t.Errorf("Pushed %+v to the removed Member's device", s.Payload)
		}
	}

	// The Owner's own Push Subscriptions and Reminders are untouched.
	if got := pushSubscriptionsOf(t, household.Owner); len(got) != 1 {
		t.Errorf("Owner's Push Subscriptions = %+v, want 1", got)
	}
	if got := remindersOf(t, household.Owner); len(got) == 0 {
		t.Errorf("Owner got no Reminders, want theirs to still be sent")
	}
}

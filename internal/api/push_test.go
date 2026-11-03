package api_test

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"homebase/internal/api"
	"homebase/internal/apitest"
	"homebase/internal/push"
	"homebase/internal/push/pushtest"
	"homebase/internal/store"
)

func newPushHarness(t *testing.T) (*apitest.Harness, *pushtest.Recorder) {
	t.Helper()
	recorder := pushtest.NewRecorder()
	return apitest.New(t, api.WithPushSender(recorder)), recorder
}

func subscribe(t *testing.T, c *apitest.Client, endpoint string) store.PushSubscription {
	t.Helper()
	resp := c.Post("/api/v1/push-subscriptions", map[string]any{
		"endpoint": endpoint,
		"keys":     map[string]string{"p256dh": "p256dh-" + endpoint, "auth": "auth-" + endpoint},
	})
	if resp.Status != http.StatusCreated && resp.Status != http.StatusOK {
		resp.Expect(http.StatusCreated)
	}
	return apitest.Decode[store.PushSubscription](t, resp)
}

func pushSubscriptionsOf(t *testing.T, c *apitest.Client) []store.PushSubscription {
	t.Helper()
	return apitest.Decode[[]store.PushSubscription](t, c.Get("/api/v1/push-subscriptions").Expect(http.StatusOK))
}

func TestNotificationIsPushedToEveryPushSubscriptionOfTheRecipient(t *testing.T) {
	t.Parallel()
	h, recorder := newPushHarness(t)
	household := h.SeedHousehold()
	subscribe(t, household.Member, "https://push.example/phone")
	subscribe(t, household.Member, "https://push.example/laptop")
	subscribe(t, household.Owner, "https://push.example/owner")

	task := apitest.Decode[store.Task](t, household.Owner.Post("/api/v1/tasks", map[string]any{
		"title":       "Replace furnace filter",
		"assigned_to": household.Member.User.ID,
	}).Expect(http.StatusCreated))
	notification := notificationsOf(t, household.Member).Notifications[0]

	sent := recorder.Sent()
	got := map[string]push.Payload{}
	for _, s := range sent {
		got[s.Subscription.Endpoint] = s.Payload
	}
	if len(sent) != 2 || len(got) != 2 {
		t.Fatalf("sent %d Pushes to %v, want one to each of the Member's two Push Subscriptions", len(sent), got)
	}
	want := push.Payload{
		Title:          "Owner 1 assigned you a Task",
		Body:           "Replace furnace filter",
		URL:            fmt.Sprintf("/tasks/%d", task.ID),
		NotificationID: notification.ID,
		UnreadCount:    1,
	}
	for _, endpoint := range []string{"https://push.example/phone", "https://push.example/laptop"} {
		if got[endpoint] != want {
			t.Errorf("Push to %s = %+v, want %+v", endpoint, got[endpoint], want)
		}
	}
	for _, s := range sent {
		if s.Subscription.P256dh != "p256dh-"+s.Subscription.Endpoint || s.Subscription.Auth != "auth-"+s.Subscription.Endpoint {
			t.Errorf("Push to %s used keys %+v, want the registered keys", s.Subscription.Endpoint, s.Subscription)
		}
	}
}

func TestPushCarriesTheRecipientsCurrentUnreadCount(t *testing.T) {
	t.Parallel()
	h, recorder := newPushHarness(t)
	household := h.SeedHousehold()
	subscribe(t, household.Member, "https://push.example/phone")

	for _, title := range []string{"First", "Second", "Third"} {
		household.Owner.Post("/api/v1/tasks", map[string]any{"title": title, "assigned_to": household.Member.User.ID}).Expect(http.StatusCreated)
	}
	household.Member.Post("/api/v1/notifications/read-all", nil).Expect(http.StatusOK)
	household.Owner.Post("/api/v1/tasks", map[string]any{"title": "Fourth", "assigned_to": household.Member.User.ID}).Expect(http.StatusCreated)

	var counts []int
	for _, s := range recorder.Sent() {
		counts = append(counts, s.Payload.UnreadCount)
	}
	if fmt.Sprint(counts) != "[1 2 3 1]" {
		t.Errorf("unread counts pushed = %v, want [1 2 3 1]", counts)
	}
}

func TestGonePushSubscriptionsAreRemoved(t *testing.T) {
	t.Parallel()
	h, recorder := newPushHarness(t)
	household := h.SeedHousehold()
	subscribe(t, household.Member, "https://push.example/old-phone")
	subscribe(t, household.Member, "https://push.example/laptop")
	recorder.Gone("https://push.example/old-phone")
	h.Clock.Advance(time.Hour)

	household.Owner.Post("/api/v1/tasks", map[string]any{"title": "Sweep the porch", "assigned_to": household.Member.User.ID}).Expect(http.StatusCreated)

	subs := pushSubscriptionsOf(t, household.Member)
	if len(subs) != 1 || subs[0].Endpoint != "https://push.example/laptop" {
		t.Fatalf("Push Subscriptions = %+v, want only the laptop", subs)
	}
	if subs[0].LastUsedAt == nil || !subs[0].LastUsedAt.Equal(h.Clock.Now()) {
		t.Errorf("laptop last_used_at = %v, want the Clock's now %v", subs[0].LastUsedAt, h.Clock.Now())
	}
}

func TestRegisteringAPushSubscriptionIsIdempotentPerEndpoint(t *testing.T) {
	t.Parallel()
	h, recorder := newPushHarness(t)
	household := h.SeedHousehold()

	first := household.Member.Post("/api/v1/push-subscriptions", map[string]any{
		"endpoint": "https://push.example/phone",
		"keys":     map[string]string{"p256dh": "old-key", "auth": "old-auth"},
	}).Expect(http.StatusCreated)
	again := household.Member.Post("/api/v1/push-subscriptions", map[string]any{
		"endpoint": "https://push.example/phone",
		"keys":     map[string]string{"p256dh": "new-key", "auth": "new-auth"},
	}).Expect(http.StatusOK)
	if a, b := apitest.Decode[store.PushSubscription](t, first), apitest.Decode[store.PushSubscription](t, again); a.ID != b.ID {
		t.Errorf("registering again gave id %d, want the same Push Subscription %d", b.ID, a.ID)
	}

	if subs := pushSubscriptionsOf(t, household.Member); len(subs) != 1 {
		t.Fatalf("Push Subscriptions = %+v, want exactly one", subs)
	}
	household.Owner.Post("/api/v1/tasks", map[string]any{"title": "Fold laundry", "assigned_to": household.Member.User.ID}).Expect(http.StatusCreated)
	sent := recorder.Sent()
	if len(sent) != 1 || sent[0].Subscription.P256dh != "new-key" || sent[0].Subscription.Auth != "new-auth" {
		t.Errorf("sent %+v, want one Push using the latest keys", sent)
	}
}

func TestPushSubscriptionsAreListedWithARecognizableName(t *testing.T) {
	t.Parallel()
	h, _ := newPushHarness(t)
	household := h.SeedHousehold()
	subscribe(t, household.Member, "https://push.example/phone")
	subscribe(t, household.Owner, "https://push.example/owner")

	subs := pushSubscriptionsOf(t, household.Member)
	if len(subs) != 1 || subs[0].Endpoint != "https://push.example/phone" {
		t.Fatalf("Member's Push Subscriptions = %+v, want only their own", subs)
	}
	if subs[0].Name == "" {
		t.Error("Push Subscription has no display name")
	}
	if !subs[0].CreatedAt.Equal(h.Clock.Now()) {
		t.Errorf("created_at = %v, want the Clock's now %v", subs[0].CreatedAt, h.Clock.Now())
	}
	if subs[0].LastUsedAt != nil {
		t.Errorf("last_used_at = %v, want unset before any Push", subs[0].LastUsedAt)
	}
	if body := string(household.Member.Get("/api/v1/push-subscriptions").Body); strings.Contains(body, "p256dh") || strings.Contains(body, `"auth"`) {
		t.Errorf("listing exposes key material: %s", body)
	}
}

func TestRemovedPushSubscriptionReceivesNoPush(t *testing.T) {
	t.Parallel()
	h, recorder := newPushHarness(t)
	household := h.SeedHousehold()
	lost := subscribe(t, household.Member, "https://push.example/lost-phone")
	subscribe(t, household.Member, "https://push.example/laptop")

	household.Owner.Delete(fmt.Sprintf("/api/v1/push-subscriptions/%d", lost.ID)).Expect(http.StatusNotFound)
	household.Member.Delete(fmt.Sprintf("/api/v1/push-subscriptions/%d", lost.ID)).Expect(http.StatusOK)
	household.Member.Delete(fmt.Sprintf("/api/v1/push-subscriptions/%d", lost.ID)).Expect(http.StatusNotFound)

	household.Owner.Post("/api/v1/tasks", map[string]any{"title": "Take out recycling", "assigned_to": household.Member.User.ID}).Expect(http.StatusCreated)
	sent := recorder.Sent()
	if len(sent) != 1 || sent[0].Subscription.Endpoint != "https://push.example/laptop" {
		t.Errorf("sent %+v, want one Push to the laptop only", sent)
	}
}

func TestRegisteringAPushSubscriptionRequiresABrowserSession(t *testing.T) {
	t.Parallel()
	h, _ := newPushHarness(t)
	household := h.SeedHousehold()
	body := map[string]any{"endpoint": "https://push.example/phone", "keys": map[string]string{"p256dh": "k", "auth": "a"}}

	household.Member.WithToken("write").Post("/api/v1/push-subscriptions", body).Expect(http.StatusForbidden)
	h.Anonymous().Post("/api/v1/push-subscriptions", body).Expect(http.StatusUnauthorized)
	household.Member.Post("/api/v1/push-subscriptions", map[string]any{"endpoint": "https://push.example/phone"}).Expect(http.StatusBadRequest)
	household.Member.Post("/api/v1/push-subscriptions", map[string]any{"endpoint": "http://push.example/phone", "keys": map[string]string{"p256dh": "k", "auth": "a"}}).Expect(http.StatusBadRequest)

	if subs := pushSubscriptionsOf(t, household.Member); len(subs) != 0 {
		t.Errorf("Push Subscriptions = %+v, want none", subs)
	}
}

func TestVAPIDPublicKeyIsGeneratedOnceAndKept(t *testing.T) {
	t.Parallel()
	h, _ := newPushHarness(t)
	household := h.SeedHousehold()

	first := apitest.Decode[map[string]string](t, household.Member.Get("/api/v1/push/vapid-public-key").Expect(http.StatusOK))["public_key"]
	key, err := base64.RawURLEncoding.DecodeString(first)
	if err != nil || len(key) != 65 || key[0] != 0x04 {
		t.Fatalf("public_key = %q, want a base64url uncompressed P-256 point", first)
	}
	again := apitest.Decode[map[string]string](t, household.Owner.WithToken("read").Get("/api/v1/push/vapid-public-key").Expect(http.StatusOK))["public_key"]
	if again != first {
		t.Errorf("public_key changed from %q to %q", first, again)
	}
}

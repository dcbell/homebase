package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"homebase/internal/api"
	"homebase/internal/apitest"
	"homebase/internal/push"
)

// stalledSender is a push service that does not answer until released.
type stalledSender struct {
	release chan struct{}
	mu      sync.Mutex
	sent    []string
}

func newStalledSender() *stalledSender {
	return &stalledSender{release: make(chan struct{})}
}

func (s *stalledSender) Send(ctx context.Context, sub push.Subscription, _ []byte) (push.Result, error) {
	select {
	case <-s.release:
	case <-ctx.Done():
		return push.Failed, ctx.Err()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, sub.Endpoint)
	return push.Delivered, nil
}

func (s *stalledSender) endpoints() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.sent...)
}

// within fails the test unless fn returns within d.
func within(t *testing.T, d time.Duration, what string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatalf("%s waited for a stalled push service", what)
	}
}

func TestRequestsDoNotWaitForPushServices(t *testing.T) {
	t.Parallel()
	sender := newStalledSender()
	h := apitest.New(t, api.WithPushSender(sender))
	household := h.SeedHousehold()
	subscribe(t, household.Member, "https://push.example/phone")

	// Sent by hand: the harness's Client waits for Push after each request.
	session, err := h.Store.CreateSession(context.Background(), household.Owner.User.ID, household.ID, time.Hour)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	body, _ := json.Marshal(map[string]any{"title": "Mow the lawn", "assigned_to": household.Member.User.ID})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: h.Config.SessionCookieName, Value: session.ID})
	rec := httptest.NewRecorder()
	within(t, 5*time.Second, "creating a Task", func() { h.Server.ServeHTTP(rec, req) })
	if rec.Code != http.StatusCreated {
		t.Fatalf("create Task: status %d: %s", rec.Code, rec.Body)
	}

	close(sender.release)
	h.Server.WaitForPush()
	if got := sender.endpoints(); len(got) != 1 || got[0] != "https://push.example/phone" {
		t.Errorf("Pushes sent = %v, want one to the Member's phone once the push service answered", got)
	}
}

func TestReminderRunnerDoesNotWaitForPushServices(t *testing.T) {
	t.Parallel()
	sender := newStalledSender()
	h := apitest.New(t, api.WithPushSender(sender))
	household := h.SeedHousehold()
	subscribe(t, household.Member, "https://push.example/phone")
	createDueTask(t, household.Member, household.Member, at(15, 18, 0))

	h.Clock.Set(at(15, 18, 0))
	within(t, 5*time.Second, "sending due Reminders", func() {
		if _, err := h.Server.SendDueReminders(context.Background(), at(15, 18, 0)); err != nil {
			t.Errorf("send due Reminders: %v", err)
		}
	})

	close(sender.release)
	h.Server.WaitForPush()
	if got := sender.endpoints(); len(got) != 1 {
		t.Errorf("Pushes sent = %v, want the Due Now Reminder once the push service answered", got)
	}
}

func TestClosingTheServerSendsQueuedPushes(t *testing.T) {
	t.Parallel()
	sender := newStalledSender()
	h := apitest.New(t, api.WithPushSender(sender))
	household := h.SeedHousehold()
	subscribe(t, household.Member, "https://push.example/phone")
	createDueTask(t, household.Member, household.Member, at(15, 18, 0))
	h.Clock.Set(at(15, 18, 0))
	if _, err := h.Server.SendDueReminders(context.Background(), at(15, 18, 0)); err != nil {
		t.Fatalf("send due Reminders: %v", err)
	}

	close(sender.release)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.Server.Close(ctx); err != nil {
		t.Fatalf("close: %v", err)
	}
	if got := sender.endpoints(); len(got) != 1 {
		t.Errorf("Pushes sent before Close returned = %v, want the queued one", got)
	}
}

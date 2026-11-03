package api_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"homebase/internal/api"
	"homebase/internal/apitest"
	"homebase/internal/clock"
)

// failingClock cancels a context the first time it is read after being armed,
// so whatever the Notifier does next with that context fails the way a lost
// database connection or an expired deadline would.
type failingClock struct {
	*clock.Fake
	mu     sync.Mutex
	cancel context.CancelFunc
}

func (c *failingClock) failNextUse(cancel context.CancelFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cancel = cancel
}

func (c *failingClock) Now() time.Time {
	c.mu.Lock()
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	c.mu.Unlock()
	return c.Fake.Now()
}

func TestReminderThatFailsToSendIsRetriedOnTheNextRun(t *testing.T) {
	t.Parallel()
	failing := &failingClock{}
	h := apitest.New(t, api.WithClock(failing))
	failing.Fake = h.Clock
	household := h.SeedHousehold()
	task := createDueTask(t, household.Owner, household.Member, at(17, 18, 0))

	h.Clock.Set(at(16, 18, 0))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	failing.failNextUse(cancel)
	if _, err := h.Server.SendDueReminders(ctx, at(16, 18, 0)); err == nil {
		t.Fatalf("SendDueReminders succeeded although its context was cancelled mid-send")
	}
	expectReminders(t, household.Member)

	sendRemindersAt(t, h, at(16, 18, 1))
	expectReminders(t, household.Member, fmt.Sprintf("due_soon %d", task.ID))
}

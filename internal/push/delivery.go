package push

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"homebase/internal/clock"
	"homebase/internal/store"
)

const (
	// deliveryWorkers is how many Pushes are sent at once.
	deliveryWorkers = 4
	// deliveryQueueSize bounds the Pushes waiting to be sent. Beyond it new
	// Pushes are dropped; their Notifications are still in the bell.
	deliveryQueueSize = 256
	// sendTimeout bounds one Push to one Push Subscription.
	sendTimeout = 15 * time.Second
)

// Delivery pushes each stored Notification to every Push Subscription of its
// recipient. It implements notify.Delivery.
//
// Deliver only reads what to send (the recipient's Push Subscriptions and
// Unread count, as of the Notification) and queues it; a small pool of
// background workers talks to the push services, so requests and the
// Reminder runner never wait on them. Close drains the queue on shutdown.
type Delivery struct {
	store  *store.Store
	sender Sender
	clock  clock.Clock
	logger *slog.Logger

	queue   chan job
	pending sync.WaitGroup // queued or in-flight Pushes
	workers sync.WaitGroup

	// ctx is the workers' own context, cancelled when Close gives up
	// waiting, so in-flight sends stop.
	ctx    context.Context
	cancel context.CancelFunc

	mu     sync.RWMutex
	closed bool
}

// job is one Push to one Push Subscription.
type job struct {
	notificationID int64
	sub            store.PushSubscription
	payload        []byte
}

// NewDelivery starts a Delivery's background workers. Call Close to stop them.
func NewDelivery(st *store.Store, sender Sender, c clock.Clock, logger *slog.Logger) *Delivery {
	ctx, cancel := context.WithCancel(context.Background())
	d := &Delivery{
		store:  st,
		sender: sender,
		clock:  c,
		logger: logger,
		queue:  make(chan job, deliveryQueueSize),
		ctx:    ctx,
		cancel: cancel,
	}
	for range deliveryWorkers {
		d.workers.Add(1)
		go d.work()
	}
	return d
}

// Deliver queues n for every Push Subscription of its recipient. Failures
// are logged, never returned: the Notification is already in the bell.
func (d *Delivery) Deliver(ctx context.Context, n store.Notification) {
	subs, err := d.store.ListPushSubscriptions(ctx, n.HouseholdID, n.RecipientID)
	if err != nil {
		d.logger.Error("list push subscriptions", "notification_id", n.ID, "error", err)
		return
	}
	if len(subs) == 0 {
		return
	}
	unread, err := d.store.UnreadNotificationCount(ctx, n.HouseholdID, n.RecipientID)
	if err != nil {
		d.logger.Error("count unread notifications", "notification_id", n.ID, "error", err)
		return
	}
	payload, err := json.Marshal(Payload{
		Title:          n.Title,
		Body:           n.Body,
		URL:            n.URL,
		NotificationID: n.ID,
		UnreadCount:    unread,
	})
	if err != nil {
		d.logger.Error("encode push payload", "notification_id", n.ID, "error", err)
		return
	}

	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.closed {
		d.logger.Warn("push delivery closed; not pushing", "notification_id", n.ID)
		return
	}
	for _, sub := range subs {
		d.pending.Add(1)
		select {
		case d.queue <- job{notificationID: n.ID, sub: sub, payload: payload}:
		default:
			d.pending.Done()
			d.logger.Warn("push queue full; dropping push", "notification_id", n.ID, "subscription_id", sub.ID)
		}
	}
}

// Wait blocks until every Push queued so far has been sent or has failed.
// Tests use it to observe Push deterministically.
func (d *Delivery) Wait() {
	d.pending.Wait()
}

// Close stops accepting Pushes and waits for the queued ones to be sent. If
// ctx ends first, in-flight sends are cancelled and the rest are dropped.
func (d *Delivery) Close(ctx context.Context) error {
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return nil
	}
	d.closed = true
	close(d.queue)
	d.mu.Unlock()

	done := make(chan struct{})
	go func() {
		d.workers.Wait()
		close(done)
	}()
	select {
	case <-done:
		d.cancel()
		return nil
	case <-ctx.Done():
		d.cancel()
		<-done
		return ctx.Err()
	}
}

func (d *Delivery) work() {
	defer d.workers.Done()
	for j := range d.queue {
		d.send(j)
		d.pending.Done()
	}
}

func (d *Delivery) send(j job) {
	ctx, cancel := context.WithTimeout(d.ctx, sendTimeout)
	defer cancel()
	sub := j.sub
	result, err := d.sender.Send(ctx, Subscription{Endpoint: sub.Endpoint, P256dh: sub.P256dh, Auth: sub.Auth}, j.payload)
	switch result {
	case Delivered:
		if err := d.store.TouchPushSubscription(ctx, sub.ID, d.clock.Now()); err != nil {
			d.logger.Error("record push subscription use", "subscription_id", sub.ID, "error", err)
		}
	case Gone:
		d.logger.Info("forgetting gone push subscription", "subscription_id", sub.ID, "user_id", sub.UserID)
		if err := d.store.ForgetPushSubscription(ctx, sub.ID); err != nil {
			d.logger.Error("forget push subscription", "subscription_id", sub.ID, "error", err)
		}
	default:
		d.logger.Warn("push failed", "subscription_id", sub.ID, "notification_id", j.notificationID, "error", err)
	}
}

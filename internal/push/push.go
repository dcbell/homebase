// Package push delivers stored Notifications to Members' Push Subscriptions
// with standard Web Push and VAPID (ADR 0003).
//
// The Sender interface is the seam: production uses WebPush, tests use
// pushtest.Recorder. Delivery plugs into the Notifier as its notify.Delivery.
package push

import (
	"context"
	"encoding/json"
	"log/slog"

	"homebase/internal/clock"
	"homebase/internal/store"
)

// Subscription is where to send a Push and the keys to encrypt it with.
type Subscription struct {
	Endpoint string
	P256dh   string
	Auth     string
}

// Result is what a push service said about one Push.
type Result int

const (
	// Delivered means the push service accepted the Push.
	Delivered Result = iota
	// Gone means the subscription no longer exists (404 or 410) and should
	// be forgotten.
	Gone
	// Failed is a temporary failure; the subscription is kept.
	Failed
)

func (r Result) String() string {
	switch r {
	case Delivered:
		return "delivered"
	case Gone:
		return "gone"
	default:
		return "failed"
	}
}

// Sender sends one encrypted payload to one Push Subscription.
type Sender interface {
	Send(ctx context.Context, sub Subscription, payload []byte) (Result, error)
}

// Payload is what the service worker receives with each Push.
type Payload struct {
	Title          string `json:"title"`
	Body           string `json:"body"`
	URL            string `json:"url"`
	NotificationID int64  `json:"notification_id"`
	UnreadCount    int    `json:"unread_count"`
}

// Delivery pushes each stored Notification to every Push Subscription of its
// recipient. It implements notify.Delivery.
type Delivery struct {
	store  *store.Store
	sender Sender
	clock  clock.Clock
	logger *slog.Logger
}

func NewDelivery(st *store.Store, sender Sender, c clock.Clock, logger *slog.Logger) *Delivery {
	return &Delivery{store: st, sender: sender, clock: c, logger: logger}
}

// Deliver sends n to all of its recipient's Push Subscriptions. Failures are
// logged, never returned: the Notification is already in the bell.
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

	for _, sub := range subs {
		result, err := d.sender.Send(ctx, Subscription{Endpoint: sub.Endpoint, P256dh: sub.P256dh, Auth: sub.Auth}, payload)
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
			d.logger.Warn("push failed", "subscription_id", sub.ID, "notification_id", n.ID, "error", err)
		}
	}
}

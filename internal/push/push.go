// Package push delivers stored Notifications to Members' Push Subscriptions
// with standard Web Push and VAPID (ADR 0003).
//
// The Sender interface is the seam: production uses WebPush, tests use
// pushtest.Recorder. Delivery plugs into the Notifier as its notify.Delivery
// and sends in the background, so no request waits on a push service.
package push

import (
	"context"
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


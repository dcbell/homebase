// Package notify holds the Notifier: the one place that decides whether a
// domain change produces Notifications, who receives them, and how they are
// delivered beyond the in-app bell.
//
// Domain mutations report what happened (for example TaskChanged with the
// Task before and after) and never decide anything themselves. Every
// Notification the Notifier stores is then handed to its Delivery, which is
// where Push plugs in.
package notify

import (
	"context"
	"fmt"
	"log/slog"

	"homebase/internal/clock"
	"homebase/internal/store"
)

// Delivery sends a stored Notification somewhere beyond the bell, such as a
// Member's Push Subscriptions. It runs after the Notification is stored, so a
// failed delivery never loses the Notification.
type Delivery interface {
	Deliver(ctx context.Context, n store.Notification)
}

// DeliveryFunc adapts a function to a Delivery.
type DeliveryFunc func(ctx context.Context, n store.Notification)

func (f DeliveryFunc) Deliver(ctx context.Context, n store.Notification) {
	f(ctx, n)
}

// Option configures a Notifier.
type Option func(*Notifier)

// WithDelivery sets how stored Notifications are delivered. The default
// delivers nothing beyond the bell.
func WithDelivery(d Delivery) Option {
	return func(n *Notifier) {
		n.delivery = d
	}
}

// Notifier turns domain events into Notifications.
type Notifier struct {
	store    *store.Store
	clock    clock.Clock
	logger   *slog.Logger
	delivery Delivery
}

func New(st *store.Store, c clock.Clock, logger *slog.Logger, opts ...Option) *Notifier {
	n := &Notifier{
		store:    st,
		clock:    c,
		logger:   logger,
		delivery: DeliveryFunc(func(context.Context, store.Notification) {}),
	}
	for _, opt := range opts {
		opt(n)
	}
	return n
}

// TaskChange describes a Task that was created or updated.
type TaskChange struct {
	// Actor is the Member who made the change, or nil when the system did
	// (a Routine or Maintenance Schedule generating the Task).
	Actor *store.User
	// Before is the Task as it was, or nil when it was just created.
	Before *store.Task
	// After is the Task as it is now.
	After store.Task
}

// TaskChanged is called after every Task create or update.
func (n *Notifier) TaskChanged(ctx context.Context, change TaskChange) error {
	task := change.After
	if task.AssignedTo == nil {
		return nil
	}
	if change.Before != nil && change.Before.AssignedTo != nil && *change.Before.AssignedTo == *task.AssignedTo {
		return nil
	}
	return n.assigned(ctx, change.Actor, assignment{
		householdID: task.HouseholdID,
		recipientID: *task.AssignedTo,
		subjectType: store.SubjectTask,
		subjectID:   task.ID,
		noun:        "Task",
		name:        task.Title,
	})
}

type assignment struct {
	householdID int64
	recipientID int64
	subjectType string
	subjectID   int64
	noun        string
	name        string
}

// assigned sends an Assignment Notification unless the actor assigned the
// item to themselves.
func (n *Notifier) assigned(ctx context.Context, actor *store.User, a assignment) error {
	if actor != nil && actor.ID == a.recipientID {
		return nil
	}
	title := fmt.Sprintf("You were assigned a %s", a.noun)
	if actor != nil {
		title = fmt.Sprintf("%s assigned you a %s", actor.Name, a.noun)
	}
	return n.notify(ctx, store.NotificationInput{
		HouseholdID: a.householdID,
		RecipientID: a.recipientID,
		Kind:        store.NotificationAssignment,
		SubjectType: a.subjectType,
		SubjectID:   a.subjectID,
		Title:       title,
		Body:        a.name,
	})
}

// notify stores a Notification and hands it to the Delivery.
func (n *Notifier) notify(ctx context.Context, input store.NotificationInput) error {
	input.CreatedAt = n.clock.Now()
	stored, err := n.store.CreateNotification(ctx, input)
	if err != nil {
		return fmt.Errorf("store %s notification: %w", input.Kind, err)
	}
	n.delivery.Deliver(ctx, stored)
	return nil
}

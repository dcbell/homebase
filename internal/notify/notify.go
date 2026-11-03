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
	"errors"
	"fmt"
	"log/slog"
	"strings"

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

	// reminderRenderers build the Notification for a due Reminder, by
	// subject type.
	reminderRenderers map[string]reminderRenderer
}

func New(st *store.Store, c clock.Clock, logger *slog.Logger, opts ...Option) *Notifier {
	n := &Notifier{
		store:    st,
		clock:    c,
		logger:   logger,
		delivery: DeliveryFunc(func(context.Context, store.Notification) {}),
	}
	n.reminderRenderers = map[string]reminderRenderer{
		store.SubjectTask:        n.renderTaskReminder,
		store.SubjectAppointment: n.renderAppointmentReminder,
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

// TaskChanged is called after every Task create, update, completion, reopen
// or archive. It keeps the Task's Reminders following its current assignee,
// due date and status, and sends an Assignment Notification when the Task
// became someone's.
func (n *Notifier) TaskChanged(ctx context.Context, change TaskChange) error {
	task := change.After
	if err := n.taskReminders(ctx, task); err != nil {
		return err
	}
	var before *int64
	if change.Before != nil {
		before = change.Before.AssignedTo
	}
	if !newlyAssigned(before, task.AssignedTo) {
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

// AppointmentChange describes an Appointment that was created or updated.
// Its Attendees are the Members it is for.
type AppointmentChange struct {
	// Actor is the Member who made the change, or nil when the system did.
	Actor *store.User
	// Before is the Appointment as it was, or nil when it was just created.
	Before *store.Event
	// After is the Appointment as it is now.
	After store.Event
}

// AppointmentChanged is called after every Appointment create or update.
// Each Member who has just become an Attendee gets an Assignment
// Notification, unless they added themselves.
func (n *Notifier) AppointmentChanged(ctx context.Context, change AppointmentChange) error {
	appointment := change.After
	wasAttending := map[int64]bool{}
	if change.Before != nil {
		for _, id := range change.Before.AttendeeIDs {
			wasAttending[id] = true
		}
	}
	var errs []error
	if err := n.appointmentReminders(ctx, appointment); err != nil {
		errs = append(errs, err)
	}
	for _, id := range appointment.AttendeeIDs {
		if wasAttending[id] {
			continue
		}
		if err := n.assigned(ctx, change.Actor, assignment{
			householdID: appointment.HouseholdID,
			recipientID: id,
			subjectType: store.SubjectAppointment,
			subjectID:   appointment.ID,
			noun:        "Appointment",
			name:        appointment.Title,
		}); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
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
	title := fmt.Sprintf("You were assigned %s", withArticle(a.noun))
	if actor != nil {
		title = fmt.Sprintf("%s assigned you %s", actor.Name, withArticle(a.noun))
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

func withArticle(noun string) string {
	if noun != "" && strings.ContainsRune("AEIOU", rune(noun[0])) {
		return "an " + noun
	}
	return "a " + noun
}

// notify stores a Notification and hands it to the Delivery, unless the
// recipient turned Push off for its kind. The Notification is stored either
// way, so it still shows in the bell and counts towards Unread.
func (n *Notifier) notify(ctx context.Context, input store.NotificationInput) error {
	input.CreatedAt = n.clock.Now()
	stored, err := n.store.CreateNotification(ctx, input)
	if err != nil {
		return fmt.Errorf("store %s notification: %w", input.Kind, err)
	}
	settings, err := n.store.PushSettings(ctx, stored.HouseholdID, stored.RecipientID)
	if err != nil {
		return fmt.Errorf("load push settings for %s notification: %w", input.Kind, err)
	}
	if settings.Allows(stored.Kind) {
		n.delivery.Deliver(ctx, stored)
	}
	return nil
}

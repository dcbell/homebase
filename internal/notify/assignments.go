package notify

import (
	"context"

	"homebase/internal/store"
)

// ListItemChange describes a List Item that was created or updated.
type ListItemChange struct {
	// Actor is the Member who made the change.
	Actor *store.User
	// Before is the List Item as it was, or nil when it was just created.
	Before *store.ListItem
	// After is the List Item as it is now.
	After store.ListItem
}

// ListItemChanged is called after every List Item create or update.
func (n *Notifier) ListItemChanged(ctx context.Context, change ListItemChange) error {
	item := change.After
	var before *int64
	if change.Before != nil {
		before = change.Before.AssignedTo
	}
	if !newlyAssigned(before, item.AssignedTo) {
		return nil
	}
	return n.assigned(ctx, change.Actor, assignment{
		householdID: item.HouseholdID,
		recipientID: *item.AssignedTo,
		subjectType: store.SubjectListItem,
		subjectID:   item.ID,
		noun:        "List Item",
		name:        item.Title,
	})
}

// RoutineChange describes a Routine that was created or updated.
type RoutineChange struct {
	// Actor is the Member who made the change.
	Actor *store.User
	// Before is the Routine as it was, or nil when it was just created.
	Before *store.Routine
	// After is the Routine as it is now.
	After store.Routine
}

// RoutineChanged is called after every Routine create or update.
func (n *Notifier) RoutineChanged(ctx context.Context, change RoutineChange) error {
	routine := change.After
	var before *int64
	if change.Before != nil {
		before = change.Before.AssignedTo
	}
	if !newlyAssigned(before, routine.AssignedTo) {
		return nil
	}
	return n.assigned(ctx, change.Actor, assignment{
		householdID: routine.HouseholdID,
		recipientID: *routine.AssignedTo,
		subjectType: store.SubjectRoutine,
		subjectID:   routine.ID,
		noun:        "Routine",
		name:        routine.Title,
	})
}

// newlyAssigned reports whether an item now belongs to a Member it did not
// belong to before.
func newlyAssigned(before, after *int64) bool {
	if after == nil {
		return false
	}
	return before == nil || *before != *after
}

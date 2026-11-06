package api

import (
	"context"

	"homebase/internal/notify"
	"homebase/internal/store"
)

// The shared save operations change Appointments and Tasks under the same
// rules whoever calls them: the JSON API handlers, and other ways in such as
// CalDAV. Each saves the change for the acting Member in their Household and
// then does the follow-on work: scheduling Reminders and sending Assignment
// Notifications. Completing a Generated Task also advances its Routine or
// Maintenance Schedule.
//
// Errors come straight from the store: store.ErrNotFound when the item is not
// in the Household, otherwise a validation or database error. Follow-on work
// happens after the change is saved, so its failures are logged rather than
// returned.

// CreateAppointment adds an Appointment and notifies its Attendees.
func (s *Server) CreateAppointment(ctx context.Context, household store.Household, actor store.User, input store.EventInput) (store.Event, error) {
	appointment, err := s.store.CreateEvent(ctx, household.ID, actor.ID, input)
	if err != nil {
		return store.Event{}, err
	}
	s.appointmentChanged(ctx, notify.AppointmentChange{Actor: &actor, After: appointment})
	return appointment, nil
}

// UpdateAppointment changes an Appointment, rescheduling its Reminders and
// notifying Members who have just become Attendees.
func (s *Server) UpdateAppointment(ctx context.Context, household store.Household, actor store.User, id int64, input store.EventInput) (store.Event, error) {
	appointment, previous, err := s.store.UpdateEvent(ctx, household.ID, id, input)
	if err != nil {
		return store.Event{}, err
	}
	s.appointmentChanged(ctx, notify.AppointmentChange{Actor: &actor, Before: &previous, After: appointment})
	return appointment, nil
}

// ArchiveAppointment archives an Appointment and cancels its Reminders.
func (s *Server) ArchiveAppointment(ctx context.Context, household store.Household, actor store.User, id int64) error {
	if err := s.store.ArchiveEvent(ctx, household.ID, id); err != nil {
		return err
	}
	if err := s.notifier.AppointmentRemoved(ctx, household.ID, id); err != nil {
		s.logger.Error("notify appointment removed", "event_id", id, "error", err)
	}
	return nil
}

// CreateTask adds a Task, scheduling its Reminders and notifying its
// assignee.
func (s *Server) CreateTask(ctx context.Context, household store.Household, actor store.User, input store.TaskInput) (store.Task, error) {
	task, err := s.store.CreateTask(ctx, household.ID, actor.ID, input)
	if err != nil {
		return store.Task{}, err
	}
	s.taskChanged(ctx, notify.TaskChange{Actor: &actor, After: task})
	return task, nil
}

// UpdateTask changes a Task, rescheduling its Reminders and notifying a
// newly assigned Member.
func (s *Server) UpdateTask(ctx context.Context, household store.Household, actor store.User, id int64, input store.TaskInput) (store.Task, error) {
	task, previous, err := s.store.UpdateTask(ctx, household.ID, id, input)
	if err != nil {
		return store.Task{}, err
	}
	s.taskChanged(ctx, notify.TaskChange{Actor: &actor, Before: &previous, After: task})
	return task, nil
}

// CompleteTask completes a Task, cancelling its Reminders. Completing a
// Generated Task advances its Routine or Maintenance Schedule.
func (s *Server) CompleteTask(ctx context.Context, household store.Household, actor store.User, id int64) (store.Task, error) {
	task, err := s.store.CompleteTask(ctx, household.ID, id)
	if err != nil {
		return store.Task{}, err
	}
	s.taskStatusChanged(ctx, task)
	return task, nil
}

// ReopenTask reopens a Task, restoring its Reminders still ahead.
func (s *Server) ReopenTask(ctx context.Context, household store.Household, actor store.User, id int64) (store.Task, error) {
	task, err := s.store.ReopenTask(ctx, household.ID, id)
	if err != nil {
		return store.Task{}, err
	}
	s.taskStatusChanged(ctx, task)
	return task, nil
}

// SetTaskPersonalReminderTimes sets the acting Member's Personal Reminder
// Times on a Task assigned to them, rescheduling their Reminders. It returns
// store.ErrForbidden when they aren't its assignee.
func (s *Server) SetTaskPersonalReminderTimes(ctx context.Context, household store.Household, actor store.User, id int64, times []store.ReminderTime) (store.Task, error) {
	if err := s.store.SetTaskPersonalReminderTimes(ctx, household.ID, id, actor.ID, times); err != nil {
		return store.Task{}, err
	}
	return s.taskPersonalReminderTimesChanged(ctx, household, actor, id)
}

// UseSharedTaskReminderTimes removes the acting Member's Personal Reminder
// Times on a Task assigned to them, so its shared Reminder Times apply to
// them again. It returns store.ErrForbidden when they aren't its assignee.
func (s *Server) UseSharedTaskReminderTimes(ctx context.Context, household store.Household, actor store.User, id int64) (store.Task, error) {
	if err := s.store.RemoveTaskPersonalReminderTimes(ctx, household.ID, id, actor.ID); err != nil {
		return store.Task{}, err
	}
	return s.taskPersonalReminderTimesChanged(ctx, household, actor, id)
}

// taskPersonalReminderTimesChanged reschedules a Task's Reminders after its
// assignee changed their Personal Reminder Times, and returns the Task as
// they see it.
func (s *Server) taskPersonalReminderTimesChanged(ctx context.Context, household store.Household, actor store.User, id int64) (store.Task, error) {
	task, err := s.taskFor(ctx, household, actor, id)
	if err != nil {
		return store.Task{}, err
	}
	if err := s.notifier.TaskPersonalReminderTimesChanged(ctx, task); err != nil {
		s.logger.Error("notify task personal reminder times changed", "task_id", id, "error", err)
	}
	return task, nil
}

// taskFor loads a Task as the acting Member sees it: with their own Personal
// Reminder Times when they are its assignee, who alone may set them.
func (s *Server) taskFor(ctx context.Context, household store.Household, actor store.User, id int64) (store.Task, error) {
	task, err := s.store.GetTask(ctx, household.ID, id)
	if err != nil {
		return store.Task{}, err
	}
	if task.AssignedTo == nil || *task.AssignedTo != actor.ID {
		return task, nil
	}
	subject := store.ReminderSubject{HouseholdID: household.ID, Type: store.SubjectTask, ID: id}
	personal, err := s.store.PersonalReminderTimesOf(ctx, subject, actor.ID)
	if err != nil {
		return store.Task{}, err
	}
	task.PersonalReminderTimes = &personal
	return task, nil
}

// ArchiveTask archives a Task and cancels its Reminders.
func (s *Server) ArchiveTask(ctx context.Context, household store.Household, actor store.User, id int64) error {
	if err := s.store.ArchiveTask(ctx, household.ID, id); err != nil {
		return err
	}
	if task, err := s.store.GetTask(ctx, household.ID, id); err != nil {
		s.logger.Error("load archived task", "task_id", id, "error", err)
	} else {
		s.taskStatusChanged(ctx, task)
	}
	return nil
}

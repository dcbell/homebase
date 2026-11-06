package notify

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"homebase/internal/store"
)

// reminderBatchSize is how many Reminders the runner claims at a time.
const reminderBatchSize = 100

// reminderRenderer turns a claimed Reminder into the Notification to store.
// It returns ok=false to drop a Reminder that no longer applies to its
// subject's current state.
type reminderRenderer func(ctx context.Context, r store.ScheduledReminder) (input store.NotificationInput, ok bool, err error)

// scheduleReminders makes the subject's Reminders one per Reminder Time in
// times for each recipient at due. With no recipients or no due time, every
// pending Reminder for the subject is cancelled. Reminders whose time has
// already passed are skipped, not sent late.
func (n *Notifier) scheduleReminders(ctx context.Context, subject store.ReminderSubject, recipients []int64, due *time.Time, times []store.ReminderTime) error {
	want := []store.ReminderInput{}
	if due != nil {
		recipients, err := n.members(ctx, subject.HouseholdID, recipients)
		if err != nil {
			return fmt.Errorf("schedule %s %d reminders: %w", subject.Type, subject.ID, err)
		}
		for _, recipientID := range recipients {
			for _, reminderTime := range times {
				want = append(want, store.ReminderInput{
					RecipientID:  recipientID,
					ReminderTime: reminderTime,
					DueAt:        *due,
				})
			}
		}
	}
	if err := n.store.ScheduleReminders(ctx, subject, want, n.clock.Now()); err != nil {
		return fmt.Errorf("schedule %s %d reminders: %w", subject.Type, subject.ID, err)
	}
	return nil
}

// members keeps the ids that belong to current Members of the household, so
// nothing is scheduled for someone who was removed but is still named on a
// Task or Appointment.
func (n *Notifier) members(ctx context.Context, householdID int64, ids []int64) ([]int64, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	list, err := n.store.ListMembers(ctx, householdID)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	current := make(map[int64]bool, len(list))
	for _, m := range list {
		current[m.ID] = true
	}
	kept := make([]int64, 0, len(ids))
	for _, id := range ids {
		if current[id] {
			kept = append(kept, id)
		}
	}
	return kept, nil
}

// TaskStatusChanged is called after a Task is completed, reopened or
// archived. Completing or archiving cancels its pending Reminders; reopening
// restores those still ahead.
func (n *Notifier) TaskStatusChanged(ctx context.Context, task store.Task) error {
	return n.taskReminders(ctx, task)
}

// ProjectChanged is called after a Project is updated or archived. Tasks of
// an archived Project get no Reminders; restoring the Project restores those
// still ahead. The Tasks' own status is untouched.
func (n *Notifier) ProjectChanged(ctx context.Context, householdID, projectID int64) error {
	tasks, err := n.store.ListProjectTasks(ctx, householdID, projectID)
	if err != nil {
		return fmt.Errorf("list project %d tasks for reminders: %w", projectID, err)
	}
	var errs []error
	for _, task := range tasks {
		if err := n.taskReminders(ctx, task); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// taskReminders keeps a Task's Reminders in line with its current state: an
// open Task with a due date and an assignee, outside any archived Project,
// has them, one per Reminder Time of the Task; any other Task has none.
func (n *Notifier) taskReminders(ctx context.Context, task store.Task) error {
	active, err := n.taskActive(ctx, task)
	if err != nil {
		return err
	}
	var recipients []int64
	if active && task.AssignedTo != nil {
		recipients = []int64{*task.AssignedTo}
	}
	subject := store.ReminderSubject{HouseholdID: task.HouseholdID, Type: store.SubjectTask, ID: task.ID}
	return n.scheduleReminders(ctx, subject, recipients, task.DueAt, task.ReminderTimes)
}

// taskActive reports whether a Task is still something to be reminded of:
// open, and not part of an archived Project.
func (n *Notifier) taskActive(ctx context.Context, task store.Task) (bool, error) {
	if task.Status != "open" {
		return false, nil
	}
	if task.ProjectID == nil {
		return true, nil
	}
	project, err := n.store.GetProject(ctx, task.HouseholdID, *task.ProjectID)
	if errors.Is(err, store.ErrNotFound) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("load project %d of task %d: %w", *task.ProjectID, task.ID, err)
	}
	return project.Status != "archived", nil
}

// SendDueReminders sends every Reminder due as of asOf and returns how many
// Notifications it created. Each Reminder is claimed atomically before it is
// sent, so running it again, or on two instances at once, sends nothing twice.
//
// A Reminder whose Notification could not be created (a database error, an
// expired deadline) is released at the end of the run, so the next run tries
// it again. A Reminder its renderer drops as no longer applying stays used.
func (n *Notifier) SendDueReminders(ctx context.Context, asOf time.Time) (sent int, err error) {
	var errs []error
	var failed []int64
	defer func() {
		if len(failed) == 0 {
			return
		}
		// Release even when ctx is what failed, or the Reminders are lost.
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		if releaseErr := n.store.ReleaseReminders(releaseCtx, failed); releaseErr != nil {
			err = errors.Join(err, fmt.Errorf("release %d failed reminders: %w", len(failed), releaseErr))
		}
	}()
	for {
		claimed, err := n.store.ClaimDueReminders(ctx, asOf, reminderBatchSize)
		if err != nil {
			return sent, errors.Join(append(errs, fmt.Errorf("claim due reminders: %w", err))...)
		}
		sort.Slice(claimed, func(i, j int) bool {
			if !claimed[i].SendAt.Equal(claimed[j].SendAt) {
				return claimed[i].SendAt.Before(claimed[j].SendAt)
			}
			return claimed[i].ID < claimed[j].ID
		})
		for _, r := range claimed {
			created, err := n.sendReminder(ctx, r)
			if created {
				sent++
			}
			if err != nil {
				errs = append(errs, err)
				if !created {
					failed = append(failed, r.ID)
				}
			}
		}
		if len(claimed) < reminderBatchSize {
			return sent, errors.Join(errs...)
		}
	}
}

// sendReminder creates the Notification for one claimed Reminder. created
// reports whether it was stored, even if a later step failed.
func (n *Notifier) sendReminder(ctx context.Context, r store.ScheduledReminder) (created bool, err error) {
	render, found := n.reminderRenderers[r.SubjectType]
	if !found {
		return false, fmt.Errorf("no reminder renderer for %s", r.SubjectType)
	}
	input, ok, err := render(ctx, r)
	if err != nil || !ok {
		return false, err
	}
	input.HouseholdID = r.HouseholdID
	input.RecipientID = r.RecipientID
	input.Stage = r.ReminderTime.Stage()
	input.SubjectType = r.SubjectType
	input.SubjectID = r.SubjectID
	return n.notify(ctx, input)
}

// taskReminderTitle says how far a Task's due time is from its Reminder,
// such as "Task due in 24 hours", "Task due now" or "Task overdue by 1 day".
func taskReminderTitle(r store.ReminderTime) string {
	switch r.Stage() {
	case store.ReminderDueSoon:
		return "Task due in " + reminderDistance(r)
	case store.ReminderOverdue:
		return "Task overdue by " + reminderDistance(r)
	}
	if r.IsDateOnly() {
		return "Task due today"
	}
	return "Task due now"
}

// appointmentReminderTitle says how soon an Appointment starts, such as
// "Appointment in 1 hour" or "Appointment starting now".
func appointmentReminderTitle(r store.ReminderTime) string {
	if r.Stage() == store.ReminderDueNow {
		return "Appointment starting now"
	}
	return "Appointment in " + reminderDistance(r)
}

// reminderDistance is how far a Reminder Time is from the due time, before or
// after: whole days for the date-only form; for the timed form, days when it
// is a whole number of them beyond one, otherwise hours and minutes, so 24
// hours stays "24 hours".
func reminderDistance(r store.ReminderTime) string {
	if r.IsDateOnly() {
		return plural(abs(r.Days), "day")
	}
	minutes := abs(r.Minutes)
	const day = 24 * 60
	switch {
	case minutes > day && minutes%day == 0:
		return plural(minutes/day, "day")
	case minutes%60 == 0:
		return plural(minutes/60, "hour")
	case minutes < 60:
		return plural(minutes, "minute")
	}
	return plural(minutes/60, "hour") + " " + plural(minutes%60, "minute")
}

func plural(n int, unit string) string {
	if n == 1 {
		return "1 " + unit
	}
	return fmt.Sprintf("%d %ss", n, unit)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// renderTaskReminder builds a Task Reminder from the Task as it is now. It
// drops the Reminder if the Task has since been finished, reassigned or
// rescheduled, or its Project archived, as a guard behind cancellation.
func (n *Notifier) renderTaskReminder(ctx context.Context, r store.ScheduledReminder) (store.NotificationInput, bool, error) {
	task, err := n.store.GetTask(ctx, r.HouseholdID, r.SubjectID)
	if errors.Is(err, store.ErrNotFound) {
		return store.NotificationInput{}, false, nil
	}
	if err != nil {
		return store.NotificationInput{}, false, fmt.Errorf("load task %d for reminder: %w", r.SubjectID, err)
	}
	if task.AssignedTo == nil || *task.AssignedTo != r.RecipientID || task.DueAt == nil || !task.DueAt.Equal(r.DueAt) {
		return store.NotificationInput{}, false, nil
	}
	active, err := n.taskActive(ctx, task)
	if err != nil || !active {
		return store.NotificationInput{}, false, err
	}
	return store.NotificationInput{
		Kind:  store.NotificationTaskReminder,
		Title: taskReminderTitle(r.ReminderTime),
		Body:  task.Title,
	}, true, nil
}

// appointmentReminders keeps an Appointment's Reminders in line with its
// start time, Attendees and Reminder Times. An archived Appointment has none.
func (n *Notifier) appointmentReminders(ctx context.Context, appointment store.Event) error {
	subject := store.ReminderSubject{HouseholdID: appointment.HouseholdID, Type: store.SubjectAppointment, ID: appointment.ID}
	if appointment.Status == "archived" {
		return n.scheduleReminders(ctx, subject, nil, nil, nil)
	}
	recipients, err := n.appointmentRecipients(ctx, appointment)
	if err != nil {
		return err
	}
	starts := appointment.StartsAt
	return n.scheduleReminders(ctx, subject, recipients, &starts, appointment.ReminderTimes)
}

// AppointmentRemoved is called after an Appointment is archived. It cancels
// the Appointment's pending Reminders.
func (n *Notifier) AppointmentRemoved(ctx context.Context, householdID, appointmentID int64) error {
	subject := store.ReminderSubject{HouseholdID: householdID, Type: store.SubjectAppointment, ID: appointmentID}
	return n.scheduleReminders(ctx, subject, nil, nil, nil)
}

// MembersChanged is called after a Member joins or leaves the household. An
// Appointment with no Attendees is for whoever is a Member, so the Reminders
// of every such Appointment still ahead are rescheduled for the current
// Members: a newcomer gets those still to come, and a leaver gets none.
func (n *Notifier) MembersChanged(ctx context.Context, householdID int64) error {
	appointments, err := n.store.ListWholeHouseholdEventsAfter(ctx, householdID, n.clock.Now())
	if err != nil {
		return fmt.Errorf("list whole-household appointments: %w", err)
	}
	var errs []error
	for _, appointment := range appointments {
		if err := n.appointmentReminders(ctx, appointment); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// appointmentRecipients are the Members an Appointment's Reminders go to: its
// Attendees, or every Member when it has none.
func (n *Notifier) appointmentRecipients(ctx context.Context, appointment store.Event) ([]int64, error) {
	if len(appointment.AttendeeIDs) > 0 {
		return appointment.AttendeeIDs, nil
	}
	members, err := n.store.ListMembers(ctx, appointment.HouseholdID)
	if err != nil {
		return nil, fmt.Errorf("list members for appointment %d reminders: %w", appointment.ID, err)
	}
	ids := make([]int64, 0, len(members))
	for _, m := range members {
		ids = append(ids, m.ID)
	}
	return ids, nil
}

// renderAppointmentReminder builds an Appointment Reminder from the
// Appointment as it is now. It drops the Reminder if the Appointment has since
// been archived or rescheduled, as a guard behind cancellation.
func (n *Notifier) renderAppointmentReminder(ctx context.Context, r store.ScheduledReminder) (store.NotificationInput, bool, error) {
	appointment, err := n.store.GetEvent(ctx, r.HouseholdID, r.SubjectID)
	if errors.Is(err, store.ErrNotFound) {
		return store.NotificationInput{}, false, nil
	}
	if err != nil {
		return store.NotificationInput{}, false, fmt.Errorf("load appointment %d for reminder: %w", r.SubjectID, err)
	}
	if appointment.Status == "archived" || !appointment.StartsAt.Equal(r.DueAt) {
		return store.NotificationInput{}, false, nil
	}
	return store.NotificationInput{
		Kind:  store.NotificationAppointmentReminder,
		Title: appointmentReminderTitle(r.ReminderTime),
		Body:  appointment.Title,
	}, true, nil
}

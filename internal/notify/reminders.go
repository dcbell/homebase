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

// ReminderStage is one Reminder a subject gets, and when it is sent relative
// to the subject's due time.
type ReminderStage struct {
	Stage  string
	SendAt func(due time.Time) time.Time
}

// TaskReminderStages are a Task's Reminders: Due Soon 24h before, Due Now at
// the due time, and Overdue at 08:00 APP_TIMEZONE on the first morning after
// the due date.
var TaskReminderStages = []ReminderStage{
	{Stage: store.ReminderDueSoon, SendAt: before(24 * time.Hour)},
	{Stage: store.ReminderDueNow, SendAt: before(0)},
	{Stage: store.ReminderOverdue, SendAt: overdueMorning},
}

func before(d time.Duration) func(time.Time) time.Time {
	return func(due time.Time) time.Time { return due.Add(-d) }
}

// overdueMorning is 08:00 on the calendar day after due. The day is the one
// the calendar shows the due time on: the stored instant read in time.Local,
// which the API sets to APP_TIMEZONE.
func overdueMorning(due time.Time) time.Time {
	local := due.In(time.Local)
	return time.Date(local.Year(), local.Month(), local.Day()+1, 8, 0, 0, 0, time.Local)
}

// reminderRenderer turns a claimed Reminder into the Notification to store.
// It returns ok=false to drop a Reminder that no longer applies to its
// subject's current state.
type reminderRenderer func(ctx context.Context, r store.ScheduledReminder) (input store.NotificationInput, ok bool, err error)

// scheduleReminders makes the subject's Reminders those of stages for each
// recipient at due. With no recipients or no due time, every pending Reminder
// for the subject is cancelled. Reminders whose time has already passed are
// skipped, not sent late.
func (n *Notifier) scheduleReminders(ctx context.Context, subject store.ReminderSubject, recipients []int64, due *time.Time, stages []ReminderStage) error {
	want := []store.ReminderInput{}
	if due != nil {
		for _, recipientID := range recipients {
			for _, stage := range stages {
				want = append(want, store.ReminderInput{
					RecipientID: recipientID,
					Stage:       stage.Stage,
					DueAt:       *due,
					SendAt:      stage.SendAt(*due),
				})
			}
		}
	}
	if err := n.store.ScheduleReminders(ctx, subject, want, n.clock.Now()); err != nil {
		return fmt.Errorf("schedule %s %d reminders: %w", subject.Type, subject.ID, err)
	}
	return nil
}

// TaskStatusChanged is called after a Task is completed, reopened or
// archived. Completing or archiving cancels its pending Reminders; reopening
// restores those still ahead.
func (n *Notifier) TaskStatusChanged(ctx context.Context, task store.Task) error {
	return n.taskReminders(ctx, task)
}

// taskReminders keeps a Task's Reminders in line with its current state: an
// open Task with a due date and an assignee has them; any other Task has none.
func (n *Notifier) taskReminders(ctx context.Context, task store.Task) error {
	var recipients []int64
	if task.Status == "open" && task.AssignedTo != nil {
		recipients = []int64{*task.AssignedTo}
	}
	subject := store.ReminderSubject{HouseholdID: task.HouseholdID, Type: store.SubjectTask, ID: task.ID}
	return n.scheduleReminders(ctx, subject, recipients, task.DueAt, TaskReminderStages)
}

// SendDueReminders sends every Reminder due as of asOf and returns how many
// Notifications it created. Each Reminder is claimed atomically before it is
// sent, so running it again, or on two instances at once, sends nothing twice.
func (n *Notifier) SendDueReminders(ctx context.Context, asOf time.Time) (int, error) {
	sent := 0
	var errs []error
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
			ok, err := n.sendReminder(ctx, r)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if ok {
				sent++
			}
		}
		if len(claimed) < reminderBatchSize {
			return sent, errors.Join(errs...)
		}
	}
}

func (n *Notifier) sendReminder(ctx context.Context, r store.ScheduledReminder) (bool, error) {
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
	input.Stage = r.Stage
	input.SubjectType = r.SubjectType
	input.SubjectID = r.SubjectID
	return true, n.notify(ctx, input)
}

var taskReminderTitles = map[string]string{
	store.ReminderDueSoon: "Task due in 24 hours",
	store.ReminderDueNow:  "Task due now",
	store.ReminderOverdue: "Task overdue",
}

// renderTaskReminder builds a Task Reminder from the Task as it is now. It
// drops the Reminder if the Task has since been finished, reassigned or
// rescheduled, as a guard behind cancellation.
func (n *Notifier) renderTaskReminder(ctx context.Context, r store.ScheduledReminder) (store.NotificationInput, bool, error) {
	task, err := n.store.GetTask(ctx, r.HouseholdID, r.SubjectID)
	if errors.Is(err, store.ErrNotFound) {
		return store.NotificationInput{}, false, nil
	}
	if err != nil {
		return store.NotificationInput{}, false, fmt.Errorf("load task %d for reminder: %w", r.SubjectID, err)
	}
	if task.Status != "open" || task.AssignedTo == nil || *task.AssignedTo != r.RecipientID || task.DueAt == nil || !task.DueAt.Equal(r.DueAt) {
		return store.NotificationInput{}, false, nil
	}
	return store.NotificationInput{
		Kind:  store.NotificationTaskReminder,
		Title: taskReminderTitles[r.Stage],
		Body:  task.Title,
	}, true, nil
}

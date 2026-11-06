package api_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"homebase/internal/apitest"
	"homebase/internal/store"
)

// generateDueTasks runs the Task generator as the background check does.
func generateDueTasks(t *testing.T, h *apitest.Harness) {
	t.Helper()
	if err := h.Server.GenerateDueTasks(context.Background()); err != nil {
		t.Fatalf("GenerateDueTasks: %v", err)
	}
}

// generatedTasksOf returns the Tasks a Routine or Maintenance Schedule has
// generated, oldest first.
func generatedTasksOf(t *testing.T, c *apitest.Client, matches func(store.Task) bool) []store.Task {
	t.Helper()
	tasks := apitest.Decode[[]store.Task](t, c.Get("/api/v1/tasks").Expect(http.StatusOK))
	generated := []store.Task{}
	for i := len(tasks) - 1; i >= 0; i-- {
		if matches(tasks[i]) {
			generated = append(generated, tasks[i])
		}
	}
	return generated
}

func TestRoutineWithCustomReminderTimesGeneratesATaskSendingExactlyThose(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	household.Owner.Post("/api/v1/routines", map[string]any{
		"title":       "Pay the council tax",
		"cadence":     "monthly",
		"next_due_at": localAt(15, 0, 0).Format(time.RFC3339),
		"assigned_to": household.Member.User.ID,
		"reminder_times": []map[string]any{
			{"days": 0, "at": "18:00"},
			{"days": 2, "at": "09:00"},
		},
	}).Expect(http.StatusCreated)

	generateDueTasks(t, h)

	sendRemindersAt(t, h, localAt(15, 17, 59))
	expectTaskReminders(t, household.Member)
	sendRemindersAt(t, h, localAt(15, 18, 0))
	expectTaskReminders(t, household.Member, "due_now: Task due today")
	sendRemindersAt(t, h, localAt(16, 9, 0))
	expectTaskReminders(t, household.Member, "due_now: Task due today")
	sendRemindersAt(t, h, localAt(17, 9, 0))
	expectTaskReminders(t, household.Member, "due_now: Task due today", "overdue: Task overdue by 2 days")
}

// dateOnlyReminderTimes formats Reminder Times as "{days at}", or
// "{n minutes}" for the timed form.
func dateOnlyReminderTimes(times []store.ReminderTime) string {
	parts := make([]string, 0, len(times))
	for _, r := range times {
		if r.IsDateOnly() {
			parts = append(parts, fmt.Sprintf("{%d %s}", r.Days, r.At))
		} else {
			parts = append(parts, fmt.Sprintf("{%d minutes}", r.Minutes))
		}
	}
	return fmt.Sprint(parts)
}

func TestEditingARoutinesReminderTimesReachesOnlyTasksGeneratedAfterwards(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	routine := apitest.Decode[store.Routine](t, household.Owner.Post("/api/v1/routines", map[string]any{
		"title":       "Water the plants",
		"cadence":     "weekly",
		"next_due_at": localAt(20, 0, 0).Format(time.RFC3339),
		"assigned_to": household.Member.User.ID,
	}).Expect(http.StatusCreated))
	if want := "[{-1 08:00} {0 08:00} {1 08:00}]"; dateOnlyReminderTimes(routine.ReminderTimes) != want {
		t.Errorf("new Routine Reminder Times = %v, want the date-only defaults %s", routine.ReminderTimes, want)
	}
	generate := fmt.Sprintf("/api/v1/routines/%d/generate-task", routine.ID)
	first := apitest.Decode[store.Task](t, household.Owner.Post(generate, nil).Expect(http.StatusCreated))

	path := fmt.Sprintf("/api/v1/routines/%d", routine.ID)
	edited := apitest.Decode[store.Routine](t, household.Owner.Patch(path, map[string]any{
		"title":          routine.Title,
		"cadence":        routine.Cadence,
		"next_due_at":    routine.NextDueAt.Format(time.RFC3339),
		"assigned_to":    household.Member.User.ID,
		"reminder_times": []map[string]any{{"days": -2, "at": "19:00"}},
	}).Expect(http.StatusOK))
	if want := "[{-2 19:00}]"; dateOnlyReminderTimes(edited.ReminderTimes) != want {
		t.Errorf("edited Routine Reminder Times = %v, want %s", edited.ReminderTimes, want)
	}
	if got, want := dateOnlyReminderTimes(getTask(t, household.Member, first.ID).ReminderTimes), "[{-1 08:00} {0 08:00} {1 08:00}]"; got != want {
		t.Errorf("open Generated Task Reminder Times = %s, want them unchanged at %s", got, want)
	}
	sendRemindersAt(t, h, localAt(19, 8, 0))
	expectTaskReminders(t, household.Member, "due_soon: Task due in 1 day")

	// Completing the Task moves the Routine on a week; its next Task gets the
	// new Reminder Times.
	household.Member.Patch(fmt.Sprintf("/api/v1/tasks/%d/complete", first.ID), nil).Expect(http.StatusOK)
	next := apitest.Decode[store.Task](t, household.Owner.Post(generate, nil).Expect(http.StatusCreated))
	if want := "[{-2 19:00}]"; dateOnlyReminderTimes(next.ReminderTimes) != want {
		t.Errorf("next Generated Task Reminder Times = %v, want %s", next.ReminderTimes, want)
	}
	sendRemindersAt(t, h, localAt(25, 19, 0))
	expectTaskReminders(t, household.Member, "due_soon: Task due in 1 day", "due_soon: Task due in 2 days")

	// Saving the Routine without Reminder Times, as a full replace, keeps them.
	kept := apitest.Decode[store.Routine](t, household.Owner.Patch(path, map[string]any{
		"title":       "Water the plants and the herbs",
		"cadence":     routine.Cadence,
		"next_due_at": localAt(27, 0, 0).Format(time.RFC3339),
	}).Expect(http.StatusOK))
	if want := "[{-2 19:00}]"; dateOnlyReminderTimes(kept.ReminderTimes) != want {
		t.Errorf("Routine saved without Reminder Times has %v, want them kept at %s", kept.ReminderTimes, want)
	}
}

func TestRoutineAndMaintenanceScheduleReminderTimesMustFitTheirNextDueDate(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	dateDue, timedDue := localAt(20, 0, 0).Format(time.RFC3339), at(20, 18, 0).Format(time.RFC3339)
	routine := apitest.Decode[store.Routine](t, household.Owner.Post("/api/v1/routines", map[string]any{
		"title": "Water the plants", "next_due_at": dateDue,
	}).Expect(http.StatusCreated))
	asset := apitest.Decode[store.Asset](t, household.Owner.Post("/api/v1/assets", map[string]any{
		"name": "Boiler",
	}).Expect(http.StatusCreated))
	item := apitest.Decode[store.AssetMaintenanceItem](t, household.Owner.Post(fmt.Sprintf("/api/v1/assets/%d/maintenance", asset.ID), map[string]any{
		"title": "Service the boiler", "next_due_at": dateDue,
	}).Expect(http.StatusCreated))

	cases := []struct {
		due           string
		reminderTimes []map[string]any
	}{
		{dateDue, []map[string]any{{"minutes": -60}}},
		{dateDue, []map[string]any{{"days": -1, "at": "08:00"}, {"minutes": 0}}},
		{dateDue, []map[string]any{{"days": -31, "at": "08:00"}}},
		{dateDue, []map[string]any{{"days": -1, "at": "08:00"}, {"days": -1, "at": "08:00"}}},
		{timedDue, []map[string]any{{"minutes": -31 * 24 * 60}}},
	}
	for _, c := range cases {
		household.Owner.Post("/api/v1/routines", map[string]any{
			"title": "Water the plants", "next_due_at": c.due, "reminder_times": c.reminderTimes,
		}).Expect(http.StatusBadRequest)
		household.Owner.Patch(fmt.Sprintf("/api/v1/routines/%d", routine.ID), map[string]any{
			"title": "Water the plants", "next_due_at": c.due, "reminder_times": c.reminderTimes,
		}).Expect(http.StatusBadRequest)
		household.Owner.Post(fmt.Sprintf("/api/v1/assets/%d/maintenance", asset.ID), map[string]any{
			"title": "Service the boiler", "next_due_at": c.due, "reminder_times": c.reminderTimes,
		}).Expect(http.StatusBadRequest)
		household.Owner.Patch(fmt.Sprintf("/api/v1/asset-maintenance/%d", item.ID), map[string]any{
			"title": "Service the boiler", "next_due_at": c.due, "reminder_times": c.reminderTimes,
		}).Expect(http.StatusBadRequest)
	}
}

func TestRoutineWithATimedNextDueDateUsesTheTimedForm(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	routine := apitest.Decode[store.Routine](t, household.Owner.Post("/api/v1/routines", map[string]any{
		"title":       "Take the bins out",
		"cadence":     "weekly",
		"next_due_at": at(15, 19, 0).Format(time.RFC3339),
		"assigned_to": household.Member.User.ID,
	}).Expect(http.StatusCreated))
	if want := "[{-1440 minutes} {0 minutes} {1 08:00}]"; dateOnlyReminderTimes(routine.ReminderTimes) != want {
		t.Errorf("new Routine Reminder Times = %s, want the timed Task defaults %s", dateOnlyReminderTimes(routine.ReminderTimes), want)
	}
	asset := apitest.Decode[store.Asset](t, household.Owner.Post("/api/v1/assets", map[string]any{
		"name": "Boiler",
	}).Expect(http.StatusCreated))
	item := apitest.Decode[store.AssetMaintenanceItem](t, household.Owner.Post(fmt.Sprintf("/api/v1/assets/%d/maintenance", asset.ID), map[string]any{
		"title":          "Service the boiler",
		"next_due_at":    at(25, 10, 0).Format(time.RFC3339),
		"reminder_times": []map[string]any{{"minutes": -90}},
	}).Expect(http.StatusCreated))
	if want := "[{-90 minutes}]"; dateOnlyReminderTimes(item.ReminderTimes) != want {
		t.Errorf("Maintenance Schedule Reminder Times = %s, want %s", dateOnlyReminderTimes(item.ReminderTimes), want)
	}

	household.Owner.Post(fmt.Sprintf("/api/v1/routines/%d/generate-task", routine.ID), nil).Expect(http.StatusCreated)
	sendRemindersAt(t, h, at(17, 8, 0))
	expectTaskReminders(t, household.Member, "due_now: Task due now", "overdue: Task overdue by 1 day")
}

func TestTasksAndTheirSourcesSayWhetherTheirDueDateIsDateOnly(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	timedTask := createDueTask(t, household.Owner, household.Member, at(17, 18, 0))
	datedTask := createDueTask(t, household.Owner, household.Member, localAt(17, 0, 0))
	if getTask(t, household.Owner, timedTask.ID).DateOnly || !getTask(t, household.Owner, datedTask.ID).DateOnly {
		t.Errorf("Task date_only = %v (timed), %v (date-only), want false, true", timedTask.DateOnly, datedTask.DateOnly)
	}

	timedRoutine := apitest.Decode[store.Routine](t, household.Owner.Post("/api/v1/routines", map[string]any{
		"title": "Take the bins out", "next_due_at": at(20, 19, 0).Format(time.RFC3339),
	}).Expect(http.StatusCreated))
	asset := apitest.Decode[store.Asset](t, household.Owner.Post("/api/v1/assets", map[string]any{
		"name": "Boiler",
	}).Expect(http.StatusCreated))
	datedItem := apitest.Decode[store.AssetMaintenanceItem](t, household.Owner.Post(fmt.Sprintf("/api/v1/assets/%d/maintenance", asset.ID), map[string]any{
		"title": "Service the boiler", "next_due_at": localAt(20, 0, 0).Format(time.RFC3339),
	}).Expect(http.StatusCreated))
	if timedRoutine.DateOnly || !datedItem.DateOnly {
		t.Errorf("date_only = %v (timed Routine), %v (date-only Maintenance Schedule), want false, true", timedRoutine.DateOnly, datedItem.DateOnly)
	}
}

func TestSwitchingARoutinesNextDueDateBetweenTimedAndDateOnlyResetsItsReminderTimes(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	routine := apitest.Decode[store.Routine](t, household.Owner.Post("/api/v1/routines", map[string]any{
		"title":          "Take the bins out",
		"cadence":        "weekly",
		"next_due_at":    at(20, 19, 0).Format(time.RFC3339),
		"reminder_times": []map[string]any{{"minutes": -120}},
	}).Expect(http.StatusCreated))
	path := fmt.Sprintf("/api/v1/routines/%d", routine.ID)

	// The old ones, resent, don't fit a date with no time of day.
	dated := apitest.Decode[store.Routine](t, household.Owner.Patch(path, map[string]any{
		"title": routine.Title, "cadence": routine.Cadence,
		"next_due_at":    localAt(21, 0, 0).Format(time.RFC3339),
		"reminder_times": []map[string]any{{"minutes": -120}},
	}).Expect(http.StatusOK))
	if want := "[{-1 08:00} {0 08:00} {1 08:00}]"; dateOnlyReminderTimes(dated.ReminderTimes) != want {
		t.Errorf("Reminder Times after switching to date-only = %s, want %s", dateOnlyReminderTimes(dated.ReminderTimes), want)
	}

	// Ones sent with the switch that fit the new kind are kept.
	timed := apitest.Decode[store.Routine](t, household.Owner.Patch(path, map[string]any{
		"title": routine.Title, "cadence": routine.Cadence,
		"next_due_at":    at(22, 19, 0).Format(time.RFC3339),
		"reminder_times": []map[string]any{{"minutes": -30}},
	}).Expect(http.StatusOK))
	if want := "[{-30 minutes}]"; dateOnlyReminderTimes(timed.ReminderTimes) != want {
		t.Errorf("Reminder Times after switching to timed = %s, want %s", dateOnlyReminderTimes(timed.ReminderTimes), want)
	}

	// Left out, they are the new kind's defaults.
	redated := apitest.Decode[store.Routine](t, household.Owner.Patch(path, map[string]any{
		"title": routine.Title, "cadence": routine.Cadence,
		"next_due_at": localAt(23, 0, 0).Format(time.RFC3339),
	}).Expect(http.StatusOK))
	if want := "[{-1 08:00} {0 08:00} {1 08:00}]"; dateOnlyReminderTimes(redated.ReminderTimes) != want {
		t.Errorf("Reminder Times after switching back without any = %s, want %s", dateOnlyReminderTimes(redated.ReminderTimes), want)
	}
}

func TestMaintenanceScheduleWithCustomReminderTimesGeneratesATaskSendingExactlyThose(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	asset := apitest.Decode[store.Asset](t, household.Owner.Post("/api/v1/assets", map[string]any{
		"name": "Boiler",
	}).Expect(http.StatusCreated))
	item := apitest.Decode[store.AssetMaintenanceItem](t, household.Owner.Post(fmt.Sprintf("/api/v1/assets/%d/maintenance", asset.ID), map[string]any{
		"title":          "Service the boiler",
		"cadence":        "yearly",
		"next_due_at":    localAt(15, 0, 0).Format(time.RFC3339),
		"reminder_times": []map[string]any{{"days": 0, "at": "18:00"}, {"days": 2, "at": "09:00"}},
	}).Expect(http.StatusCreated))
	if want := "[{0 18:00} {2 09:00}]"; dateOnlyReminderTimes(item.ReminderTimes) != want {
		t.Errorf("Maintenance Schedule Reminder Times = %v, want %s", item.ReminderTimes, want)
	}

	generateDueTasks(t, h)
	generated := generatedTasksOf(t, household.Owner, func(task store.Task) bool {
		return task.AssetMaintenanceItemID != nil && *task.AssetMaintenanceItemID == item.ID
	})
	if len(generated) != 1 {
		t.Fatalf("Generated Tasks = %+v, want one", generated)
	}
	// Maintenance Tasks are generated unassigned; whoever takes one on gets
	// its Reminders.
	updateTask(t, household.Owner, generated[0], map[string]any{"assigned_to": household.Member.User.ID})

	sendRemindersAt(t, h, localAt(15, 17, 59))
	expectTaskReminders(t, household.Member)
	sendRemindersAt(t, h, localAt(15, 18, 0))
	expectTaskReminders(t, household.Member, "due_now: Task due today")
	sendRemindersAt(t, h, localAt(17, 9, 0))
	expectTaskReminders(t, household.Member, "due_now: Task due today", "overdue: Task overdue by 2 days")
}

func TestEditingAMaintenanceSchedulesReminderTimesReachesOnlyTasksGeneratedAfterwards(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	asset := apitest.Decode[store.Asset](t, household.Owner.Post("/api/v1/assets", map[string]any{
		"name": "Boiler",
	}).Expect(http.StatusCreated))
	item := apitest.Decode[store.AssetMaintenanceItem](t, household.Owner.Post(fmt.Sprintf("/api/v1/assets/%d/maintenance", asset.ID), map[string]any{
		"title":       "Bleed the radiators",
		"cadence":     "monthly",
		"next_due_at": localAt(20, 0, 0).Format(time.RFC3339),
	}).Expect(http.StatusCreated))
	generate := fmt.Sprintf("/api/v1/asset-maintenance/%d/generate-task", item.ID)
	first := apitest.Decode[store.Task](t, household.Owner.Post(generate, nil).Expect(http.StatusCreated))

	path := fmt.Sprintf("/api/v1/asset-maintenance/%d", item.ID)
	edited := apitest.Decode[store.AssetMaintenanceItem](t, household.Owner.Patch(path, map[string]any{
		"title":          item.Title,
		"cadence":        item.Cadence,
		"next_due_at":    item.NextDueAt.Format(time.RFC3339),
		"reminder_times": []map[string]any{},
	}).Expect(http.StatusOK))
	if len(edited.ReminderTimes) != 0 {
		t.Errorf("edited Maintenance Schedule Reminder Times = %v, want none", edited.ReminderTimes)
	}
	if got, want := dateOnlyReminderTimes(getTask(t, household.Owner, first.ID).ReminderTimes), "[{-1 08:00} {0 08:00} {1 08:00}]"; got != want {
		t.Errorf("open Generated Task Reminder Times = %s, want them unchanged at %s", got, want)
	}

	household.Owner.Patch(fmt.Sprintf("/api/v1/tasks/%d/complete", first.ID), nil).Expect(http.StatusOK)
	next := apitest.Decode[store.Task](t, household.Owner.Post(generate, nil).Expect(http.StatusCreated))
	if next.ReminderTimes == nil || len(next.ReminderTimes) != 0 {
		t.Errorf("next Generated Task Reminder Times = %v, want none", next.ReminderTimes)
	}

	// Saving it without Reminder Times, as a full replace, keeps them.
	kept := apitest.Decode[store.AssetMaintenanceItem](t, household.Owner.Patch(path, map[string]any{
		"title": "Bleed every radiator", "cadence": item.Cadence, "next_due_at": localAt(20, 0, 0).Format(time.RFC3339),
	}).Expect(http.StatusOK))
	if kept.ReminderTimes == nil || len(kept.ReminderTimes) != 0 {
		t.Errorf("Maintenance Schedule saved without Reminder Times has %v, want them kept as none", kept.ReminderTimes)
	}
}

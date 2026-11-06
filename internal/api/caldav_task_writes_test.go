package api_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"homebase/internal/apitest"
	caldavpkg "homebase/internal/caldav"
	"homebase/internal/store"
)

// vtodo wraps VTODO lines in a calendar object, the way Reminders PUTs one.
func vtodo(lines ...string) string {
	return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Test phone//EN\r\nBEGIN:VTODO\r\n" +
		strings.Join(lines, "\r\n") + "\r\nEND:VTODO\r\nEND:VCALENDAR\r\n"
}

func getTask(t *testing.T, c *apitest.Client, id int64) store.Task {
	t.Helper()
	return apitest.Decode[store.Task](t, c.Get(fmt.Sprintf("/api/v1/tasks/%d", id)).Expect(http.StatusOK))
}

// taskTitled finds the Household's active Task with a title through the JSON
// API.
func taskTitled(t *testing.T, c *apitest.Client, title string) (store.Task, bool) {
	t.Helper()
	for _, task := range apitest.Decode[[]store.Task](t, c.Get("/api/v1/tasks").Expect(http.StatusOK)) {
		if task.Title == title {
			return task, true
		}
	}
	return store.Task{}, false
}

func TestTickingAGeneratedTaskOnThePhoneCompletesItAndAdvancesItsRoutine(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	routine := apitest.Decode[store.Routine](t, household.Owner.Post("/api/v1/routines", map[string]any{
		"title":       "Feed the sourdough starter",
		"cadence":     "daily",
		"next_due_at": "2030-01-15T08:00:00Z",
		"assigned_to": household.Member.User.ID,
	}).Expect(http.StatusCreated))
	if err := h.Server.GenerateDueTasks(context.Background()); err != nil {
		t.Fatalf("GenerateDueTasks: %v", err)
	}
	cal := household.Member.ConnectCalendar()
	generated, ok := taskTitled(t, household.Owner, "Feed the sourdough starter")
	if !ok || generated.RoutineID == nil {
		t.Fatalf("no Generated Task: %+v", generated)
	}
	uid := caldavpkg.TaskUID(generated.ID)
	path := cal.CollectionPath("tasks") + uid + ".ics"

	putObject(cal, "tasks", uid, http.Header{"If-Match": {currentETag(t, cal, path)}}, vtodo(
		"UID:"+uid,
		"DTSTAMP:20300115T120000Z",
		"SUMMARY:Feed the sourdough starter",
		"DUE:20300115T080000Z",
		"STATUS:COMPLETED",
		"COMPLETED:20300115T120000Z",
		"PERCENT-COMPLETE:100",
	)).Expect(http.StatusCreated)

	if task := getTask(t, household.Owner, generated.ID); task.Status != "done" {
		t.Errorf("status = %q after ticking it on the phone, want done", task.Status)
	}
	after := apitest.Decode[store.Routine](t, household.Owner.Get(fmt.Sprintf("/api/v1/routines/%d", routine.ID)).Expect(http.StatusOK))
	if after.NextDueAt == nil || !after.NextDueAt.After(*routine.NextDueAt) {
		t.Errorf("Routine next due %v after completing its Task, want later than %v", after.NextDueAt, routine.NextDueAt)
	}
	if _, ok := allTodos(t, cal)["Feed the sourdough starter"]; ok {
		t.Errorf("the ticked to-do is still on the list")
	}
}

func TestUntickingAToDoOnThePhoneReopensTheTask(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	task := createTask(t, household.Owner, "Mow the lawn", map[string]any{"assigned_to": household.Member.User.ID})
	cal := household.Member.ConnectCalendar()
	uid := caldavpkg.TaskUID(task.ID)
	path := cal.CollectionPath("tasks") + uid + ".ics"

	putObject(cal, "tasks", uid, http.Header{"If-Match": {currentETag(t, cal, path)}}, vtodo(
		"UID:"+uid, "DTSTAMP:20300115T120000Z", "SUMMARY:Mow the lawn",
		"STATUS:COMPLETED", "COMPLETED:20300115T120000Z",
	)).Expect(http.StatusCreated)
	if got := getTask(t, household.Owner, task.ID); got.Status != "done" {
		t.Fatalf("status = %q after ticking, want done", got.Status)
	}
	// Homebase answered with no ETag, so the phone unticks without one.
	putObject(cal, "tasks", uid, nil, vtodo(
		"UID:"+uid, "DTSTAMP:20300115T120100Z", "SUMMARY:Mow the lawn", "STATUS:NEEDS-ACTION",
	)).Expect(http.StatusCreated)

	if got := getTask(t, household.Owner, task.ID); got.Status != "open" || got.CompletedAt != nil {
		t.Errorf("status, completed = %q, %v after unticking, want open and not completed", got.Status, got.CompletedAt)
	}
	if _, ok := allTodos(t, cal)["Mow the lawn"]; !ok {
		t.Errorf("the reopened Task is missing from the list")
	}
}

func TestEditingAToDoOnThePhoneChangesTitleDueDateAndNotesOnly(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	task := createTask(t, household.Owner, "Pay the water bill", map[string]any{
		"assigned_to": household.Member.User.ID,
		"due_at":      at(17, 18, 0).Format(time.RFC3339),
		"priority":    "high",
		"notes":       "Online.",
	})
	cal := household.Member.ConnectCalendar()
	uid := caldavpkg.TaskUID(task.ID)
	path := cal.CollectionPath("tasks") + uid + ".ics"

	putObject(cal, "tasks", uid, http.Header{"If-Match": {currentETag(t, cal, path)}}, vtodo(
		"UID:"+uid, "DTSTAMP:20300115T120000Z",
		"SUMMARY:Pay the water and sewer bill",
		`DESCRIPTION:Online\, or by phone.`,
		"DUE:20300120T180000Z",
		"PRIORITY:9",
		"STATUS:NEEDS-ACTION",
	)).Expect(http.StatusCreated)

	got := getTask(t, household.Owner, task.ID)
	if got.Title != "Pay the water and sewer bill" || got.Notes != "Online, or by phone." {
		t.Errorf("title, notes = %q, %q", got.Title, got.Notes)
	}
	if got.DueAt == nil || !got.DueAt.Equal(at(20, 18, 0)) {
		t.Errorf("due = %v, want %s", got.DueAt, at(20, 18, 0))
	}
	if got.Priority != "high" || got.AssignedTo == nil || *got.AssignedTo != household.Member.User.ID || got.Status != "open" {
		t.Errorf("priority, assignee, status = %q, %v, %q; want them unchanged", got.Priority, got.AssignedTo, got.Status)
	}

	// The Reminders follow the new due date.
	sendRemindersAt(t, h, at(16, 18, 0))
	expectReminders(t, household.Member)
	sendRemindersAt(t, h, at(19, 18, 0))
	expectReminders(t, household.Member, fmt.Sprintf("due_soon %d", task.ID))
}

func TestCreatingAToDoOnThePhoneMakesAStandaloneTaskForTheMember(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	cal := household.Member.ConnectCalendar()

	putObject(cal, "tasks", "phone-todo-1", http.Header{"If-None-Match": {"*"}}, vtodo(
		"UID:phone-todo-1", "DTSTAMP:20300115T120000Z",
		`SUMMARY:Buy milk\, eggs`,
		"DUE;VALUE=DATE:20300118",
		"STATUS:NEEDS-ACTION",
	)).Expect(http.StatusCreated)

	task, ok := taskTitled(t, household.Owner, "Buy milk, eggs")
	if !ok {
		t.Fatalf("no Task titled Buy milk, eggs after the phone created one")
	}
	if task.AssignedTo == nil || *task.AssignedTo != household.Member.User.ID {
		t.Errorf("assigned to %v, want the connecting Member %d", task.AssignedTo, household.Member.User.ID)
	}
	if task.ProjectID != nil || task.RoutineID != nil || task.AssetMaintenanceItemID != nil {
		t.Errorf("Task %+v is not Standalone", task)
	}
	if want := time.Date(2030, 1, 18, 0, 0, 0, 0, time.Local); task.DueAt == nil || !task.DueAt.Equal(want) {
		t.Errorf("due = %v, want local midnight %s", task.DueAt, want)
	}
	if got := notificationsOf(t, household.Member); len(got.Notifications) != 0 {
		t.Errorf("Member got %+v, want no Assignment Notification for a self-assigned Task", got.Notifications)
	}

	raw := string(cal.Do(http.MethodGet, cal.CollectionPath("tasks")+"phone-todo-1.ics", nil, "").Expect(http.StatusOK).Body)
	if !strings.Contains(raw, "UID:phone-todo-1\r\n") {
		t.Errorf("phone-made to-do lost its UID:\n%s", raw)
	}

	// Another Member's device can't reuse the UID.
	other := household.Owner.ConnectCalendar()
	putObject(other, "tasks", "phone-todo-1", nil, vtodo(
		"UID:phone-todo-1", "DTSTAMP:20300115T120000Z", "SUMMARY:Someone else's",
	)).Expect(http.StatusForbidden)
}

func TestDeletingAToDoOnThePhoneArchivesTheTask(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	task := createTask(t, household.Owner, "Mow the lawn", nil)
	cal := household.Member.ConnectCalendar()
	path := allTodos(t, cal)["Mow the lawn"].Path

	cal.Do(http.MethodDelete, path, http.Header{"If-Match": {currentETag(t, cal, path)}}, "").Expect(http.StatusNoContent)

	if got := getTask(t, household.Owner, task.ID); got.Status != "archived" {
		t.Errorf("status = %q after the phone deleted it, want archived", got.Status)
	}
	if _, ok := allTodos(t, cal)["Mow the lawn"]; ok {
		t.Errorf("the deleted to-do is still on the list")
	}
	cal.Do(http.MethodDelete, path, nil, "").Expect(http.StatusNotFound)
}

func TestRepeatingToDosAreRefusedAlarmsDiscardedAndStaleEditsRejected(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	task := createTask(t, household.Owner, "Mow the lawn", nil)
	cal := household.Member.ConnectCalendar()
	uid := caldavpkg.TaskUID(task.ID)
	path := cal.CollectionPath("tasks") + uid + ".ics"

	resp := putObject(cal, "tasks", "weekly", nil, vtodo(
		"UID:weekly", "DTSTAMP:20300115T120000Z", "SUMMARY:Water plants",
		"DTSTART:20300120T090000Z", "DUE:20300120T090000Z", "RRULE:FREQ=WEEKLY",
	)).Expect(http.StatusForbidden)
	if !strings.Contains(string(resp.Body), "valid-calendar-object-resource") {
		t.Errorf("repeating to-do refusal has no CalDAV error: %s", resp.Body)
	}
	if _, ok := taskTitled(t, household.Owner, "Water plants"); ok {
		t.Errorf("a repeating to-do became a Task")
	}

	putObject(cal, "tasks", "with-alarm", nil, "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Test phone//EN\r\nBEGIN:VTODO\r\n"+
		"UID:with-alarm\r\nDTSTAMP:20300115T120000Z\r\nSUMMARY:Call the plumber\r\nDUE:20300120T090000Z\r\n"+
		"BEGIN:VALARM\r\nACTION:DISPLAY\r\nDESCRIPTION:Reminder\r\nTRIGGER:-PT15M\r\nEND:VALARM\r\n"+
		"END:VTODO\r\nEND:VCALENDAR\r\n").Expect(http.StatusCreated)
	if _, ok := taskTitled(t, household.Owner, "Call the plumber"); !ok {
		t.Errorf("the to-do with an alarm didn't become a Task")
	}
	raw := string(cal.Do(http.MethodGet, cal.CollectionPath("tasks")+"with-alarm.ics", nil, "").Expect(http.StatusOK).Body)
	if strings.Contains(raw, "VALARM") {
		t.Errorf("the alarm was kept:\n%s", raw)
	}

	stale := currentETag(t, cal, path)
	household.Owner.Patch(fmt.Sprintf("/api/v1/tasks/%d", task.ID), map[string]any{"title": "Mow the back lawn"}).Expect(http.StatusOK)
	putObject(cal, "tasks", uid, http.Header{"If-Match": {stale}}, vtodo(
		"UID:"+uid, "DTSTAMP:20300115T120000Z", "SUMMARY:Mow the lawn", "STATUS:COMPLETED",
	)).Expect(http.StatusPreconditionFailed)
	if got := getTask(t, household.Owner, task.ID); got.Status != "open" || got.Title != "Mow the back lawn" {
		t.Errorf("status, title = %q, %q after a stale write, want open and unchanged", got.Status, got.Title)
	}
}

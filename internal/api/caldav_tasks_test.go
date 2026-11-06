package api_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav/caldav"

	"homebase/internal/apitest"
	"homebase/internal/store"
)

// createTask creates a Task through the JSON API; fields adds to its body.
func createTask(t *testing.T, c *apitest.Client, title string, fields map[string]any) store.Task {
	t.Helper()
	body := map[string]any{"title": title}
	for k, v := range fields {
		body[k] = v
	}
	return apitest.Decode[store.Task](t, c.Post("/api/v1/tasks", body).Expect(http.StatusCreated))
}

// allTodos asks the tasks collection for every VTODO, the way Reminders
// lists it, and returns them by SUMMARY.
func allTodos(t *testing.T, cal *apitest.CalDAV) map[string]caldav.CalendarObject {
	t.Helper()
	return queryTodos(t, cal, time.Time{}, time.Time{})
}

func queryTodos(t *testing.T, cal *apitest.CalDAV, start, end time.Time) map[string]caldav.CalendarObject {
	t.Helper()
	objects, err := cal.Client.QueryCalendar(context.Background(), cal.CollectionPath("tasks"), &caldav.CalendarQuery{
		CompRequest: caldav.CalendarCompRequest{Name: "VCALENDAR", AllProps: true, AllComps: true},
		CompFilter: caldav.CompFilter{Name: "VCALENDAR", Comps: []caldav.CompFilter{
			{Name: "VTODO", Start: start, End: end},
		}},
	})
	if err != nil {
		t.Fatalf("calendar-query: %v", err)
	}
	byTitle := map[string]caldav.CalendarObject{}
	for _, o := range objects {
		byTitle[todoOf(t, o).Props.Get(ical.PropSummary).Value] = o
	}
	return byTitle
}

func todoOf(t *testing.T, o caldav.CalendarObject) *ical.Component {
	t.Helper()
	if len(o.Data.Children) != 1 || o.Data.Children[0].Name != ical.CompToDo {
		t.Fatalf("%s holds %+v, want one VTODO", o.Path, o.Data.Children)
	}
	return o.Data.Children[0]
}

func TestDiscoveryListsAHomebaseTasksListOfToDos(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	cal := household.Member.ConnectCalendar()

	calendars, err := cal.Client.FindCalendars(context.Background(), cal.HomePath())
	if err != nil {
		t.Fatalf("find calendars: %v", err)
	}
	found := false
	for _, c := range calendars {
		if c.Path != cal.CollectionPath("tasks") {
			continue
		}
		found = true
		if c.Name != "Homebase Tasks" {
			t.Errorf("tasks name = %q, want Homebase Tasks", c.Name)
		}
		if len(c.SupportedComponentSet) != 1 || c.SupportedComponentSet[0] != "VTODO" {
			t.Errorf("tasks components = %v, want only VTODO", c.SupportedComponentSet)
		}
	}
	if !found {
		t.Fatalf("calendars = %+v, want a tasks collection", calendars)
	}

	props := cal.Propfind(cal.HomePath(), "1", apitest.GetCTag, apitest.CurrentUserPrivilegeSet)[cal.CollectionPath("tasks")]
	if p := props[apitest.GetCTag]; p.Status != http.StatusOK || p.Text == "" {
		t.Errorf("tasks getctag = %+v, want a change tag", p)
	}
	if p := props[apitest.CurrentUserPrivilegeSet]; !p.Has("{DAV:}read") || p.Has("{DAV:}write") {
		t.Errorf("tasks privileges = %+v, want read only", p)
	}
}

func TestTheTasksListHoldsTheOpenTasksThatConcernTheMember(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	me := household.Member.User.ID
	soon := apitest.DefaultNow.Add(48 * time.Hour).Format(time.RFC3339)

	createTask(t, household.Owner, "Mine", map[string]any{"assigned_to": me, "due_at": soon})
	createTask(t, household.Owner, "Unassigned", map[string]any{"due_at": soon})
	createTask(t, household.Owner, "Undated", map[string]any{"assigned_to": me})
	createTask(t, household.Owner, "Someone else's", map[string]any{"assigned_to": household.Owner.User.ID, "due_at": soon})
	done := createTask(t, household.Owner, "Done", map[string]any{"assigned_to": me})
	household.Owner.Patch(fmt.Sprintf("/api/v1/tasks/%d/complete", done.ID), nil).Expect(http.StatusOK)
	archived := createTask(t, household.Owner, "Archived", map[string]any{"assigned_to": me})
	household.Owner.Post(fmt.Sprintf("/api/v1/tasks/%d/archive", archived.ID), nil).Expect(http.StatusOK)
	elsewhere := h.SeedHousehold()
	createTask(t, elsewhere.Owner, "Another Household's", nil)

	cal := household.Member.ConnectCalendar()

	got := titles(allTodos(t, cal))
	want := []string{"Mine", "Unassigned", "Undated"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("tasks holds %q, want %q", got, want)
	}
}

func TestTheTasksListCoversDueDates90DaysBackTo12MonthsAhead(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	// The fake Clock says 2030-01-15 12:00 UTC.
	windowStart := time.Date(2029, time.October, 17, 12, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2031, time.January, 15, 12, 0, 0, 0, time.UTC)
	due := func(at time.Time) map[string]any { return map[string]any{"due_at": at.Format(time.RFC3339)} }

	createTask(t, household.Owner, "Due before the window", due(windowStart.Add(-time.Hour)))
	createTask(t, household.Owner, "Due at the window's start", due(windowStart.Add(time.Hour)))
	createTask(t, household.Owner, "Due near the window's end", due(windowEnd.Add(-time.Hour)))
	createTask(t, household.Owner, "Due after the window", due(windowEnd.Add(time.Hour)))
	createTask(t, household.Owner, "Undated", nil)

	cal := household.Member.ConnectCalendar()

	got := titles(allTodos(t, cal))
	want := []string{"Due at the window's start", "Due near the window's end", "Undated"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("tasks holds %q, want %q", got, want)
	}

	h.Clock.Advance(3 * time.Hour)
	got = titles(allTodos(t, cal))
	want = []string{"Due after the window", "Due near the window's end", "Undated"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("3 hours later tasks holds %q, want %q", got, want)
	}
}

func TestRemindersCanFilterTasksByDueDateAndStillSeeUndatedOnes(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	day := time.Date(2030, time.February, 10, 9, 0, 0, 0, time.UTC)
	createTask(t, household.Owner, "Monday", map[string]any{"due_at": day.Format(time.RFC3339)})
	createTask(t, household.Owner, "Tuesday", map[string]any{"due_at": day.AddDate(0, 0, 1).Format(time.RFC3339)})
	createTask(t, household.Owner, "Wednesday", map[string]any{"due_at": day.AddDate(0, 0, 2).Format(time.RFC3339)})
	createTask(t, household.Owner, "Someday", nil)
	cal := household.Member.ConnectCalendar()

	got := titles(queryTodos(t, cal, day.AddDate(0, 0, 1), day.AddDate(0, 0, 2)))
	if strings.Join(got, "|") != "Someday|Tuesday" {
		t.Errorf("time-range query = %q, want Tuesday and the undated Someday", got)
	}
}

func TestATaskInRemindersHasItsNotesPriorityAndALinkBack(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	task := createTask(t, household.Owner, "Call the plumber", map[string]any{
		"notes":    "Ask about the boiler too.",
		"priority": "high",
		"due_at":   "2030-01-20T09:30:00Z",
	})
	cal := household.Member.ConnectCalendar()

	object := allTodos(t, cal)["Call the plumber"]
	raw := string(cal.Do(http.MethodGet, object.Path, nil, "").Expect(http.StatusOK).Body)
	for _, line := range []string{
		"BEGIN:VTODO",
		"SUMMARY:Call the plumber",
		"DUE:20300120T093000Z",
		"PRIORITY:1",
		"STATUS:NEEDS-ACTION",
		"DESCRIPTION:Ask about the boiler too.",
		fmt.Sprintf("URL:http://homebase.test/tasks/%d", task.ID),
	} {
		if !strings.Contains(raw, line+"\r\n") {
			t.Errorf("calendar data has no %q line:\n%s", line, raw)
		}
	}
	if strings.Contains(raw, "BEGIN:VALARM") {
		t.Errorf("calendar data has an alarm:\n%s", raw)
	}
}

func TestTaskPrioritiesMapToHighestMediumAndLowest(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	for _, priority := range []string{"high", "normal", "low"} {
		createTask(t, household.Owner, priority, map[string]any{"priority": priority})
	}
	cal := household.Member.ConnectCalendar()

	todos := allTodos(t, cal)
	for priority, want := range map[string]string{"high": "1", "normal": "5", "low": "9"} {
		if p := todoOf(t, todos[priority]).Props.Get(ical.PropPriority); p == nil || p.Value != want {
			t.Errorf("%s priority = %+v, want PRIORITY %s", priority, p, want)
		}
	}
}

func TestATaskDueAtLocalMidnightIsDueOnADate(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	midnight := time.Date(2030, time.January, 21, 0, 0, 0, 0, time.Local)
	createTask(t, household.Owner, "Bins out", map[string]any{"due_at": midnight.Format(time.RFC3339)})
	createTask(t, household.Owner, "Dentist call", map[string]any{"due_at": midnight.Add(9 * time.Hour).Format(time.RFC3339)})
	cal := household.Member.ConnectCalendar()

	todos := allTodos(t, cal)
	if p := todoOf(t, todos["Bins out"]).Props.Get(ical.PropDue); p == nil || p.Value != "20300121" || p.ValueType() != ical.ValueDate {
		t.Errorf("midnight DUE = %+v, want the date 20300121", p)
	}
	if p := todoOf(t, todos["Dentist call"]).Props.Get(ical.PropDue); p == nil || p.ValueType() == ical.ValueDate || !strings.HasSuffix(p.Value, "Z") {
		t.Errorf("timed DUE = %+v, want a UTC time", p)
	}
}

func TestCompletingATaskInHomebaseTakesItOffTheListAndChangesTheChangeTag(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	task := createTask(t, household.Owner, "Mow the lawn", nil)
	createTask(t, household.Owner, "Wash the car", nil)
	cal := household.Member.ConnectCalendar()
	ctag := func() string {
		return cal.Propfind(cal.CollectionPath("tasks"), "0", apitest.GetCTag)[cal.CollectionPath("tasks")][apitest.GetCTag].Text
	}

	path := allTodos(t, cal)["Mow the lawn"].Path
	before := ctag()
	h.Clock.Advance(time.Minute)
	if again := ctag(); again != before {
		t.Fatalf("change tag moved from %q to %q with the Clock alone", before, again)
	}

	household.Member.Patch(fmt.Sprintf("/api/v1/tasks/%d/complete", task.ID), nil).Expect(http.StatusOK)

	if after := ctag(); after == "" || after == before {
		t.Errorf("change tag = %q after completing a Task, was %q; want a new one", after, before)
	}
	if got := titles(allTodos(t, cal)); strings.Join(got, "|") != "Wash the car" {
		t.Errorf("tasks holds %q after completing Mow the lawn, want only Wash the car", got)
	}
	cal.Do(http.MethodGet, path, nil, "").Expect(http.StatusNotFound)
}

func TestGeneratedTasksFromRoutinesAppearInTheTasksList(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	household.Owner.Post("/api/v1/routines", map[string]any{
		"title":       "Feed the sourdough starter",
		"cadence":     "daily",
		"next_due_at": "2030-01-15T08:00:00Z",
		"assigned_to": household.Member.User.ID,
	}).Expect(http.StatusCreated)
	if err := h.Server.GenerateDueTasks(context.Background()); err != nil {
		t.Fatalf("GenerateDueTasks: %v", err)
	}
	cal := household.Member.ConnectCalendar()

	if _, ok := allTodos(t, cal)["Feed the sourdough starter"]; !ok {
		t.Errorf("tasks holds %q, want the Generated Task", titles(allTodos(t, cal)))
	}
}

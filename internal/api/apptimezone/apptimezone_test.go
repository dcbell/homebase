// Package apptimezone_test runs CalDAV tests with APP_TIMEZONE set to
// America/Chicago instead of UTC, so dates and local midnights can't pass by
// coinciding with UTC ones.
//
// APP_TIMEZONE is time.Local, which is global to the process, so these tests
// live in their own package (go test runs each package as its own process)
// and TestMain sets it before any test runs. Tests here may run in parallel
// with each other: they all share the one zone.
package apptimezone_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav/caldav"

	"homebase/internal/apitest"
	"homebase/internal/config"
	"homebase/internal/store"
)

const zone = "America/Chicago"

func TestMain(m *testing.M) {
	if err := (config.Config{Timezone: zone}).ApplyTimezone(); err != nil {
		fmt.Fprintln(os.Stderr, "apply APP_TIMEZONE:", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// localMidnight is the start of a day in APP_TIMEZONE. In Chicago it is
// 06:00 UTC in winter.
func localMidnight(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local)
}

// objects lists a collection by SUMMARY, the way a phone does.
func objects(t *testing.T, cal *apitest.CalDAV, collection, component string) map[string]*ical.Component {
	t.Helper()
	found, err := cal.Client.QueryCalendar(context.Background(), cal.CollectionPath(collection), &caldav.CalendarQuery{
		CompRequest: caldav.CalendarCompRequest{Name: "VCALENDAR", AllProps: true, AllComps: true},
		CompFilter:  caldav.CompFilter{Name: "VCALENDAR", Comps: []caldav.CompFilter{{Name: component}}},
	})
	if err != nil {
		t.Fatalf("calendar-query %s: %v", collection, err)
	}
	bySummary := map[string]*ical.Component{}
	for _, o := range found {
		for _, child := range o.Data.Children {
			if child.Name == component {
				summary, _ := child.Props.Text(ical.PropSummary)
				bySummary[summary] = child
			}
		}
	}
	return bySummary
}

func expectDate(t *testing.T, what string, comp *ical.Component, prop, want string) {
	t.Helper()
	if comp == nil {
		t.Errorf("%s: not on the calendar", what)
		return
	}
	if p := comp.Props.Get(prop); p == nil || p.Value != want || p.ValueType() != ical.ValueDate {
		t.Errorf("%s: %s = %+v, want the date %s", what, prop, p, want)
	}
}

func put(cal *apitest.CalDAV, collection, uid, component string, lines ...string) *apitest.Response {
	ics := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Test phone//EN\r\nBEGIN:" + component + "\r\n" +
		strings.Join(lines, "\r\n") + "\r\nEND:" + component + "\r\nEND:VCALENDAR\r\n"
	return cal.Do(http.MethodPut, cal.CollectionPath(collection)+uid+".ics",
		http.Header{"Content-Type": {"text/calendar; charset=utf-8"}}, ics)
}

func TestATaskDueAtLocalMidnightIsDueOnThatLocalDate(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	household.Owner.Post("/api/v1/tasks", map[string]any{
		"title":  "Renew the passport",
		"due_at": localMidnight(2030, time.January, 18).Format(time.RFC3339),
	}).Expect(http.StatusCreated)
	household.Owner.Post("/api/v1/tasks", map[string]any{
		"title":  "Due at UTC midnight",
		"due_at": time.Date(2030, time.January, 18, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
	}).Expect(http.StatusCreated)
	cal := household.Member.ConnectCalendar()

	todos := objects(t, cal, "tasks", ical.CompToDo)
	expectDate(t, "local-midnight Task", todos["Renew the passport"], ical.PropDue, "20300118")
	if p := todos["Due at UTC midnight"].Props.Get(ical.PropDue); p == nil || p.Value != "20300118T000000Z" {
		t.Errorf("UTC-midnight Task DUE = %+v, want the instant 20300118T000000Z (18:00 the day before in Chicago)", p)
	}
}

func TestAToDoDueOnADateFromThePhoneIsDueAtLocalMidnight(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	cal := household.Member.ConnectCalendar()

	put(cal, "tasks", "phone-todo", ical.CompToDo,
		"UID:phone-todo",
		"DTSTAMP:20300115T120000Z",
		"SUMMARY:Book the vet",
		"DUE;VALUE=DATE:20300118",
	).Expect(http.StatusCreated)

	var task store.Task
	for _, candidate := range apitest.Decode[[]store.Task](t, household.Member.Get("/api/v1/tasks").Expect(http.StatusOK)) {
		if candidate.Title == "Book the vet" {
			task = candidate
		}
	}
	if want := time.Date(2030, time.January, 18, 6, 0, 0, 0, time.UTC); task.DueAt == nil || !task.DueAt.Equal(want) {
		t.Errorf("due %v, want Chicago midnight %s", task.DueAt, want)
	}
}

func TestAllDayAppointmentsGoBothWaysInLocalDays(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	cal := household.Member.ConnectCalendar()
	start := localMidnight(2030, time.January, 21)
	household.Owner.Post("/api/v1/events", map[string]any{
		"title":        "Holiday",
		"starts_at":    start.Format(time.RFC3339),
		"ends_at":      start.AddDate(0, 0, 2).Format(time.RFC3339),
		"attendee_ids": []int64{},
	}).Expect(http.StatusCreated)
	household.Owner.Post("/api/v1/events", map[string]any{
		"title":        "UTC midnight to midnight",
		"starts_at":    "2030-01-21T00:00:00Z",
		"ends_at":      "2030-01-22T00:00:00Z",
		"attendee_ids": []int64{},
	}).Expect(http.StatusCreated)

	events := objects(t, cal, "homebase", ical.CompEvent)
	expectDate(t, "Holiday start", events["Holiday"], ical.PropDateTimeStart, "20300121")
	expectDate(t, "Holiday end", events["Holiday"], ical.PropDateTimeEnd, "20300123")
	if p := events["UTC midnight to midnight"].Props.Get(ical.PropDateTimeStart); p == nil || p.ValueType() == ical.ValueDate {
		t.Errorf("UTC-midnight Appointment DTSTART = %+v, want a timed event", p)
	}

	put(cal, "homebase", "phone-all-day", ical.CompEvent,
		"UID:phone-all-day",
		"DTSTAMP:20300115T120000Z",
		"SUMMARY:Phone holiday",
		"DTSTART;VALUE=DATE:20300125",
		"DTEND;VALUE=DATE:20300126",
	).Expect(http.StatusCreated)
	var made store.Event
	for _, e := range apitest.Decode[[]store.Event](t, household.Owner.Get("/api/v1/events").Expect(http.StatusOK)) {
		if e.Title == "Phone holiday" {
			made = e
		}
	}
	if want := time.Date(2030, time.January, 25, 6, 0, 0, 0, time.UTC); !made.StartsAt.Equal(want) || !made.EndsAt.Equal(want.Add(24*time.Hour)) {
		t.Errorf("phone all-day event runs %s to %s, want Chicago midnight %s for a day", made.StartsAt.UTC(), made.EndsAt.UTC(), want)
	}
	expectDate(t, "phone all-day reads back", objects(t, cal, "homebase", ical.CompEvent)["Phone holiday"], ical.PropDateTimeStart, "20300125")
}

func TestAProjectDueDateIsItsOwnLocalDate(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	household.Owner.Post("/api/v1/projects", map[string]any{
		"title":    "Kitchen remodel",
		"status":   "active",
		"due_date": "2030-02-01T00:00:00Z",
	}).Expect(http.StatusCreated)
	cal := household.Member.ConnectCalendar()

	project := objects(t, cal, "deadlines", ical.CompEvent)["Project due: Kitchen remodel"]
	expectDate(t, "Project start", project, ical.PropDateTimeStart, "20300201")
	expectDate(t, "Project end", project, ical.PropDateTimeEnd, "20300202")
}

func TestARoutineDueOnADateIsAllDayOnThatLocalDate(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	household.Owner.Post("/api/v1/routines", map[string]any{
		"title":       "Test the alarms",
		"cadence":     "weekly",
		"next_due_at": localMidnight(2030, time.January, 21).Format(time.RFC3339),
	}).Expect(http.StatusCreated)
	household.Owner.Post("/api/v1/routines", map[string]any{
		"title":       "Due at UTC midnight",
		"cadence":     "weekly",
		"next_due_at": "2030-01-21T00:00:00Z",
	}).Expect(http.StatusCreated)
	cal := household.Member.ConnectCalendar()

	deadlines := objects(t, cal, "deadlines", ical.CompEvent)
	expectDate(t, "date-only Routine", deadlines["Routine: Test the alarms"], ical.PropDateTimeStart, "20300121")
	if p := deadlines["Routine: Due at UTC midnight"].Props.Get(ical.PropDateTimeStart); p == nil || p.Value != "20300121T000000Z" {
		t.Errorf("UTC-midnight Routine DTSTART = %+v, want the instant 20300121T000000Z", p)
	}
}

func TestTheWindowsEdgesAreLocalMidnights(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	// The fake Clock says 2030-01-15 12:00 UTC, 06:00 in Chicago: the
	// Window runs from 2029-10-17 07:00 Chicago (CDT) to 2031-01-15 06:00.
	for _, p := range []struct{ title, due string }{
		{"Before", "2029-10-17T00:00:00Z"},
		{"First", "2029-10-18T00:00:00Z"},
		{"Last", "2031-01-15T00:00:00Z"},
		{"After", "2031-01-16T00:00:00Z"},
	} {
		household.Owner.Post("/api/v1/projects", map[string]any{"title": p.title, "status": "active", "due_date": p.due}).Expect(http.StatusCreated)
	}
	for _, task := range []struct {
		title string
		due   time.Time
	}{
		{"Before", localMidnight(2029, time.October, 17)},
		{"First", localMidnight(2029, time.October, 18)},
		{"Last", localMidnight(2031, time.January, 15)},
		{"After", localMidnight(2031, time.January, 16)},
	} {
		household.Owner.Post("/api/v1/tasks", map[string]any{"title": task.title, "due_at": task.due.Format(time.RFC3339)}).Expect(http.StatusCreated)
	}
	cal := household.Member.ConnectCalendar()

	deadlines := objects(t, cal, "deadlines", ical.CompEvent)
	todos := objects(t, cal, "tasks", ical.CompToDo)
	for _, title := range []string{"Before", "First", "Last", "After"} {
		_, project := deadlines["Project due: "+title]
		_, task := todos[title]
		want := title == "First" || title == "Last"
		if project != want || task != want {
			t.Errorf("%s: Project shown %v, Task shown %v, want both %v", title, project, task, want)
		}
	}
}

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

// deadlines lists the deadlines collection by SUMMARY, the way a phone does.
func deadlines(t *testing.T, cal *apitest.CalDAV) map[string]caldav.CalendarObject {
	t.Helper()
	objects, err := cal.Client.QueryCalendar(context.Background(), cal.CollectionPath("deadlines"), &caldav.CalendarQuery{
		CompRequest: caldav.CalendarCompRequest{Name: "VCALENDAR", AllProps: true, AllComps: true},
		CompFilter:  caldav.CompFilter{Name: "VCALENDAR", Comps: []caldav.CompFilter{{Name: "VEVENT"}}},
	})
	if err != nil {
		t.Fatalf("calendar-query: %v", err)
	}
	byTitle := map[string]caldav.CalendarObject{}
	for _, o := range objects {
		byTitle[summaryOf(t, o)] = o
	}
	return byTitle
}

func expectDeadlines(t *testing.T, cal *apitest.CalDAV, want ...string) {
	t.Helper()
	got := titles(deadlines(t, cal))
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("deadlines holds %q, want %q", got, want)
	}
}

func createProject(t *testing.T, c *apitest.Client, title, status string, due time.Time) store.Project {
	t.Helper()
	return apitest.Decode[store.Project](t, c.Post("/api/v1/projects", map[string]any{
		"title":    title,
		"status":   status,
		"due_date": due.Format(time.RFC3339),
	}).Expect(http.StatusCreated))
}

func createRoutine(t *testing.T, c *apitest.Client, title string, due time.Time, assignee *int64) store.Routine {
	t.Helper()
	body := map[string]any{
		"title":       title,
		"cadence":     "weekly",
		"next_due_at": due.Format(time.RFC3339),
	}
	if assignee != nil {
		body["assigned_to"] = *assignee
	}
	return apitest.Decode[store.Routine](t, c.Post("/api/v1/routines", body).Expect(http.StatusCreated))
}

func TestTheDeadlinesCalendarIsDiscoveredAsReadOnly(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	cal := household.Member.ConnectCalendar()

	calendars, err := cal.Client.FindCalendars(context.Background(), cal.HomePath())
	if err != nil {
		t.Fatalf("find calendars: %v", err)
	}
	var found *caldav.Calendar
	for i := range calendars {
		if calendars[i].Path == cal.CollectionPath("deadlines") {
			found = &calendars[i]
		}
	}
	if found == nil {
		t.Fatalf("calendars = %+v, want a deadlines collection", calendars)
	}
	if found.Name != "Homebase Deadlines" {
		t.Errorf("name = %q, want Homebase Deadlines", found.Name)
	}
	if len(found.SupportedComponentSet) != 1 || found.SupportedComponentSet[0] != "VEVENT" {
		t.Errorf("components = %v, want only VEVENT", found.SupportedComponentSet)
	}

	ms := cal.Propfind(cal.CollectionPath("deadlines"), "0", apitest.CurrentUserPrivilegeSet, apitest.GetCTag)
	props := ms[cal.CollectionPath("deadlines")]
	if p := props[apitest.CurrentUserPrivilegeSet]; !p.Has("{DAV:}read") || p.Has("{DAV:}write") || p.Has("{DAV:}all") {
		t.Errorf("privileges = %+v, want read only", p)
	}
	if p := props[apitest.GetCTag]; p.Status != http.StatusOK || p.Text == "" {
		t.Errorf("getctag = %+v, want a change tag", p)
	}
}

func TestTheDeadlinesCalendarHoldsProjectsThatArentDoneWithinTheWindow(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	// The fake Clock says 2030-01-15 12:00 UTC: the window runs from
	// 2029-10-17 12:00 to 2031-01-15 12:00, half-open. A due date counts by
	// its local midnight, like a date-only Task or Routine.
	day := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

	createProject(t, household.Owner, "Active", "active", day(2030, time.February, 1))
	createProject(t, household.Owner, "Waiting", "waiting", day(2030, time.February, 1))
	createProject(t, household.Owner, "Done", "done", day(2030, time.February, 1))
	archived := createProject(t, household.Owner, "Archived", "active", day(2030, time.February, 1))
	household.Owner.Post(fmt.Sprintf("/api/v1/projects/%d/archive", archived.ID), nil).Expect(http.StatusOK)
	createProject(t, household.Owner, "Too long ago", "active", day(2029, time.October, 17))
	createProject(t, household.Owner, "First day", "active", day(2029, time.October, 18))
	createProject(t, household.Owner, "Last day", "active", day(2031, time.January, 15))
	createProject(t, household.Owner, "Too far ahead", "active", day(2031, time.January, 16))
	household.Owner.Post("/api/v1/projects", map[string]any{"title": "No due date"}).Expect(http.StatusCreated)
	elsewhere := h.SeedHousehold()
	createProject(t, elsewhere.Owner, "Another Household's", "active", day(2030, time.February, 1))

	cal := household.Member.ConnectCalendar()

	expectDeadlines(t, cal,
		"Project due: Active",
		"Project due: First day",
		"Project due: Last day",
		"Project due: Waiting",
	)
}

func TestAProjectDueDateIsAFreeAllDayEntryWithALinkBack(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	project := createProject(t, household.Owner, "Kitchen remodel", "active", time.Date(2030, time.February, 1, 0, 0, 0, 0, time.UTC))
	cal := household.Member.ConnectCalendar()

	object, ok := deadlines(t, cal)["Project due: Kitchen remodel"]
	if !ok {
		t.Fatalf("deadlines has no Project due: Kitchen remodel")
	}
	event := object.Data.Events()[0]
	if p := event.Props.Get(ical.PropDateTimeStart); p == nil || p.Value != "20300201" || p.ValueType() != ical.ValueDate {
		t.Errorf("DTSTART = %+v, want the date 20300201", p)
	}
	if p := event.Props.Get(ical.PropDateTimeEnd); p == nil || p.Value != "20300202" || p.ValueType() != ical.ValueDate {
		t.Errorf("DTEND = %+v, want the date 20300202", p)
	}
	if transp, _ := event.Props.Text(ical.PropTransparency); transp != "TRANSPARENT" {
		t.Errorf("TRANSP = %q, want TRANSPARENT (free)", transp)
	}
	if link := event.Props.Get(ical.PropURL); link == nil || link.Value != fmt.Sprintf("http://homebase.test/projects/%d", project.ID) {
		t.Errorf("URL = %+v, want a link to the Project's page", link)
	}
}

func TestTheDeadlinesCalendarHoldsTheMembersRoutinesNextDueDates(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	other := addMember(t, h, household, "third@example.test", "Third")
	soon := time.Date(2030, time.January, 20, 9, 0, 0, 0, time.UTC)
	member := household.Member.User.ID

	createRoutine(t, household.Owner, "Mine", soon, &member)
	createRoutine(t, household.Owner, "Unassigned", soon, nil)
	createRoutine(t, household.Owner, "Someone else's", soon, &other.User.ID)
	archived := createRoutine(t, household.Owner, "Archived", soon, &member)
	household.Owner.Post(fmt.Sprintf("/api/v1/routines/%d/archive", archived.ID), nil).Expect(http.StatusOK)
	createRoutine(t, household.Owner, "Too long ago", time.Date(2029, time.October, 17, 11, 0, 0, 0, time.UTC), nil)
	createRoutine(t, household.Owner, "Too far ahead", time.Date(2031, time.January, 15, 13, 0, 0, 0, time.UTC), nil)
	createRoutine(t, household.Owner, "Date-only before the window", time.Date(2029, time.October, 17, 0, 0, 0, 0, time.Local), nil)
	createRoutine(t, household.Owner, "Date-only on the last day", time.Date(2031, time.January, 15, 0, 0, 0, 0, time.Local), nil)
	household.Owner.Post("/api/v1/routines", map[string]any{"title": "Never due"}).Expect(http.StatusCreated)
	elsewhere := h.SeedHousehold()
	createRoutine(t, elsewhere.Owner, "Another Household's", soon, nil)

	cal := household.Member.ConnectCalendar()

	expectDeadlines(t, cal, "Routine: Date-only on the last day", "Routine: Mine", "Routine: Unassigned")
}

func TestARoutineDropsOutOfTheDeadlinesCalendarOnceItsGeneratedTaskExists(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	routine := createRoutine(t, household.Owner, "Clean the gutters", time.Date(2030, time.January, 20, 9, 0, 0, 0, time.UTC), nil)
	cal := household.Member.ConnectCalendar()
	expectDeadlines(t, cal, "Routine: Clean the gutters")

	household.Owner.Post(fmt.Sprintf("/api/v1/routines/%d/generate-task", routine.ID), nil).Expect(http.StatusCreated)

	expectDeadlines(t, cal)
}

func TestARoutineIsAFreeInstantOrAllDayWhenDueOnADate(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	timed := createRoutine(t, household.Owner, "Feed the starter", time.Date(2030, time.January, 20, 9, 0, 0, 0, time.UTC), nil)
	createRoutine(t, household.Owner, "Test the alarms", time.Date(2030, time.January, 21, 0, 0, 0, 0, time.Local), nil)
	cal := household.Member.ConnectCalendar()

	objects := deadlines(t, cal)
	starter := objects["Routine: Feed the starter"].Data.Events()[0]
	if p := starter.Props.Get(ical.PropDateTimeStart); p == nil || p.Value != "20300120T090000Z" {
		t.Errorf("timed DTSTART = %+v, want 20300120T090000Z", p)
	}
	if p := starter.Props.Get(ical.PropDateTimeEnd); p != nil {
		t.Errorf("timed DTEND = %+v, want none (an instant)", p)
	}
	if link := starter.Props.Get(ical.PropURL); link == nil || link.Value != fmt.Sprintf("http://homebase.test/routines/%d", timed.ID) {
		t.Errorf("URL = %+v, want a link to the Routine's page", link)
	}
	alarms := objects["Routine: Test the alarms"].Data.Events()[0]
	if p := alarms.Props.Get(ical.PropDateTimeStart); p == nil || p.Value != "20300121" || p.ValueType() != ical.ValueDate {
		t.Errorf("date-only DTSTART = %+v, want the date 20300121", p)
	}
	for title, o := range objects {
		if transp, _ := o.Data.Events()[0].Props.Text(ical.PropTransparency); transp != "TRANSPARENT" {
			t.Errorf("%s TRANSP = %q, want TRANSPARENT (free)", title, transp)
		}
	}
}

func TestWritesToTheDeadlinesCalendarAreRefused(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	createProject(t, household.Owner, "Kitchen remodel", "active", time.Date(2030, time.February, 1, 0, 0, 0, 0, time.UTC))
	cal := household.Member.ConnectCalendar()
	existing := deadlines(t, cal)["Project due: Kitchen remodel"].Path
	ics := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Test//EN\r\nBEGIN:VEVENT\r\nUID:new-one\r\nDTSTAMP:20300115T120000Z\r\nDTSTART:20300120T090000Z\r\nSUMMARY:Sneaky\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	header := http.Header{"Content-Type": {"text/calendar"}}

	cal.Do(http.MethodPut, cal.CollectionPath("deadlines")+"new-one.ics", header, ics).Expect(http.StatusForbidden)
	cal.Do(http.MethodPut, existing, header, ics).Expect(http.StatusForbidden)
	cal.Do(http.MethodDelete, existing, nil, "").Expect(http.StatusForbidden)

	expectDeadlines(t, cal, "Project due: Kitchen remodel")
}

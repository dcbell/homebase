package api_test

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav/caldav"

	"homebase/internal/apitest"
	"homebase/internal/store"
)

// createAppointment creates an Appointment through the JSON API.
func createAppointment(t *testing.T, c *apitest.Client, title string, starts, ends time.Time, attendees ...int64) store.Event {
	t.Helper()
	body := map[string]any{
		"title":        title,
		"starts_at":    starts.Format(time.RFC3339),
		"ends_at":      ends.Format(time.RFC3339),
		"attendee_ids": attendees,
	}
	if attendees == nil {
		body["attendee_ids"] = []int64{}
	}
	return apitest.Decode[store.Event](t, c.Post("/api/v1/events", body).Expect(http.StatusCreated))
}

// allEvents asks the homebase collection for every VEVENT, the way a phone
// lists a calendar, and returns them by SUMMARY.
func allEvents(t *testing.T, cal *apitest.CalDAV) map[string]caldav.CalendarObject {
	t.Helper()
	return queryEvents(t, cal, time.Time{}, time.Time{})
}

func queryEvents(t *testing.T, cal *apitest.CalDAV, start, end time.Time) map[string]caldav.CalendarObject {
	t.Helper()
	objects, err := cal.Client.QueryCalendar(context.Background(), cal.CollectionPath("homebase"), &caldav.CalendarQuery{
		CompRequest: caldav.CalendarCompRequest{Name: "VCALENDAR", AllProps: true, AllComps: true},
		CompFilter: caldav.CompFilter{Name: "VCALENDAR", Comps: []caldav.CompFilter{
			{Name: "VEVENT", Start: start, End: end},
		}},
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

func summaryOf(t *testing.T, o caldav.CalendarObject) string {
	t.Helper()
	events := o.Data.Events()
	if len(events) != 1 {
		t.Fatalf("%s holds %d events, want 1", o.Path, len(events))
	}
	summary, _ := events[0].Props.Text(ical.PropSummary)
	return summary
}

func titles(objects map[string]caldav.CalendarObject) []string {
	list := make([]string, 0, len(objects))
	for title := range objects {
		list = append(list, title)
	}
	sort.Strings(list)
	return list
}

func TestTheHomebaseCalendarHoldsTheAppointmentsThatConcernTheMember(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	other := addMember(t, h, household, "third@example.test", "Third")
	soon := apitest.DefaultNow.Add(48 * time.Hour)

	createAppointment(t, household.Owner, "Mine", soon, soon.Add(time.Hour), household.Member.User.ID)
	createAppointment(t, household.Owner, "Shared with me", soon, soon.Add(time.Hour), household.Member.User.ID, household.Owner.User.ID)
	createAppointment(t, household.Owner, "Whole Household", soon, soon.Add(time.Hour))
	createAppointment(t, household.Owner, "Someone else's", soon, soon.Add(time.Hour), other.User.ID)
	archived := createAppointment(t, household.Owner, "Archived", soon, soon.Add(time.Hour), household.Member.User.ID)
	household.Owner.Delete(fmt.Sprintf("/api/v1/events/%d", archived.ID)).Expect(http.StatusOK)
	elsewhere := h.SeedHousehold()
	createAppointment(t, elsewhere.Owner, "Another Household's", soon, soon.Add(time.Hour))

	cal := household.Member.ConnectCalendar()

	got := titles(allEvents(t, cal))
	want := []string{"Mine", "Shared with me", "Whole Household"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("homebase holds %q, want %q", got, want)
	}
}

func TestTheHomebaseCalendarCovers90DaysBackTo12MonthsAhead(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	// The fake Clock says 2030-01-15 12:00 UTC.
	windowStart := time.Date(2029, time.October, 17, 12, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2031, time.January, 15, 12, 0, 0, 0, time.UTC)

	createAppointment(t, household.Owner, "Ended before the window", windowStart.Add(-2*time.Hour), windowStart.Add(-time.Hour))
	createAppointment(t, household.Owner, "Ends inside the window", windowStart.Add(-time.Hour), windowStart.Add(time.Hour))
	createAppointment(t, household.Owner, "Starts inside the window", windowEnd.Add(-time.Hour), windowEnd.Add(time.Hour))
	createAppointment(t, household.Owner, "Starts after the window", windowEnd.Add(time.Hour), windowEnd.Add(2*time.Hour))

	cal := household.Member.ConnectCalendar()

	got := titles(allEvents(t, cal))
	want := []string{"Ends inside the window", "Starts inside the window"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("homebase holds %q, want %q", got, want)
	}

	h.Clock.Advance(3 * time.Hour)
	got = titles(allEvents(t, cal))
	want = []string{"Starts after the window", "Starts inside the window"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("3 hours later homebase holds %q, want %q", got, want)
	}
}

func TestAnAppointmentOnThePhoneIsBusyWithItsDetailsAndALinkBack(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	created := apitest.Decode[store.Event](t, household.Owner.Post("/api/v1/events", map[string]any{
		"title":        "Dentist",
		"description":  "Bring the insurance card.",
		"location":     "12 Main St",
		"starts_at":    "2030-01-20T09:00:00Z",
		"ends_at":      "2030-01-20T10:30:00Z",
		"attendee_ids": []int64{household.Member.User.ID, household.Owner.User.ID},
	}).Expect(http.StatusCreated))
	cal := household.Member.ConnectCalendar()

	object := allEvents(t, cal)["Dentist"]
	raw := string(cal.Do(http.MethodGet, object.Path, nil, "").Expect(http.StatusOK).Body)

	for _, line := range []string{
		"SUMMARY:Dentist",
		"DTSTART:20300120T090000Z",
		"DTEND:20300120T103000Z",
		"LOCATION:12 Main St",
		"TRANSP:OPAQUE",
	} {
		if !strings.Contains(raw, line+"\r\n") {
			t.Errorf("calendar data has no %q line:\n%s", line, raw)
		}
	}
	event := object.Data.Events()[0]
	description, _ := event.Props.Text(ical.PropDescription)
	if want := "Bring the insurance card.\n\nAttendees: Member 1, Owner 1"; description != want {
		t.Errorf("DESCRIPTION = %q, want %q", description, want)
	}
	if link := event.Props.Get(ical.PropURL); link == nil || link.Value != fmt.Sprintf("http://homebase.test/events/%d", created.ID) {
		t.Errorf("URL = %+v, want a link to the Appointment's page", link)
	}
	for _, absent := range []string{"BEGIN:VALARM", "ATTENDEE", "ORGANIZER"} {
		if strings.Contains(raw, absent) {
			t.Errorf("calendar data has %s:\n%s", absent, raw)
		}
	}
}

func TestAnAppointmentFromMidnightToMidnightIsAllDay(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	start := time.Date(2030, time.January, 21, 0, 0, 0, 0, time.Local)
	createAppointment(t, household.Owner, "Holiday", start, start.AddDate(0, 0, 2))
	createAppointment(t, household.Owner, "Whole Household meeting", start.Add(9*time.Hour), start.Add(10*time.Hour))
	cal := household.Member.ConnectCalendar()

	events := allEvents(t, cal)
	holiday := events["Holiday"].Data.Events()[0]
	if p := holiday.Props.Get(ical.PropDateTimeStart); p == nil || p.Value != "20300121" || p.ValueType() != ical.ValueDate {
		t.Errorf("all-day DTSTART = %+v, want the date 20300121", p)
	}
	if p := holiday.Props.Get(ical.PropDateTimeEnd); p == nil || p.Value != "20300123" || p.ValueType() != ical.ValueDate {
		t.Errorf("all-day DTEND = %+v, want the date 20300123", p)
	}
	meeting := events["Whole Household meeting"].Data.Events()[0]
	if p := meeting.Props.Get(ical.PropDateTimeStart); p == nil || p.ValueType() == ical.ValueDate || !strings.HasSuffix(p.Value, "Z") {
		t.Errorf("timed DTSTART = %+v, want a UTC time", p)
	}
	description, _ := meeting.Props.Text(ical.PropDescription)
	if description != "Attendees: everyone in the Household" {
		t.Errorf("DESCRIPTION = %q, want only the Attendee trailer", description)
	}
}

func collectionCTag(t *testing.T, cal *apitest.CalDAV) string {
	t.Helper()
	ms := cal.Propfind(cal.CollectionPath("homebase"), "0", apitest.GetCTag)
	return ms[cal.CollectionPath("homebase")][apitest.GetCTag].Text
}

func TestEditingAnAppointmentInHomebaseChangesItsETagAndTheChangeTag(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	soon := apitest.DefaultNow.Add(48 * time.Hour)
	event := createAppointment(t, household.Owner, "Dentist", soon, soon.Add(time.Hour))
	createAppointment(t, household.Owner, "Haircut", soon, soon.Add(time.Hour))
	cal := household.Member.ConnectCalendar()
	ctx := context.Background()

	path := allEvents(t, cal)["Dentist"].Path
	before, err := cal.Client.GetCalendarObject(ctx, path)
	if err != nil {
		t.Fatalf("get %s: %v", path, err)
	}
	ctagBefore := collectionCTag(t, cal)
	if again := collectionCTag(t, cal); again != ctagBefore {
		t.Fatalf("change tag moved from %q to %q with no change", ctagBefore, again)
	}
	h.Clock.Advance(time.Minute)
	if again, _ := cal.Client.GetCalendarObject(ctx, path); again.ETag != before.ETag || collectionCTag(t, cal) != ctagBefore {
		t.Fatalf("ETag or change tag moved with the Clock alone")
	}

	household.Owner.Patch(fmt.Sprintf("/api/v1/events/%d", event.ID), map[string]any{
		"title":     "Dentist (moved)",
		"starts_at": soon.Add(time.Hour).Format(time.RFC3339),
		"ends_at":   soon.Add(2 * time.Hour).Format(time.RFC3339),
	}).Expect(http.StatusOK)

	after, err := cal.Client.GetCalendarObject(ctx, path)
	if err != nil {
		t.Fatalf("get %s after edit: %v", path, err)
	}
	if after.ETag == "" || after.ETag == before.ETag {
		t.Errorf("ETag = %q after edit, was %q; want a new one", after.ETag, before.ETag)
	}
	if summaryOf(t, *after) != "Dentist (moved)" {
		t.Errorf("phone sees %q, want the edited title", summaryOf(t, *after))
	}
	if ctag := collectionCTag(t, cal); ctag == ctagBefore {
		t.Errorf("change tag stayed %q after an edit", ctag)
	}
}

func TestPhonesCanFetchSeveralAppointmentsAndFilterByDate(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	day := time.Date(2030, time.February, 10, 9, 0, 0, 0, time.UTC)
	createAppointment(t, household.Owner, "Monday", day, day.Add(time.Hour))
	createAppointment(t, household.Owner, "Tuesday", day.AddDate(0, 0, 1), day.AddDate(0, 0, 1).Add(time.Hour))
	createAppointment(t, household.Owner, "Wednesday", day.AddDate(0, 0, 2), day.AddDate(0, 0, 2).Add(time.Hour))
	cal := household.Member.ConnectCalendar()

	inRange := titles(queryEvents(t, cal, day.AddDate(0, 0, 1), day.AddDate(0, 0, 2)))
	if strings.Join(inRange, "|") != "Tuesday" {
		t.Errorf("time-range query = %q, want only Tuesday", inRange)
	}

	all := allEvents(t, cal)
	missing := cal.CollectionPath("homebase") + "no-such-item.ics"
	objects, err := cal.Client.MultiGetCalendar(context.Background(), cal.CollectionPath("homebase"), &caldav.CalendarMultiGet{
		Paths:       []string{all["Monday"].Path, all["Wednesday"].Path},
		CompRequest: caldav.CalendarCompRequest{Name: "VCALENDAR", AllProps: true, AllComps: true},
	})
	if err != nil {
		t.Fatalf("calendar-multiget: %v", err)
	}
	got := map[string]caldav.CalendarObject{}
	for _, o := range objects {
		got[summaryOf(t, o)] = o
	}
	if strings.Join(titles(got), "|") != "Monday|Wednesday" {
		t.Errorf("multiget = %q, want Monday and Wednesday", titles(got))
	}
	if got["Monday"].ETag == "" {
		t.Errorf("multiget gave Monday no ETag")
	}

	cal.Do(http.MethodGet, missing, nil, "").Expect(http.StatusNotFound)
}

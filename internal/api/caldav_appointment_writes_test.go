package api_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-ical"

	"homebase/internal/apitest"
	caldavpkg "homebase/internal/caldav"
	"homebase/internal/store"
)

// vevent wraps VEVENT lines in a calendar object, the way a phone PUTs one.
func vevent(lines ...string) string {
	return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Test phone//EN\r\nBEGIN:VEVENT\r\n" +
		strings.Join(lines, "\r\n") + "\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
}

// putObject PUTs calendar data at uid.ics in a collection. header may be nil.
func putObject(cal *apitest.CalDAV, collection, uid string, header http.Header, ics string) *apitest.Response {
	all := http.Header{"Content-Type": {"text/calendar; charset=utf-8"}}
	for key, values := range header {
		all[key] = values
	}
	return cal.Do(http.MethodPut, cal.CollectionPath(collection)+uid+".ics", all, ics)
}

// appointmentTitled finds the Household's active Appointment with a title
// through the JSON API.
func appointmentTitled(t *testing.T, c *apitest.Client, title string) (store.Event, bool) {
	t.Helper()
	for _, e := range apitest.Decode[[]store.Event](t, c.Get("/api/v1/events").Expect(http.StatusOK)) {
		if e.Title == title {
			return e, true
		}
	}
	return store.Event{}, false
}

func getAppointment(t *testing.T, c *apitest.Client, id int64) store.Event {
	t.Helper()
	return apitest.Decode[store.Event](t, c.Get(fmt.Sprintf("/api/v1/events/%d", id)).Expect(http.StatusOK))
}

func TestCreatingAnEventOnThePhoneMakesAnAppointmentForTheMember(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	cal := household.Member.ConnectCalendar()

	putObject(cal, "homebase", "phone-made-1", http.Header{"If-None-Match": {"*"}}, vevent(
		"UID:phone-made-1",
		"DTSTAMP:20300115T120000Z",
		"DTSTART:20300120T090000Z",
		"DTEND:20300120T100000Z",
		"SUMMARY:Vet",
		"LOCATION:Animal clinic",
		"DESCRIPTION:Bring the cat.",
	)).Expect(http.StatusCreated)

	appt, ok := appointmentTitled(t, household.Owner, "Vet")
	if !ok {
		t.Fatalf("no Appointment titled Vet after the phone created one")
	}
	if got := fmt.Sprint(appt.AttendeeIDs); got != fmt.Sprint([]int64{household.Member.User.ID}) {
		t.Errorf("Attendees = %s, want only the connecting Member %d", got, household.Member.User.ID)
	}
	if appt.Location != "Animal clinic" || appt.Description != "Bring the cat." {
		t.Errorf("location, notes = %q, %q", appt.Location, appt.Description)
	}
	if want := "2030-01-20T09:00:00Z 2030-01-20T10:00:00Z"; appt.StartsAt.UTC().Format("2006-01-02T15:04:05Z")+" "+appt.EndsAt.UTC().Format("2006-01-02T15:04:05Z") != want {
		t.Errorf("runs %s to %s, want %s", appt.StartsAt, appt.EndsAt, want)
	}

	// The phone's own UID and address survive a later read.
	raw := string(cal.Do(http.MethodGet, cal.CollectionPath("homebase")+"phone-made-1.ics", nil, "").Expect(http.StatusOK).Body)
	if !strings.Contains(raw, "UID:phone-made-1\r\n") {
		t.Errorf("phone-made event lost its UID:\n%s", raw)
	}
	if _, ok := allEvents(t, cal)["Vet"]; !ok {
		t.Errorf("phone-made event is missing from the calendar")
	}
}

// currentETag is the If-Match value for what the phone last fetched at path.
func currentETag(t *testing.T, cal *apitest.CalDAV, path string) string {
	t.Helper()
	o, err := cal.Client.GetCalendarObject(context.Background(), path)
	if err != nil {
		t.Fatalf("get %s: %v", path, err)
	}
	return `"` + strings.Trim(o.ETag, `"`) + `"`
}

func TestEditingAnEventOnThePhoneChangesTheAppointmentButNotItsAttendees(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	created := apitest.Decode[store.Event](t, household.Owner.Post("/api/v1/events", map[string]any{
		"title":        "Dentist",
		"description":  "Bring the insurance card.",
		"starts_at":    "2030-01-20T09:00:00Z",
		"ends_at":      "2030-01-20T10:00:00Z",
		"attendee_ids": []int64{household.Member.User.ID, household.Owner.User.ID},
	}).Expect(http.StatusCreated))
	cal := household.Member.ConnectCalendar()
	uid := caldavpkg.AppointmentUID(created.ID)
	path := cal.CollectionPath("homebase") + uid + ".ics"
	etag := currentETag(t, cal, path)

	// The phone sends back the notes it was given, Attendee trailer and all.
	resp := putObject(cal, "homebase", uid, http.Header{"If-Match": {etag}}, vevent(
		"UID:"+uid,
		"DTSTAMP:20300115T120000Z",
		"DTSTART:20300121T140000Z",
		"DTEND:20300121T153000Z",
		"SUMMARY:Dentist (moved)",
		"LOCATION:New clinic",
		`DESCRIPTION:Bring the insurance card\, and the forms.\n\nAttendees: Member 1\, Owner 1`,
	)).Expect(http.StatusCreated)
	if got := resp.Header.Get("ETag"); got != "" {
		t.Errorf("PUT answered ETag %q; want none so the phone refetches Homebase's version", got)
	}

	appt := getAppointment(t, household.Owner, created.ID)
	if appt.Title != "Dentist (moved)" || appt.Location != "New clinic" || appt.Description != "Bring the insurance card, and the forms." {
		t.Errorf("title, location, notes = %q, %q, %q", appt.Title, appt.Location, appt.Description)
	}
	if !appt.StartsAt.Equal(time.Date(2030, 1, 21, 14, 0, 0, 0, time.UTC)) || !appt.EndsAt.Equal(time.Date(2030, 1, 21, 15, 30, 0, 0, time.UTC)) {
		t.Errorf("runs %s to %s, want 14:00 to 15:30 UTC on the 21st", appt.StartsAt, appt.EndsAt)
	}
	if got, want := fmt.Sprint(appt.AttendeeIDs), fmt.Sprint(created.AttendeeIDs); got != want {
		t.Errorf("Attendees = %s, want them unchanged at %s", got, want)
	}
	if again := currentETag(t, cal, path); again == etag {
		t.Errorf("ETag stayed %s after the edit", etag)
	}
}

func TestDeletingAnEventOnThePhoneArchivesTheAppointment(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	soon := apitest.DefaultNow.Add(48 * time.Hour)
	created := createAppointment(t, household.Owner, "Dentist", soon, soon.Add(time.Hour))
	cal := household.Member.ConnectCalendar()
	path := allEvents(t, cal)["Dentist"].Path

	cal.Do(http.MethodDelete, path, http.Header{"If-Match": {currentETag(t, cal, path)}}, "").Expect(http.StatusNoContent)

	if appt := getAppointment(t, household.Owner, created.ID); appt.Status != "archived" {
		t.Errorf("status = %q after the phone deleted it, want archived", appt.Status)
	}
	if _, ok := allEvents(t, cal)["Dentist"]; ok {
		t.Errorf("the deleted event is still on the phone's calendar")
	}
	cal.Do(http.MethodDelete, path, nil, "").Expect(http.StatusNotFound)
}

func TestAllDayAndNamedTimezoneEventsFromThePhoneAreStoredCorrectly(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	cal := household.Member.ConnectCalendar()

	putObject(cal, "homebase", "all-day", nil, vevent(
		"UID:all-day",
		"DTSTAMP:20300115T120000Z",
		"DTSTART;VALUE=DATE:20300121",
		"DTEND;VALUE=DATE:20300123",
		"SUMMARY:Holiday",
	)).Expect(http.StatusCreated)
	holiday, _ := appointmentTitled(t, household.Owner, "Holiday")
	if want := time.Date(2030, 1, 21, 0, 0, 0, 0, time.Local); !holiday.StartsAt.Equal(want) || !holiday.EndsAt.Equal(want.AddDate(0, 0, 2)) {
		t.Errorf("all-day event runs %s to %s, want local midnight %s for two days", holiday.StartsAt, holiday.EndsAt, want)
	}
	if p := allEvents(t, cal)["Holiday"].Data.Events()[0].Props.Get(ical.PropDateTimeStart); p == nil || p.Value != "20300121" {
		t.Errorf("all-day event reads back with DTSTART %+v, want the date 20300121", p)
	}

	ics := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Test phone//EN\r\n" +
		"BEGIN:VTIMEZONE\r\nTZID:America/Chicago\r\nBEGIN:STANDARD\r\nDTSTART:19701101T020000\r\nTZOFFSETFROM:-0500\r\nTZOFFSETTO:-0600\r\nEND:STANDARD\r\nEND:VTIMEZONE\r\n" +
		"BEGIN:VEVENT\r\nUID:chicago\r\nDTSTAMP:20300115T120000Z\r\n" +
		"DTSTART;TZID=America/Chicago:20300120T090000\r\nDTEND;TZID=America/Chicago:20300120T100000\r\n" +
		"SUMMARY:Call Chicago\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	putObject(cal, "homebase", "chicago", nil, ics).Expect(http.StatusCreated)
	call, _ := appointmentTitled(t, household.Owner, "Call Chicago")
	if want := time.Date(2030, 1, 20, 15, 0, 0, 0, time.UTC); !call.StartsAt.Equal(want) || !call.EndsAt.Equal(want.Add(time.Hour)) {
		t.Errorf("Chicago event runs %s to %s, want 15:00 UTC for an hour", call.StartsAt, call.EndsAt)
	}

}

// withTimezone is a calendar object holding a VTIMEZONE and one VEVENT.
func withTimezone(vtimezone string, lines ...string) string {
	return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Test phone//EN\r\n" + vtimezone +
		"BEGIN:VEVENT\r\n" + strings.Join(lines, "\r\n") + "\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
}

func TestTimesInAZoneGoDoesNotKnowAreReadWithTheObjectsVTIMEZONE(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	cal := household.Member.ConnectCalendar()

	// The way Outlook names US Central time, with yearly rules.
	central := "BEGIN:VTIMEZONE\r\nTZID:Central Standard Time\r\n" +
		"BEGIN:STANDARD\r\nDTSTART:16010101T020000\r\nTZOFFSETFROM:-0500\r\nTZOFFSETTO:-0600\r\nRRULE:FREQ=YEARLY;BYDAY=1SU;BYMONTH=11\r\nEND:STANDARD\r\n" +
		"BEGIN:DAYLIGHT\r\nDTSTART:16010101T020000\r\nTZOFFSETFROM:-0600\r\nTZOFFSETTO:-0500\r\nRRULE:FREQ=YEARLY;BYDAY=2SU;BYMONTH=3\r\nEND:DAYLIGHT\r\n" +
		"END:VTIMEZONE\r\n"
	fixed := "BEGIN:VTIMEZONE\r\nTZID:Custom Zone\r\n" +
		"BEGIN:STANDARD\r\nDTSTART:19700101T000000\r\nTZOFFSETFROM:+0530\r\nTZOFFSETTO:+0530\r\nEND:STANDARD\r\n" +
		"END:VTIMEZONE\r\n"
	cases := []struct {
		uid, vtimezone, tzid, start string
		want                        time.Time
	}{
		{"central-winter", central, "Central Standard Time", "20300120T090000", time.Date(2030, 1, 20, 15, 0, 0, 0, time.UTC)},
		{"central-summer", central, "Central Standard Time", "20300720T090000", time.Date(2030, 7, 20, 14, 0, 0, 0, time.UTC)},
		{"single-offset", fixed, "Custom Zone", "20300120T090000", time.Date(2030, 1, 20, 3, 30, 0, 0, time.UTC)},
	}
	for _, c := range cases {
		putObject(cal, "homebase", c.uid, nil, withTimezone(c.vtimezone,
			"UID:"+c.uid,
			"DTSTAMP:20300115T120000Z",
			"DTSTART;TZID="+c.tzid+":"+c.start,
			"DURATION:PT1H",
			"SUMMARY:"+c.uid,
		)).Expect(http.StatusCreated)
		got, _ := appointmentTitled(t, household.Owner, c.uid)
		if !got.StartsAt.Equal(c.want) {
			t.Errorf("%s: starts %s, want %s", c.uid, got.StartsAt.UTC(), c.want)
		}
	}
}

func TestTimesInAnUnknownZoneWithoutAVTIMEZONEAreRefused(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	cal := household.Member.ConnectCalendar()

	resp := putObject(cal, "homebase", "custom-zone", nil, vevent(
		"UID:custom-zone",
		"DTSTAMP:20300115T120000Z",
		"DTSTART;TZID=Custom Zone:20300120T090000",
		"DTEND;TZID=Custom Zone:20300120T100000",
		"SUMMARY:Custom zone",
	))
	if resp.Status != http.StatusForbidden || !strings.Contains(string(resp.Body), "valid-calendar-object-resource") {
		t.Errorf("status %d, body %s; want 403 valid-calendar-object-resource", resp.Status, resp.Body)
	}
	if _, ok := appointmentTitled(t, household.Owner, "Custom zone"); ok {
		t.Errorf("an Appointment was made from an event in an unknown timezone")
	}
}

func TestRepeatingEventsAndWrongItemTypesAreRefused(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	soon := apitest.DefaultNow.Add(48 * time.Hour)
	existing := createAppointment(t, household.Owner, "Dentist", soon, soon.Add(time.Hour))
	cal := household.Member.ConnectCalendar()

	resp := putObject(cal, "homebase", "weekly", nil, vevent(
		"UID:weekly",
		"DTSTAMP:20300115T120000Z",
		"DTSTART:20300120T090000Z",
		"DTEND:20300120T100000Z",
		"RRULE:FREQ=WEEKLY",
		"SUMMARY:Weekly",
	)).Expect(http.StatusForbidden)
	if !strings.Contains(string(resp.Body), "valid-calendar-object-resource") {
		t.Errorf("repeating event refusal has no CalDAV error: %s", resp.Body)
	}

	// Changing one occurrence of a series, sent against an existing item.
	uid := caldavpkg.AppointmentUID(existing.ID)
	putObject(cal, "homebase", uid, nil, vevent(
		"UID:"+uid,
		"DTSTAMP:20300115T120000Z",
		"RECURRENCE-ID:20300117T120000Z",
		"DTSTART:20300117T130000Z",
		"DTEND:20300117T140000Z",
		"SUMMARY:Dentist (one occurrence)",
	)).Expect(http.StatusForbidden)

	resp = putObject(cal, "homebase", "a-todo", nil,
		"BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Test phone//EN\r\nBEGIN:VTODO\r\nUID:a-todo\r\nDTSTAMP:20300115T120000Z\r\nSUMMARY:Buy milk\r\nEND:VTODO\r\nEND:VCALENDAR\r\n",
	).Expect(http.StatusForbidden)
	if !strings.Contains(string(resp.Body), "supported-calendar-component") {
		t.Errorf("wrong item type refusal has no CalDAV error: %s", resp.Body)
	}

	if got := titles(allEvents(t, cal)); strings.Join(got, "|") != "Dentist" {
		t.Errorf("homebase holds %q after refused writes, want only Dentist", got)
	}
}

func TestAlarmsFromThePhoneAreDiscarded(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	cal := household.Member.ConnectCalendar()

	putObject(cal, "homebase", "with-alarm", nil, vevent(
		"UID:with-alarm",
		"DTSTAMP:20300115T120000Z",
		"DTSTART:20300120T090000Z",
		"DTEND:20300120T100000Z",
		"SUMMARY:Vet",
		"BEGIN:VALARM",
		"ACTION:DISPLAY",
		"DESCRIPTION:Reminder",
		"TRIGGER:-PT15M",
		"END:VALARM",
	)).Expect(http.StatusCreated)

	raw := string(cal.Do(http.MethodGet, cal.CollectionPath("homebase")+"with-alarm.ics", nil, "").Expect(http.StatusOK).Body)
	if strings.Contains(raw, "VALARM") {
		t.Errorf("the phone's alarm came back:\n%s", raw)
	}
}

func TestWritesBasedOnAnOlderVersionAreRefused(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	soon := apitest.DefaultNow.Add(48 * time.Hour)
	created := createAppointment(t, household.Owner, "Dentist", soon, soon.Add(time.Hour))
	cal := household.Member.ConnectCalendar()
	uid := caldavpkg.AppointmentUID(created.ID)
	path := cal.CollectionPath("homebase") + uid + ".ics"
	stale := currentETag(t, cal, path)
	household.Owner.Patch(fmt.Sprintf("/api/v1/events/%d", created.ID), map[string]any{
		"title":     "Dentist (Homebase's version)",
		"starts_at": soon.Format(time.RFC3339),
		"ends_at":   soon.Add(time.Hour).Format(time.RFC3339),
	}).Expect(http.StatusOK)
	edit := func(title string) string {
		return vevent("UID:"+uid, "DTSTAMP:20300115T120000Z", "DTSTART:20300120T090000Z", "DTEND:20300120T100000Z", "SUMMARY:"+title)
	}

	putObject(cal, "homebase", uid, http.Header{"If-Match": {stale}}, edit("Phone's version")).Expect(http.StatusPreconditionFailed)
	cal.Do(http.MethodDelete, path, http.Header{"If-Match": {stale}}, "").Expect(http.StatusPreconditionFailed)
	putObject(cal, "homebase", uid, http.Header{"If-None-Match": {"*"}}, edit("Created over it")).Expect(http.StatusPreconditionFailed)
	if appt := getAppointment(t, household.Owner, created.ID); appt.Title != "Dentist (Homebase's version)" || appt.Status != "active" {
		t.Errorf("after refused writes the Appointment is %q (%s), want Homebase's version, active", appt.Title, appt.Status)
	}

	// A save that names no version overwrites.
	putObject(cal, "homebase", uid, nil, edit("Phone's version")).Expect(http.StatusCreated)
	if appt := getAppointment(t, household.Owner, created.ID); appt.Title != "Phone's version" {
		t.Errorf("unconditional save left the title %q", appt.Title)
	}
}

func TestMovingAnAppointmentFromThePhoneMovesItsReminders(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	appt := createAppointmentAt(t, household.Owner, at(17, 9, 0), household.Member.User.ID)
	cal := household.Member.ConnectCalendar()
	uid := caldavpkg.AppointmentUID(appt.ID)

	putObject(cal, "homebase", uid, nil, vevent(
		"UID:"+uid,
		"DTSTAMP:20300115T120000Z",
		"DTSTART:20300118T090000Z",
		"DTEND:20300118T100000Z",
		"SUMMARY:Dentist",
	)).Expect(http.StatusCreated)

	sendRemindersAt(t, h, at(16, 9, 0))
	expectAppointmentReminders(t, household.Member)
	sendRemindersAt(t, h, at(17, 9, 0))
	expectAppointmentReminders(t, household.Member, stages(appt.ID, "due_soon")...)
	sendRemindersAt(t, h, at(18, 8, 0))
	expectAppointmentReminders(t, household.Member, stages(appt.ID, "due_soon", "due_soon")...)
	sendRemindersAt(t, h, at(18, 9, 0))
	expectAppointmentReminders(t, household.Member, stages(appt.ID, "due_soon", "due_soon", "due_now")...)
}

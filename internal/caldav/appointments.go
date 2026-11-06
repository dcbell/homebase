package caldav

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/emersion/go-ical"

	"homebase/internal/store"
)

// Appointments is the "homebase" collection: the Appointments that concern
// the Member (they are an Attendee, or there are no Attendees) within the
// Window, archived ones excluded.
type Appointments struct {
	Store *store.Store
	// WebBaseURL is the public address each item links back to.
	WebBaseURL string
}

func (Appointments) Name() string        { return "homebase" }
func (Appointments) DisplayName() string { return "Homebase" }
func (Appointments) Component() string   { return ical.CompEvent }
func (Appointments) Color() string       { return "#2563EBFF" }

func (a Appointments) Objects(ctx context.Context, m Member, now time.Time) ([]Object, error) {
	from, to := Window(now)
	events, err := a.Store.ListAppointmentsConcerning(ctx, m.HouseholdID, m.UserID, from, to)
	if err != nil {
		return nil, err
	}
	objects := make([]Object, len(events))
	for i, e := range events {
		objects[i] = Object{UID: AppointmentUID(e.ID), Calendar: a.calendar(e), ModTime: e.UpdatedAt}
	}
	return objects, nil
}

// AppointmentUID is the stable UID of a Homebase-originated Appointment.
func AppointmentUID(id int64) string {
	return fmt.Sprintf("homebase-appointment-%d", id)
}

func (a Appointments) calendar(e store.Event) *ical.Calendar {
	event := ical.NewEvent()
	event.Props.SetText(ical.PropUID, AppointmentUID(e.ID))
	// DTSTAMP follows the Appointment, never the request time, so the ETag
	// only changes when the Appointment does.
	event.Props.SetDateTime(ical.PropDateTimeStamp, e.UpdatedAt.UTC())
	event.Props.SetText(ical.PropSummary, e.Title)
	if AllDay(e.StartsAt, e.EndsAt) {
		event.Props.SetDate(ical.PropDateTimeStart, e.StartsAt.In(time.Local))
		event.Props.SetDate(ical.PropDateTimeEnd, e.EndsAt.In(time.Local))
	} else {
		event.Props.SetDateTime(ical.PropDateTimeStart, e.StartsAt.UTC())
		event.Props.SetDateTime(ical.PropDateTimeEnd, e.EndsAt.UTC())
	}
	if e.Location != "" {
		event.Props.SetText(ical.PropLocation, e.Location)
	}
	event.Props.SetText(ical.PropDescription, withAttendeeTrailer(e.Description, e.Attendees))
	if link, err := url.Parse(fmt.Sprintf("%s/events/%d", a.WebBaseURL, e.ID)); err == nil {
		event.Props.SetURI(ical.PropURL, link)
	}
	event.Props.SetText(ical.PropTransparency, "OPAQUE")

	cal := newCalendar()
	cal.Children = append(cal.Children, event.Component)
	return cal
}

// AllDay reports whether an Appointment runs from local midnight to local
// midnight (in APP_TIMEZONE), which calendars show as an all-day event.
func AllDay(start, end time.Time) bool {
	return end.After(start) && localMidnight(start) && localMidnight(end)
}

func localMidnight(t time.Time) bool {
	t = t.In(time.Local)
	return t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 && t.Nanosecond() == 0
}

// AttendeeTrailerPrefix starts the generated last paragraph of an
// Appointment's DESCRIPTION that names its Attendees. Attendees are never sent
// as ATTENDEE properties, so phones don't treat Appointments as invitations.
const AttendeeTrailerPrefix = "Attendees: "

func withAttendeeTrailer(notes string, attendees []store.Attendee) string {
	trailer := AttendeeTrailerPrefix + "everyone in the Household"
	if len(attendees) > 0 {
		names := make([]string, len(attendees))
		for i, a := range attendees {
			names[i] = a.Name
		}
		trailer = AttendeeTrailerPrefix + strings.Join(names, ", ")
	}
	notes = strings.TrimRight(notes, "\r\n\t ")
	if notes == "" {
		return trailer
	}
	return notes + "\n\n" + trailer
}

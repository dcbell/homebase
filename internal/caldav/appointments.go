package caldav

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav/caldav"

	"homebase/internal/store"
)

// Appointments is the "homebase" collection: the Appointments that concern
// the Member (they are an Attendee, or there are no Attendees) within the
// Window, archived ones excluded.
//
// Devices can create, edit and delete Appointments here (see Put and Delete).
type Appointments struct {
	Store *store.Store
	// Saves changes Appointments under the app's rules.
	Saves AppointmentSaves
	// WebBaseURL is the public address each item links back to.
	WebBaseURL string
}

// AppointmentSaves are the shared Appointment save operations, acting as the
// connected Member in their Household.
type AppointmentSaves interface {
	CreateAppointment(ctx context.Context, m Member, input store.EventInput) (store.Event, error)
	UpdateAppointment(ctx context.Context, m Member, id int64, input store.EventInput) (store.Event, error)
	ArchiveAppointment(ctx context.Context, m Member, id int64) error
}

var _ Writable = Appointments{}

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
		objects[i] = a.object(e)
	}
	return objects, nil
}

func (a Appointments) object(e store.Event) Object {
	return Object{UID: appointmentUIDOf(e), Calendar: a.calendar(e), ModTime: e.UpdatedAt}
}

const appointmentUIDPrefix = "homebase-appointment-"

// AppointmentUID is the stable UID of a Homebase-originated Appointment.
func AppointmentUID(id int64) string {
	return fmt.Sprintf("%s%d", appointmentUIDPrefix, id)
}

// appointmentUIDOf is an Appointment's UID: the one the device chose when
// it was created on a device, otherwise the generated one.
func appointmentUIDOf(e store.Event) string {
	if e.CalendarUID != "" {
		return e.CalendarUID
	}
	return AppointmentUID(e.ID)
}

// appointmentID is the id in a generated UID.
func appointmentID(uid string) (int64, bool) {
	rest, ok := strings.CutPrefix(uid, appointmentUIDPrefix)
	if !ok {
		return 0, false
	}
	id, err := strconv.ParseInt(rest, 10, 64)
	if err != nil || id <= 0 || AppointmentUID(id) != uid {
		return 0, false
	}
	return id, true
}

// appointment returns the active Appointment with this UID that concerns
// the Member, or store.ErrNotFound.
func (a Appointments) appointment(ctx context.Context, m Member, uid string) (store.Event, error) {
	var e store.Event
	var err error
	if id, ok := appointmentID(uid); ok {
		e, err = a.Store.GetEvent(ctx, m.HouseholdID, id)
		if err == nil && e.CalendarUID != "" {
			err = store.ErrNotFound
		}
	} else {
		e, err = a.Store.FindEventByCalendarUID(ctx, m.HouseholdID, uid)
	}
	if err != nil {
		return store.Event{}, err
	}
	if e.Status == "archived" || !concerns(e, m.UserID) {
		return store.Event{}, store.ErrNotFound
	}
	return e, nil
}

func concerns(e store.Event, userID int64) bool {
	if len(e.AttendeeIDs) == 0 {
		return true
	}
	for _, id := range e.AttendeeIDs {
		if id == userID {
			return true
		}
	}
	return false
}

func (a Appointments) Find(ctx context.Context, m Member, uid string) (Object, bool, error) {
	e, err := a.appointment(ctx, m, uid)
	if errors.Is(err, store.ErrNotFound) {
		return Object{}, false, nil
	}
	if err != nil {
		return Object{}, false, err
	}
	return a.object(e), true, nil
}

// Put creates an Appointment for the Member alone, keeping the device's UID,
// or edits one's title, times, location and notes. Attendees never change
// from a device.
func (a Appointments) Put(ctx context.Context, m Member, uid string, comp *ical.Component, exists bool) error {
	input, err := eventInput(comp)
	if err != nil {
		return err
	}
	if !exists {
		if _, ours := appointmentID(uid); ours {
			// A generated UID that isn't in the Member's view: archived, or
			// not theirs. Devices never mint these.
			return refuse(caldav.PreconditionNoUIDConflict)
		}
		if _, err := a.Store.FindEventByCalendarUID(ctx, m.HouseholdID, uid); err == nil {
			// Another Member's device already used this UID.
			return refuse(caldav.PreconditionNoUIDConflict)
		} else if !errors.Is(err, store.ErrNotFound) {
			return err
		}
		input.CalendarUID = uid
		input.AttendeeIDs = []int64{m.UserID}
		_, err = a.Saves.CreateAppointment(ctx, m, input)
		return err
	}
	e, err := a.appointment(ctx, m, uid)
	if err != nil {
		return err
	}
	input.AttendeeIDs = nil
	_, err = a.Saves.UpdateAppointment(ctx, m, e.ID, input)
	return err
}

// Delete archives the Appointment.
func (a Appointments) Delete(ctx context.Context, m Member, uid string) error {
	e, err := a.appointment(ctx, m, uid)
	if err != nil {
		return err
	}
	return a.Saves.ArchiveAppointment(ctx, m, e.ID)
}

// eventInput reads an incoming VEVENT. An all-day event runs local midnight
// to local midnight; the generated Attendee trailer is stripped from notes.
func eventInput(comp *ical.Component) (store.EventInput, error) {
	var input store.EventInput
	var err error
	if input.Title, err = text(comp, ical.PropSummary); err != nil {
		return input, badRequest("SUMMARY: %v", err)
	}
	if strings.TrimSpace(input.Title) == "" {
		return input, badRequest("an Appointment needs a title")
	}
	if input.Location, err = text(comp, ical.PropLocation); err != nil {
		return input, badRequest("LOCATION: %v", err)
	}
	notes, err := text(comp, ical.PropDescription)
	if err != nil {
		return input, badRequest("DESCRIPTION: %v", err)
	}
	input.Description = withoutAttendeeTrailer(notes)

	start := comp.Props.Get(ical.PropDateTimeStart)
	if start == nil {
		return input, badRequest("an Appointment needs a DTSTART")
	}
	if input.StartsAt, err = dateTime(start); err != nil {
		return input, badRequest("DTSTART: %v", err)
	}
	allDay := isDate(start)
	switch end, duration := comp.Props.Get(ical.PropDateTimeEnd), comp.Props.Get(ical.PropDuration); {
	case end != nil:
		if input.EndsAt, err = dateTime(end); err != nil {
			return input, badRequest("DTEND: %v", err)
		}
	case duration != nil:
		d, err := duration.Duration()
		if err != nil {
			return input, badRequest("DURATION: %v", err)
		}
		if allDay && d%(24*time.Hour) == 0 {
			input.EndsAt = input.StartsAt.AddDate(0, 0, int(d/(24*time.Hour)))
		} else {
			input.EndsAt = input.StartsAt.Add(d)
		}
	case allDay:
		input.EndsAt = input.StartsAt.AddDate(0, 0, 1)
	}
	if !input.EndsAt.After(input.StartsAt) {
		return input, badRequest("an Appointment must end after it starts")
	}
	return input, nil
}

func (a Appointments) calendar(e store.Event) *ical.Calendar {
	event := ical.NewEvent()
	event.Props.SetText(ical.PropUID, appointmentUIDOf(e))
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

// withoutAttendeeTrailer drops the generated last paragraph from notes a
// device sends back.
func withoutAttendeeTrailer(notes string) string {
	notes = strings.ReplaceAll(notes, "\r\n", "\n")
	if strings.HasPrefix(notes, AttendeeTrailerPrefix) {
		return ""
	}
	if i := strings.LastIndex(notes, "\n\n"+AttendeeTrailerPrefix); i >= 0 && !strings.Contains(notes[i+2:], "\n\n") {
		return strings.TrimRight(notes[:i], "\n\t ")
	}
	return notes
}

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

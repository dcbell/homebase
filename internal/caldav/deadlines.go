package caldav

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/emersion/go-ical"

	"homebase/internal/store"
)

// Deadlines is the read-only "deadlines" collection: the due dates of Projects
// that aren't done or archived, and the next due date of each active Routine
// assigned to the Member or to nobody whose Generated Task doesn't exist yet
// (the in-app calendar's rule), within the Window. Every entry is free.
type Deadlines struct {
	Store *store.Store
	// WebBaseURL is the public address each item links back to.
	WebBaseURL string
}

func (Deadlines) Name() string        { return "deadlines" }
func (Deadlines) DisplayName() string { return "Homebase Deadlines" }
func (Deadlines) Component() string   { return ical.CompEvent }
func (Deadlines) Color() string       { return "#D97706FF" }

func (d Deadlines) Objects(ctx context.Context, m Member, now time.Time) ([]Object, error) {
	from, to := Window(now)
	projects, err := d.Store.ListProjectsDueBetween(ctx, m.HouseholdID, from, to)
	if err != nil {
		return nil, err
	}
	routines, err := d.Store.ListRoutinesDueFor(ctx, m.HouseholdID, m.UserID, from, to)
	if err != nil {
		return nil, err
	}
	objects := make([]Object, 0, len(projects)+len(routines))
	for _, p := range projects {
		uid := fmt.Sprintf("homebase-project-%d-due", p.ID)
		// A DATE column carries no zone: read its day as a local day.
		due := time.Date(p.DueDate.Year(), p.DueDate.Month(), p.DueDate.Day(), 0, 0, 0, 0, time.Local)
		event := d.event(uid, p.UpdatedAt, "Project due: "+p.Title, fmt.Sprintf("/projects/%d", p.ID))
		setAllDay(event, due)
		objects = append(objects, Object{UID: uid, Calendar: eventCalendar(event), ModTime: p.UpdatedAt})
	}
	for _, r := range routines {
		uid := fmt.Sprintf("homebase-routine-%d", r.ID)
		event := d.event(uid, r.UpdatedAt, "Routine: "+r.Title, fmt.Sprintf("/routines/%d", r.ID))
		if due := *r.NextDueAt; localMidnight(due) {
			setAllDay(event, due.In(time.Local))
		} else {
			event.Props.SetDateTime(ical.PropDateTimeStart, due.UTC())
		}
		objects = append(objects, Object{UID: uid, Calendar: eventCalendar(event), ModTime: r.UpdatedAt})
	}
	return objects, nil
}

func (d Deadlines) event(uid string, updatedAt time.Time, summary, page string) *ical.Event {
	event := ical.NewEvent()
	event.Props.SetText(ical.PropUID, uid)
	// DTSTAMP follows the item, never the request time, so the ETag only
	// changes when the item does.
	event.Props.SetDateTime(ical.PropDateTimeStamp, updatedAt.UTC())
	event.Props.SetText(ical.PropSummary, summary)
	if link, err := url.Parse(d.WebBaseURL + page); err == nil {
		event.Props.SetURI(ical.PropURL, link)
	}
	event.Props.SetText(ical.PropTransparency, "TRANSPARENT")
	return event
}

// setAllDay makes event span the local day that starts at day.
func setAllDay(event *ical.Event, day time.Time) {
	event.Props.SetDate(ical.PropDateTimeStart, day)
	event.Props.SetDate(ical.PropDateTimeEnd, day.AddDate(0, 0, 1))
}

func eventCalendar(event *ical.Event) *ical.Calendar {
	cal := newCalendar()
	cal.Children = append(cal.Children, event.Component)
	return cal
}

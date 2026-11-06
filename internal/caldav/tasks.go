package caldav

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/emersion/go-ical"

	"homebase/internal/store"
)

// Tasks is the "tasks" collection, which phones show as a to-do list (iOS
// puts it in Reminders): the open Tasks assigned to the Member or to nobody,
// undated or due within the Window. Done and archived Tasks leave it.
type Tasks struct {
	Store *store.Store
	// WebBaseURL is the public address each item links back to.
	WebBaseURL string
}

func (Tasks) Name() string        { return "tasks" }
func (Tasks) DisplayName() string { return "Homebase Tasks" }
func (Tasks) Component() string   { return ical.CompToDo }
func (Tasks) Color() string       { return "#16A34AFF" }

func (c Tasks) Objects(ctx context.Context, m Member, now time.Time) ([]Object, error) {
	from, to := Window(now)
	tasks, err := c.Store.ListOpenTasksConcerning(ctx, m.HouseholdID, m.UserID, from, to)
	if err != nil {
		return nil, err
	}
	objects := make([]Object, len(tasks))
	for i, t := range tasks {
		objects[i] = Object{UID: TaskUID(t.ID), Calendar: c.calendar(t), ModTime: t.UpdatedAt}
	}
	return objects, nil
}

// TaskUID is the stable UID of a Homebase Task.
func TaskUID(id int64) string {
	return fmt.Sprintf("homebase-task-%d", id)
}

// taskPriority maps a Task's priority to iCalendar PRIORITY (RFC 5545:
// 1 highest, 5 medium, 9 lowest).
var taskPriority = map[string]string{"high": "1", "normal": "5", "low": "9"}

func (c Tasks) calendar(t store.Task) *ical.Calendar {
	todo := ical.NewComponent(ical.CompToDo)
	todo.Props.SetText(ical.PropUID, TaskUID(t.ID))
	// DTSTAMP follows the Task, never the request time, so the ETag only
	// changes when the Task does.
	todo.Props.SetDateTime(ical.PropDateTimeStamp, t.UpdatedAt.UTC())
	todo.Props.SetText(ical.PropSummary, t.Title)
	todo.Props.SetText(ical.PropStatus, "NEEDS-ACTION")
	if t.DueAt != nil {
		if localMidnight(*t.DueAt) {
			todo.Props.SetDate(ical.PropDue, t.DueAt.In(time.Local))
		} else {
			todo.Props.SetDateTime(ical.PropDue, t.DueAt.UTC())
		}
	}
	if priority, ok := taskPriority[t.Priority]; ok {
		// PRIORITY is an INTEGER; SetText would mark it VALUE=TEXT.
		todo.Props.Set(&ical.Prop{Name: ical.PropPriority, Params: ical.Params{}, Value: priority})
	}
	if t.Notes != "" {
		todo.Props.SetText(ical.PropDescription, t.Notes)
	}
	if link, err := url.Parse(fmt.Sprintf("%s/tasks/%d", c.WebBaseURL, t.ID)); err == nil {
		todo.Props.SetURI(ical.PropURL, link)
	}

	cal := newCalendar()
	cal.Children = append(cal.Children, todo)
	return cal
}

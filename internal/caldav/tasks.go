package caldav

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/emersion/go-ical"

	"homebase/internal/store"
)

// Tasks is the "tasks" collection, which phones show as a to-do list (iOS
// puts it in Reminders): the open Tasks assigned to the Member or to nobody,
// undated or due within the Window. Done and archived Tasks leave it.
//
// Devices can tick, untick, create, edit and delete to-dos here (see Put and
// Delete).
type Tasks struct {
	Store *store.Store
	// Saves changes Tasks under the app's rules.
	Saves TaskSaves
	// WebBaseURL is the public address each item links back to.
	WebBaseURL string
}

// TaskSaves are the shared Task save operations, acting as the connected
// Member in their Household.
type TaskSaves interface {
	CreateTask(ctx context.Context, m CalendarLogin, input store.TaskInput) (store.Task, error)
	UpdateTask(ctx context.Context, m CalendarLogin, id int64, input store.TaskInput) (store.Task, error)
	CompleteTask(ctx context.Context, m CalendarLogin, id int64) (store.Task, error)
	ReopenTask(ctx context.Context, m CalendarLogin, id int64) (store.Task, error)
	ArchiveTask(ctx context.Context, m CalendarLogin, id int64) error
}

var _ Writable = Tasks{}

func (Tasks) Name() string        { return "tasks" }
func (Tasks) DisplayName() string { return "Homebase Tasks" }
func (Tasks) Component() string   { return ical.CompToDo }
func (Tasks) Color() string       { return "#16A34AFF" }

func (c Tasks) Objects(ctx context.Context, m CalendarLogin, now time.Time) ([]Object, error) {
	from, to := Window(now)
	tasks, err := c.Store.ListOpenTasksConcerning(ctx, m.HouseholdID, m.UserID, from, to)
	if err != nil {
		return nil, err
	}
	objects := make([]Object, len(tasks))
	for i, t := range tasks {
		objects[i] = c.object(t)
	}
	return objects, nil
}

func (c Tasks) object(t store.Task) Object {
	return Object{UID: c.uids().of(t.ID, t), Calendar: c.calendar(t), ModTime: t.UpdatedAt}
}

const taskUIDPrefix = "homebase-task-"

// TaskUID is the stable UID of a Homebase-originated Task.
func TaskUID(id int64) string {
	return generatedUID(taskUIDPrefix, id)
}

func (c Tasks) uids() uids[store.Task] {
	return uids[store.Task]{
		prefix:    taskUIDPrefix,
		get:       c.Store.GetTask,
		find:      c.Store.FindTaskByCalendarUID,
		deviceUID: func(t store.Task) string { return t.CalendarUID },
	}
}

// task returns the Task with this UID that concerns the Member, open or
// done, or store.ErrNotFound. Done Tasks stay findable so a device can
// untick a to-do it has just ticked.
func (c Tasks) task(ctx context.Context, m CalendarLogin, uid string) (store.Task, error) {
	t, err := c.uids().lookup(ctx, m.HouseholdID, uid)
	if err != nil {
		return store.Task{}, err
	}
	if t.Status == "archived" || (t.AssignedTo != nil && *t.AssignedTo != m.UserID) {
		return store.Task{}, store.ErrNotFound
	}
	return t, nil
}

func (c Tasks) Find(ctx context.Context, m CalendarLogin, uid string) (Object, bool, error) {
	t, err := c.task(ctx, m, uid)
	if errors.Is(err, store.ErrNotFound) {
		return Object{}, false, nil
	}
	if err != nil {
		return Object{}, false, err
	}
	return c.object(t), true, nil
}

// Put creates a Standalone Task assigned to the Member, keeping the device's
// UID, or changes one's title, due date and notes and ticks or unticks it.
// Other fields never change from a device.
func (c Tasks) Put(ctx context.Context, m CalendarLogin, uid string, comp *ical.Component, exists bool) error {
	in, err := readTodo(comp)
	if err != nil {
		return err
	}
	if !exists {
		if err := c.uids().checkNew(ctx, m.HouseholdID, uid); err != nil {
			return err
		}
		assignee := m.UserID
		t, err := c.Saves.CreateTask(ctx, m, store.TaskInput{
			Title:       in.title,
			Notes:       in.notes,
			DueAt:       in.due,
			AssignedTo:  &assignee,
			CalendarUID: uid,
		})
		if err == nil && in.done {
			_, err = c.Saves.CompleteTask(ctx, m, t.ID)
		}
		return err
	}

	t, err := c.task(ctx, m, uid)
	if err != nil {
		return err
	}
	wasDone := t.Status == "done"
	if wasDone && !in.done {
		if t, err = c.Saves.ReopenTask(ctx, m, t.ID); err != nil {
			return err
		}
	}
	if in.differsFrom(t) {
		if _, err := c.Saves.UpdateTask(ctx, m, t.ID, in.edit(t)); err != nil {
			return err
		}
	}
	if !wasDone && in.done {
		_, err = c.Saves.CompleteTask(ctx, m, t.ID)
	}
	return err
}

// Delete archives the Task.
func (c Tasks) Delete(ctx context.Context, m CalendarLogin, uid string) error {
	t, err := c.task(ctx, m, uid)
	if err != nil {
		return err
	}
	return c.Saves.ArchiveTask(ctx, m, t.ID)
}

// todo is what Homebase reads from an incoming VTODO.
type todo struct {
	title, notes string
	due          *time.Time
	done         bool
}

func readTodo(comp *ical.Component) (todo, error) {
	var in todo
	var err error
	if in.title, err = text(comp, ical.PropSummary); err != nil {
		return in, badRequest("SUMMARY: %v", err)
	}
	if strings.TrimSpace(in.title) == "" {
		return in, badRequest("a Task needs a title")
	}
	if in.notes, err = text(comp, ical.PropDescription); err != nil {
		return in, badRequest("DESCRIPTION: %v", err)
	}
	if p := comp.Props.Get(ical.PropDue); p != nil {
		due, err := dateTime(p)
		if err != nil {
			return in, badRequest("DUE: %v", err)
		}
		in.due = &due
	}
	status, err := text(comp, ical.PropStatus)
	if err != nil {
		return in, badRequest("STATUS: %v", err)
	}
	in.done = strings.EqualFold(status, "COMPLETED") || (status == "" && comp.Props.Get(ical.PropCompleted) != nil)
	return in, nil
}

func (in todo) differsFrom(t store.Task) bool {
	if strings.TrimSpace(in.title) != t.Title || in.notes != t.Notes {
		return true
	}
	if in.due == nil || t.DueAt == nil {
		return (in.due == nil) != (t.DueAt == nil)
	}
	return !in.due.Equal(*t.DueAt)
}

// edit is the Task with the device's title, due date and notes, still open.
func (in todo) edit(t store.Task) store.TaskInput {
	return store.TaskInput{
		ProjectID:              t.ProjectID,
		ProjectFolderID:        t.ProjectFolderID,
		RoutineID:              t.RoutineID,
		AssetID:                t.AssetID,
		AssetMaintenanceItemID: t.AssetMaintenanceItemID,
		AssignedTo:             t.AssignedTo,
		Title:                  in.title,
		Notes:                  in.notes,
		Status:                 t.Status,
		Priority:               t.Priority,
		DueAt:                  in.due,
	}
}

// taskPriority maps a Task's priority to iCalendar PRIORITY (RFC 5545:
// 1 highest, 5 medium, 9 lowest).
var taskPriority = map[string]string{"high": "1", "normal": "5", "low": "9"}

func (c Tasks) calendar(t store.Task) *ical.Calendar {
	todo := ical.NewComponent(ical.CompToDo)
	todo.Props.SetText(ical.PropUID, c.uids().of(t.ID, t))
	// DTSTAMP follows the Task, never the request time, so the ETag only
	// changes when the Task does.
	todo.Props.SetDateTime(ical.PropDateTimeStamp, t.UpdatedAt.UTC())
	todo.Props.SetText(ical.PropSummary, t.Title)
	if t.Status == "done" {
		todo.Props.SetText(ical.PropStatus, "COMPLETED")
		if t.CompletedAt != nil {
			todo.Props.SetDateTime(ical.PropCompleted, t.CompletedAt.UTC())
		}
	} else {
		todo.Props.SetText(ical.PropStatus, "NEEDS-ACTION")
	}
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

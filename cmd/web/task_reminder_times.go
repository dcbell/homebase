package main

import (
	"net/http"
	"strconv"
	"strings"

	"homebase/internal/store"
)

// taskReminderSlot is one row of a Task's Reminder Times on a form: an amount
// of minutes, hours or days before or after the due time, or at it, and for
// days optionally a time of day. A blank row is unused.
type taskReminderSlot struct {
	Amount    string
	Unit      string
	Direction string
	At        string
}

// taskReminderForm is the Reminder Times editor of a Task form: one set of
// rows for a due time of day and one for a date with none, either of which
// may be left out. Kind is the set shown; the browser shows the other when
// the due date changes kind, as the API resets Reminder Times on that switch.
type taskReminderForm struct {
	Known    bool
	Kind     string
	Timed    []taskReminderSlot
	DateOnly []taskReminderSlot
}

// taskReminderCreateForm prefills a create form with the API's defaults:
// both kinds, timed shown, or only date-only for forms that take a date.
func taskReminderCreateForm(defaults *store.ReminderTimeDefaults, dateOnly bool) taskReminderForm {
	if defaults == nil {
		return taskReminderForm{}
	}
	if dateOnly {
		return taskReminderForm{Known: true, Kind: "date_only", DateOnly: taskReminderSlots(defaults.DateOnlyTask)}
	}
	return taskReminderForm{Known: true, Kind: "timed", Timed: taskReminderSlots(defaults.TimedTask), DateOnly: taskReminderSlots(defaults.DateOnlyTask)}
}

// taskReminderEditForm shows a Task's own Reminder Times for its kind of due
// date, and the API's defaults for the other kind.
func taskReminderEditForm(task store.Task, defaults *store.ReminderTimeDefaults) taskReminderForm {
	form := taskReminderForm{Known: true, Kind: "timed"}
	if task.DueAt != nil && store.DateOnly(*task.DueAt) {
		form.Kind = "date_only"
	}
	if defaults != nil {
		form.Timed, form.DateOnly = taskReminderSlots(defaults.TimedTask), taskReminderSlots(defaults.DateOnlyTask)
	}
	if form.Kind == "date_only" {
		form.DateOnly = taskReminderSlots(task.ReminderTimes)
	} else {
		form.Timed = taskReminderSlots(task.ReminderTimes)
	}
	return form
}

// taskReminderSlots lays out Reminder Times as form rows, the timed form in
// the largest unit that divides it, with blank rows after them up to the
// most a list may have.
func taskReminderSlots(times []store.ReminderTime) []taskReminderSlot {
	slots := make([]taskReminderSlot, 0, store.MaxReminderTimes)
	for _, r := range times {
		offset, slot := r.Minutes, taskReminderSlot{Unit: "minutes"}
		if r.IsDateOnly() {
			offset, slot = r.Days, taskReminderSlot{Unit: "days", At: r.At}
		}
		switch {
		case offset < 0:
			slot.Direction, offset = "before", -offset
		case offset > 0:
			slot.Direction = "after"
		default:
			slot.Direction = "at"
			if !r.IsDateOnly() {
				slot.Unit = ""
			}
		}
		if !r.IsDateOnly() {
			switch {
			case offset != 0 && offset%(24*60) == 0:
				offset, slot.Unit = offset/(24*60), "days"
			case offset != 0 && offset%60 == 0:
				offset, slot.Unit = offset/60, "hours"
			}
		}
		if offset != 0 {
			slot.Amount = strconv.Itoa(offset)
		}
		slots = append(slots, slot)
	}
	for len(slots) < store.MaxReminderTimes {
		slots = append(slots, taskReminderSlot{})
	}
	return slots
}

// taskReminderTimesFromForm reads a Task form's Reminder Time rows, skipping
// blank ones, so clearing every row sends an empty list. A row with a time of
// day is days at that time; otherwise it is minutes from the due time. ok is
// false when the form carried no rows, so the field is left out for the API
// to apply or keep its own.
func taskReminderTimesFromForm(r *http.Request) (times []store.ReminderTime, ok bool) {
	if r.FormValue("reminder_times") == "" {
		return nil, false
	}
	field := func(name string, i int) string {
		if values := r.Form[name]; i < len(values) {
			return strings.TrimSpace(values[i])
		}
		return ""
	}
	times = []store.ReminderTime{}
	for i := range r.Form["reminder_amount"] {
		direction, amount := field("reminder_direction", i), 0
		if direction != "at" {
			n, err := strconv.Atoi(field("reminder_amount", i))
			if err != nil {
				continue
			}
			amount = n
			if direction == "before" {
				amount = -n
			}
		}
		if at := field("reminder_at", i); at != "" {
			times = append(times, store.ReminderTime{Days: amount, At: at})
			continue
		}
		perUnit := 1
		if minutes, known := reminderUnitMinutes[field("reminder_unit", i)]; known {
			perUnit = minutes
		}
		times = append(times, store.ReminderTime{Minutes: amount * perUnit})
	}
	return times, true
}

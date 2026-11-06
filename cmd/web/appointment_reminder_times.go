package main

import (
	"net/http"
	"strconv"
	"strings"

	"homebase/internal/store"
)

// reminderSlot is one row of an Appointment's Reminder Times on a form: how
// long before the start, in minutes, hours or days. A blank row is unused.
type reminderSlot struct {
	Before string
	Unit   string
}

// reminderUnitMinutes is how many minutes each form unit stands for.
var reminderUnitMinutes = map[string]int{"minutes": 1, "hours": 60, "days": 24 * 60}

// appointmentReminderSlots lays out an Appointment's Reminder Times as form
// rows, each in the largest unit that divides it, with blank rows after them
// up to the most a list may have.
func appointmentReminderSlots(times []store.ReminderTime) []reminderSlot {
	slots := make([]reminderSlot, 0, store.MaxReminderTimes)
	for _, r := range times {
		before, unit := -r.Minutes, "minutes"
		switch {
		case before != 0 && before%(24*60) == 0:
			before, unit = before/(24*60), "days"
		case before != 0 && before%60 == 0:
			before, unit = before/60, "hours"
		}
		slots = append(slots, reminderSlot{Before: strconv.Itoa(before), Unit: unit})
	}
	for len(slots) < store.MaxReminderTimes {
		slots = append(slots, reminderSlot{})
	}
	return slots
}

// appointmentPersonalReminderTimes are the Reminder Times the caller's
// Personal Reminder Times editor on an Appointment page starts from: their
// own, or the Appointment's shared ones while they use those.
func appointmentPersonalReminderTimes(event store.Event) []store.ReminderTime {
	if personal := event.PersonalReminderTimes; personal != nil && !personal.UsingShared {
		return personal.ReminderTimes
	}
	return event.ReminderTimes
}

// appointmentReminderTimesFromForm reads an Appointment form's Reminder Time
// rows as minutes before the start, skipping blank rows, so clearing every
// row sends an empty list. ok is false when the form carried no rows, so the
// field is left out for the API to apply or keep its own.
func appointmentReminderTimesFromForm(r *http.Request) (times []store.ReminderTime, ok bool) {
	if r.FormValue("reminder_times") == "" {
		return nil, false
	}
	units := r.Form["reminder_unit"]
	times = []store.ReminderTime{}
	for i, raw := range r.Form["reminder_before"] {
		before, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil {
			continue
		}
		perUnit := 1
		if i < len(units) {
			if minutes, known := reminderUnitMinutes[units[i]]; known {
				perUnit = minutes
			}
		}
		times = append(times, store.ReminderTime{Minutes: -before * perUnit})
	}
	return times, true
}

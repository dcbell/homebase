package main

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"homebase/internal/store"
)

// The kinds of due date a Reminder Times editor has a set of rows for.
const (
	reminderKindTimed    = "timed"
	reminderKindDateOnly = "date_only"
)

// The directions a Reminder Time row can take from the due time.
const (
	reminderBefore = "before"
	reminderAt     = "at"
	reminderAfter  = "after"
)

// reminderUnitMinutes is how many minutes each form unit stands for.
var reminderUnitMinutes = map[string]int{"minutes": 1, "hours": 60, "days": 24 * 60}

// reminderTimeRow is one row of Reminder Times on a form: an amount of
// minutes, hours or days before or after the due time, or 0 at it, and for
// days optionally a time of day. A row with no amount is unused.
type reminderTimeRow struct {
	Amount    string
	Unit      string
	Direction string
	At        string
}

// reminderTimesForm is the Reminder Times editor of a form: one set of rows
// for a due time of day and one for a date with none, either of which may be
// left out. Kind is the set shown; the browser shows the other when the due
// date changes kind, as the API resets Reminder Times on that switch.
type reminderTimesForm struct {
	Known bool
	Kind  string
	// Appointment limits rows to before or at the start, with no time of
	// day, as an Appointment's Reminder Times are.
	Appointment bool
	Timed       []reminderTimeRow
	DateOnly    []reminderTimeRow
}

// taskReminderCreateForm prefills a create form with the API's defaults:
// both kinds, timed shown, or only date-only for forms that take a date.
func taskReminderCreateForm(defaults *store.ReminderTimeDefaults, dateOnly bool) reminderTimesForm {
	if defaults == nil {
		return reminderTimesForm{}
	}
	if dateOnly {
		return reminderTimesForm{Known: true, Kind: reminderKindDateOnly, DateOnly: reminderTimeRows(defaults.DateOnlyTask)}
	}
	return reminderTimesForm{Known: true, Kind: reminderKindTimed, Timed: reminderTimeRows(defaults.TimedTask), DateOnly: reminderTimeRows(defaults.DateOnlyTask)}
}

// reminderEditForm shows the Reminder Times of a Task, Routine or
// Maintenance Schedule in the set for its kind of due date (next due date for
// a Routine or Maintenance Schedule), and the API's defaults in the other.
func reminderEditForm(times []store.ReminderTime, dateOnly bool, defaults *store.ReminderTimeDefaults) reminderTimesForm {
	form := reminderTimesForm{Known: true, Kind: reminderKindTimed}
	if dateOnly {
		form.Kind = reminderKindDateOnly
	}
	if defaults != nil {
		form.Timed, form.DateOnly = reminderTimeRows(defaults.TimedTask), reminderTimeRows(defaults.DateOnlyTask)
	}
	if dateOnly {
		form.DateOnly = reminderTimeRows(times)
	} else {
		form.Timed = reminderTimeRows(times)
	}
	return form
}

// taskPersonalReminderForm is the caller's Personal Reminder Times editor on
// a Task page, for the Task's kind of due date only, as that page doesn't
// change it. It starts from their own, or from the Task's shared Reminder
// Times while they use those.
func taskPersonalReminderForm(task store.Task) reminderTimesForm {
	rows := reminderTimeRows(personalOrShared(task.PersonalReminderTimes, task.ReminderTimes))
	if task.DateOnly {
		return reminderTimesForm{Known: true, Kind: reminderKindDateOnly, DateOnly: rows}
	}
	return reminderTimesForm{Known: true, Kind: reminderKindTimed, Timed: rows}
}

// appointmentReminderCreateForm prefills an Appointment create form with the
// API's defaults.
func appointmentReminderCreateForm(defaults *store.ReminderTimeDefaults) reminderTimesForm {
	if defaults == nil {
		return reminderTimesForm{}
	}
	return appointmentReminderForm(defaults.Appointment)
}

// appointmentReminderEditForm shows an Appointment's Reminder Times.
func appointmentReminderEditForm(event store.Event) reminderTimesForm {
	return appointmentReminderForm(event.ReminderTimes)
}

// appointmentPersonalReminderForm is the caller's Personal Reminder Times
// editor on an Appointment page, starting from their own, or from the
// Appointment's shared ones while they use those.
func appointmentPersonalReminderForm(event store.Event) reminderTimesForm {
	return appointmentReminderForm(personalOrShared(event.PersonalReminderTimes, event.ReminderTimes))
}

func appointmentReminderForm(times []store.ReminderTime) reminderTimesForm {
	return reminderTimesForm{Known: true, Kind: reminderKindTimed, Appointment: true, Timed: reminderTimeRows(times)}
}

// personalOrShared are the caller's own Reminder Times, or the shared ones
// while they use those.
func personalOrShared(personal *store.PersonalReminderTimes, shared []store.ReminderTime) []store.ReminderTime {
	if personal != nil && !personal.UsingShared {
		return personal.ReminderTimes
	}
	return shared
}

// reminderTimeRows lays out Reminder Times as form rows, the timed form in
// the largest unit that divides it and at the due time as 0, with blank rows
// after them up to the most a list may have.
func reminderTimeRows(times []store.ReminderTime) []reminderTimeRow {
	rows := make([]reminderTimeRow, 0, store.MaxReminderTimes)
	for _, r := range times {
		offset, row := r.Minutes, reminderTimeRow{Unit: "minutes"}
		if r.IsDateOnly() {
			offset, row = r.Days, reminderTimeRow{Unit: "days", At: r.At}
		}
		switch {
		case offset < 0:
			row.Direction, offset = reminderBefore, -offset
		case offset > 0:
			row.Direction = reminderAfter
		default:
			row.Direction = reminderAt
		}
		if !r.IsDateOnly() {
			switch {
			case offset != 0 && offset%(24*60) == 0:
				offset, row.Unit = offset/(24*60), "days"
			case offset != 0 && offset%60 == 0:
				offset, row.Unit = offset/60, "hours"
			}
		}
		row.Amount = strconv.Itoa(offset)
		rows = append(rows, row)
	}
	for len(rows) < store.MaxReminderTimes {
		rows = append(rows, reminderTimeRow{})
	}
	return rows
}

// reminderTimesFromForm reads a form's Reminder Time rows, skipping those
// with no amount, so clearing every row sends an empty list. A row at the due
// time is 0 whatever its amount. A row with a time of day is days at that
// time; otherwise it is minutes from the due time. ok is false when the form
// carried no rows, so the field is left out for the API to apply or keep its
// own.
func reminderTimesFromForm(r *http.Request) (times []store.ReminderTime, ok bool) {
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
		amount, err := strconv.Atoi(field("reminder_amount", i))
		if err != nil {
			continue
		}
		switch field("reminder_direction", i) {
		case reminderAt:
			amount = 0
		case reminderBefore:
			amount = -amount
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

// setPersonalReminderTimes forwards "Use your own Reminder Times": the
// caller's Personal Reminder Times on the Task or Appointment under prefix,
// "/tasks" or "/events". Clearing every row sends an empty list, meaning no
// Reminders for them.
func (s *webServer) setPersonalReminderTimes(prefix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page := prefix + "/" + url.PathEscape(r.PathValue("id"))
		if err := r.ParseForm(); err != nil {
			http.Redirect(w, r, page+"?error=form", http.StatusSeeOther)
			return
		}
		times, ok := reminderTimesFromForm(r)
		if !ok {
			times = []store.ReminderTime{}
		}
		payload := map[string]any{"reminder_times": times}
		if err := s.apiJSON(r, http.MethodPut, "/api/v1"+page+"/personal-reminder-times", payload, nil); err != nil {
			http.Redirect(w, r, page+"?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, page, http.StatusSeeOther)
	}
}

// useSharedReminderTimes forwards "Use shared": the caller goes back to the
// shared Reminder Times of the Task or Appointment under prefix.
func (s *webServer) useSharedReminderTimes(prefix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page := prefix + "/" + url.PathEscape(r.PathValue("id"))
		if err := s.apiJSON(r, http.MethodDelete, "/api/v1"+page+"/personal-reminder-times", nil, nil); err != nil {
			http.Redirect(w, r, page+"?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, page, http.StatusSeeOther)
	}
}

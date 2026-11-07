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

// reminderTimeRow is one Reminder Time on a form, shown in a frame: an
// amount of minutes, hours or days before or after the due time, or 0 at it,
// and for days optionally a time of day. A row with no amount is unused.
type reminderTimeRow struct {
	Amount    string
	Unit      string
	Direction string
	At        string
	// Summary says the row in plain language, as its closed frame shows it.
	Summary string
	// Controls are which of the frame's controls apply to the row as it
	// stands; the frame marks each, and keeps the value of one that doesn't.
	Controls reminderControls
}

// rendered is row as a frame in a set of kind, of an Appointment or not,
// shows it: with its summary, and which of its controls apply. An "at" row
// with no amount gets 0, since a row with none is dropped when saved; only
// rendering fills it, so blanking it on a form still drops the row.
func (row reminderTimeRow) rendered(kind string, appointment bool) reminderTimeRow {
	if row.Direction == reminderAt && row.Amount == "" {
		row.Amount = "0"
	}
	row.Summary = row.summary(kind, appointment)
	row.Controls = row.controls(kind, appointment)
	return row
}

// reminderTimeSet is the frames of one kind of due date in an editor.
type reminderTimeSet struct {
	Kind        string
	Shown       bool
	Appointment bool
	Rows        []reminderTimeRow
	// New is the frame "Add reminder" adds.
	New reminderTimeRow
	// CanAdd is whether the list has room for another Reminder Time.
	CanAdd bool
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

// Sets are the editor's sets of frames, the timed one first, leaving out a
// kind the form has none for.
func (f reminderTimesForm) Sets() []reminderTimeSet {
	sets := []reminderTimeSet{}
	if f.Timed != nil {
		sets = append(sets, f.set(reminderKindTimed, f.Timed))
	}
	if f.DateOnly != nil {
		sets = append(sets, f.set(reminderKindDateOnly, f.DateOnly))
	}
	return sets
}

// Shown is the set the editor shows.
func (f reminderTimesForm) Shown() reminderTimeSet {
	for _, set := range f.Sets() {
		if set.Shown {
			return set
		}
	}
	return reminderTimeSet{}
}

func (f reminderTimesForm) set(kind string, rows []reminderTimeRow) reminderTimeSet {
	appointment := f.Appointment && kind == reminderKindTimed
	set := reminderTimeSet{Kind: kind, Shown: f.Kind == kind, Appointment: appointment, CanAdd: len(rows) < store.MaxReminderTimes}
	set.New = reminderTimeRow{Amount: "1", Unit: "hours", Direction: reminderBefore}
	if kind == reminderKindDateOnly {
		set.New = reminderTimeRow{Amount: "1", Unit: "days", Direction: reminderBefore, At: "08:00"}
	}
	set.New = set.New.rendered(kind, appointment)
	set.Rows = make([]reminderTimeRow, len(rows))
	for i, row := range rows {
		set.Rows[i] = row.rendered(kind, appointment)
	}
	return set
}

// summary says a row in plain language, such as "24 hours before", "1 day
// after at 08:00" or "At the start", leaving out any control that doesn't
// apply to it.
func (row reminderTimeRow) summary(kind string, appointment bool) string {
	controls := row.controls(kind, appointment)
	at := ""
	if controls.At {
		at = row.At
	}
	if !controls.Amount {
		switch {
		case appointment:
			return "At the start"
		case at != "":
			return "On the due date at " + at
		case kind == reminderKindDateOnly:
			return "On the due date"
		}
		return "At the due time"
	}
	amount, _ := strconv.Atoi(row.Amount)
	unit := row.Unit
	if amount == 1 {
		unit = strings.TrimSuffix(unit, "s")
	}
	text := strconv.Itoa(amount) + " " + unit + " " + row.Direction
	switch {
	case appointment:
		text += " the start"
	case at != "":
		text += " at " + at
	}
	return text
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
// the largest unit that divides it and at the due time as 0.
func reminderTimeRows(times []store.ReminderTime) []reminderTimeRow {
	rows := make([]reminderTimeRow, 0, len(times))
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
	return rows
}

// reminderTimeRowFrame renders one Reminder Time frame from the query, as
// the editor's pages render it, so the editor's script can ask for a fresh
// copy of a row as it changes. The query names the set's kind
// (reminder_kind), whether it is an Appointment's (appointment=1), and the
// row's fields as the frame submits them. It needs no record and changes
// nothing.
func (s *webServer) reminderTimeRowFrame(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	kind := reminderKindTimed
	if q.Get("reminder_kind") == reminderKindDateOnly {
		kind = reminderKindDateOnly
	}
	appointment := q.Get("appointment") == "1" && kind == reminderKindTimed
	row := reminderTimeRow{
		Amount:    strings.TrimSpace(q.Get("reminder_amount")),
		Unit:      strings.TrimSpace(q.Get("reminder_unit")),
		Direction: strings.TrimSpace(q.Get("reminder_direction")),
		At:        strings.TrimSpace(q.Get("reminder_at")),
	}
	if kind == reminderKindDateOnly {
		row.Unit = "days"
	}
	frame := map[string]any{"Kind": kind, "Appointment": appointment, "Row": row.rendered(kind, appointment)}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "reminderTimeFrame", frame); err != nil {
		s.logger.Error("render Reminder Time frame", "error", err)
	}
}

// reminderControls are which of a Reminder Time row's controls apply to it,
// following its set and its direction and unit. A control that doesn't apply
// never counts: whatever value it holds is left out of the row's summary and
// of what is saved.
type reminderControls struct {
	// Amount is how long before or after; a row at the due time has none.
	Amount bool
	// At is the time of day the Reminder is sent on its day.
	At bool
	// AtRequired is whether the row can't be saved without its time of
	// day, as a date-only Reminder Time can't.
	AtRequired bool
}

// controls says which controls apply to row in a set of kind, of an
// Appointment or not:
//
//   - an Appointment's rows never have a time of day;
//   - a timed row may have one at the due date, or when it counts days;
//   - a date-only row always counts days, and needs one.
func (row reminderTimeRow) controls(kind string, appointment bool) reminderControls {
	c := reminderControls{Amount: row.Direction != reminderAt}
	switch {
	case appointment:
	case kind == reminderKindDateOnly:
		c.At, c.AtRequired = true, true
	default:
		c.At = row.Direction == reminderAt || row.Unit == "days"
	}
	return c
}

// reminderTimesFromForm reads a form's Reminder Time rows as the set of the
// kind the form names would have them, of an Appointment or not, skipping
// rows with no amount, so clearing every row sends an empty list. A row at
// the due time is 0 whatever its amount. A row with a time of day that
// applies to it is days at that time; otherwise it is minutes from the due
// time. A form naming no kind is read as timed. ok is false when the form
// carried no rows, so the field is left out for the API to apply or keep its
// own.
func reminderTimesFromForm(r *http.Request, appointment bool) (times []store.ReminderTime, ok bool) {
	if r.FormValue("reminder_times") == "" {
		return nil, false
	}
	kind := reminderKindTimed
	if r.FormValue("reminder_kind") == reminderKindDateOnly {
		kind = reminderKindDateOnly
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
		row := reminderTimeRow{Unit: field("reminder_unit", i), Direction: field("reminder_direction", i), At: field("reminder_at", i)}
		controls := row.controls(kind, appointment)
		switch {
		case !controls.Amount:
			amount = 0
		case row.Direction == reminderBefore:
			amount = -amount
		}
		if controls.At && row.At != "" {
			times = append(times, store.ReminderTime{Days: amount, At: row.At})
			continue
		}
		perUnit := 1
		if minutes, known := reminderUnitMinutes[row.Unit]; known {
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
		times, ok := reminderTimesFromForm(r, prefix == "/events")
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

package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ReminderTime is one moment, set relative to when something is due, at which
// a Reminder is sent. It takes one of two forms:
//
//   - Timed: Minutes from the due time. Negative is before it, zero is at it,
//     positive is after it.
//   - Date-only: Days from the due time's local date, at the local time of
//     day At ("08:00"). A Task due on a date with no time of day uses only
//     this form; a timed Task may use it too, such as for Overdue at 08:00
//     the next morning.
//
// At is empty for the timed form and set for the date-only form. In JSON
// the timed form is {"minutes": -120} and the date-only form
// {"days": -1, "at": "08:00"}.
type ReminderTime struct {
	Minutes int    `json:"minutes"`
	Days    int    `json:"days"`
	At      string `json:"at"`
}

// MarshalJSON writes only the fields of r's form.
func (r ReminderTime) MarshalJSON() ([]byte, error) {
	if r.IsDateOnly() {
		return json.Marshal(struct {
			Days int    `json:"days"`
			At   string `json:"at"`
		}{r.Days, r.At})
	}
	return json.Marshal(struct {
		Minutes int `json:"minutes"`
	}{r.Minutes})
}

// MaxReminderTimes is how many Reminder Times one list may have.
const MaxReminderTimes = 5

// maxReminderOffsetDays is how far before or after the due time a Reminder
// Time may be.
const maxReminderOffsetDays = 30

// The built-in Reminder Times every new Task and Appointment starts with.
var (
	// TimedTaskReminderTimes: Due Soon 24 hours before, Due Now at the due
	// time, and Overdue at 08:00 the morning after.
	TimedTaskReminderTimes = []ReminderTime{{Minutes: -24 * 60}, {Minutes: 0}, {Days: 1, At: "08:00"}}
	// DateOnlyTaskReminderTimes: Due Soon, Due Now and Overdue at 08:00 the
	// day before, on the day, and the day after.
	DateOnlyTaskReminderTimes = []ReminderTime{{Days: -1, At: "08:00"}, {Days: 0, At: "08:00"}, {Days: 1, At: "08:00"}}
	// AppointmentReminderTimes: 24 hours before, an hour before, and at the
	// start.
	AppointmentReminderTimes = []ReminderTime{{Minutes: -24 * 60}, {Minutes: -60}, {Minutes: 0}}
)

// DefaultTaskReminderTimes are the built-in Reminder Times for a Task due at
// due: the date-only ones for a due date with no time of day, otherwise the
// timed ones.
func DefaultTaskReminderTimes(due time.Time) []ReminderTime {
	if DateOnly(due) {
		return DateOnlyTaskReminderTimes
	}
	return TimedTaskReminderTimes
}

// DateOnly reports whether due is a due date with no time of day: exactly
// local midnight in time.Local, which the API sets to APP_TIMEZONE.
func DateOnly(due time.Time) bool {
	local := due.In(time.Local)
	return local.Hour() == 0 && local.Minute() == 0 && local.Second() == 0 && local.Nanosecond() == 0
}

// IsDateOnly reports whether r is in the date-only form.
func (r ReminderTime) IsDateOnly() bool {
	return r.At != ""
}

// Stage is the Reminder's stage: Due Soon before the due time, Due Now at it,
// Overdue after it.
func (r ReminderTime) Stage() string {
	offset := r.Minutes
	if r.IsDateOnly() {
		offset = r.Days
	}
	switch {
	case offset < 0:
		return ReminderDueSoon
	case offset > 0:
		return ReminderOverdue
	default:
		return ReminderDueNow
	}
}

// SendAt is when the Reminder for something due at due is sent.
func (r ReminderTime) SendAt(due time.Time) time.Time {
	if !r.IsDateOnly() {
		return due.Add(time.Duration(r.Minutes) * time.Minute)
	}
	at, _ := time.Parse("15:04", r.At)
	local := due.In(time.Local)
	return time.Date(local.Year(), local.Month(), local.Day()+r.Days, at.Hour(), at.Minute(), 0, 0, time.Local)
}

// ValidateReminderTimes checks a list of Reminder Times for a subject
// (SubjectTask or SubjectAppointment) whose due date is dateOnly or timed:
// at most MaxReminderTimes, no duplicates, none more than 30 days from the
// due time, none after an Appointment starts, and each in a form that fits
// the due date. An empty list is valid and means no Reminders.
func ValidateReminderTimes(times []ReminderTime, subject string, dateOnly bool) error {
	if len(times) > MaxReminderTimes {
		return fmt.Errorf("at most %d reminder times are allowed", MaxReminderTimes)
	}
	seen := map[ReminderTime]bool{}
	for _, r := range times {
		if seen[r] {
			return errors.New("reminder times must not repeat")
		}
		seen[r] = true
		if err := validateReminderTime(r, subject, dateOnly); err != nil {
			return err
		}
	}
	return nil
}

func validateReminderTime(r ReminderTime, subject string, dateOnly bool) error {
	if r.IsDateOnly() {
		if subject == SubjectAppointment {
			return errors.New("appointment reminder times must be in minutes")
		}
		if r.Minutes != 0 {
			return errors.New("a reminder time is either minutes or days at a time of day, not both")
		}
		if _, err := time.Parse("15:04", r.At); err != nil || len(r.At) != len("15:04") {
			return errors.New("reminder time of day must be HH:MM")
		}
		if r.Days < -maxReminderOffsetDays || r.Days > maxReminderOffsetDays {
			return fmt.Errorf("reminder times must be within %d days of the due time", maxReminderOffsetDays)
		}
	} else {
		if dateOnly {
			return errors.New("reminder times for a date with no time of day must be days at a time of day")
		}
		if r.Days != 0 {
			return errors.New("reminder time in days needs a time of day")
		}
		if limit := maxReminderOffsetDays * 24 * 60; r.Minutes < -limit || r.Minutes > limit {
			return fmt.Errorf("reminder times must be within %d days of the due time", maxReminderOffsetDays)
		}
	}
	if subject == SubjectAppointment && r.Stage() == ReminderOverdue {
		return errors.New("appointment reminder times must be before or at the start")
	}
	return nil
}

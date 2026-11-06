package store_test

import (
	"testing"
	"time"

	"homebase/internal/store"
)

func minutes(m int) store.ReminderTime { return store.ReminderTime{Minutes: m} }

func daysAt(days int, at string) store.ReminderTime {
	return store.ReminderTime{Days: days, At: at}
}

func TestBuiltInReminderTimesAreValid(t *testing.T) {
	cases := []struct {
		name     string
		times    []store.ReminderTime
		subject  string
		dateOnly bool
	}{
		{"timed Task", store.TimedTaskReminderTimes, store.SubjectTask, false},
		{"date-only Task", store.DateOnlyTaskReminderTimes, store.SubjectTask, true},
		{"Appointment", store.AppointmentReminderTimes, store.SubjectAppointment, false},
	}
	for _, c := range cases {
		if err := store.ValidateReminderTimes(c.times, c.subject, c.dateOnly); err != nil {
			t.Errorf("%s defaults: %v", c.name, err)
		}
	}
}

func TestReminderStageFollowsWhenItIsSent(t *testing.T) {
	timed := time.Date(2030, time.January, 17, 15, 0, 0, 0, time.Local)
	dateOnly := time.Date(2030, time.January, 17, 0, 0, 0, 0, time.Local)
	cases := []struct {
		name string
		r    store.ReminderTime
		due  time.Time
		want string
	}{
		{"a minute before the due time", minutes(-1), timed, store.ReminderDueSoon},
		{"at the due time", minutes(0), timed, store.ReminderDueNow},
		{"a minute after the due time", minutes(1), timed, store.ReminderOverdue},
		{"on a timed due date, a minute before its time", daysAt(0, "14:59"), timed, store.ReminderDueSoon},
		{"on a timed due date, at its time", daysAt(0, "15:00"), timed, store.ReminderDueNow},
		{"on a timed due date, at its time, due part way through the minute", daysAt(0, "15:00"), timed.Add(30 * time.Second), store.ReminderDueNow},
		{"on a timed due date, a minute after its time", daysAt(0, "15:01"), timed, store.ReminderOverdue},
		{"the day before a timed due date, after its time", daysAt(-1, "23:59"), timed, store.ReminderDueSoon},
		{"the day after a timed due date, before its time", daysAt(1, "00:00"), timed, store.ReminderOverdue},
		{"first minute of a date-only due date", daysAt(0, "00:00"), dateOnly, store.ReminderDueNow},
		{"last minute of a date-only due date", daysAt(0, "23:59"), dateOnly, store.ReminderDueNow},
		{"the day before a date-only due date", daysAt(-1, "23:59"), dateOnly, store.ReminderDueSoon},
		{"the day after a date-only due date", daysAt(1, "00:00"), dateOnly, store.ReminderOverdue},
	}
	for _, c := range cases {
		if got := c.r.Stage(c.due); got != c.want {
			t.Errorf("%s: Stage = %s, want %s", c.name, got, c.want)
		}
	}
}

func TestValidReminderTimes(t *testing.T) {
	cases := []struct {
		name     string
		times    []store.ReminderTime
		subject  string
		dateOnly bool
	}{
		{"none", nil, store.SubjectTask, false},
		{"five", []store.ReminderTime{minutes(-60), minutes(-30), minutes(0), minutes(30), minutes(60)}, store.SubjectTask, false},
		{"30 days either side", []store.ReminderTime{minutes(-30 * 24 * 60), minutes(30 * 24 * 60)}, store.SubjectTask, false},
		{"30 days either side, date-only", []store.ReminderTime{daysAt(-30, "23:59"), daysAt(30, "00:00")}, store.SubjectTask, true},
		{"days at a time on a timed Task", []store.ReminderTime{daysAt(2, "08:00")}, store.SubjectTask, false},
		{"same day at different times", []store.ReminderTime{daysAt(0, "08:00"), daysAt(0, "18:00")}, store.SubjectTask, true},
		{"Appointment before and at the start", []store.ReminderTime{minutes(-120), minutes(0)}, store.SubjectAppointment, false},
	}
	for _, c := range cases {
		if err := store.ValidateReminderTimes(c.times, c.subject, c.dateOnly); err != nil {
			t.Errorf("%s: %v", c.name, err)
		}
	}
}

func TestInvalidReminderTimes(t *testing.T) {
	cases := []struct {
		name     string
		times    []store.ReminderTime
		subject  string
		dateOnly bool
	}{
		{"six", []store.ReminderTime{minutes(-50), minutes(-40), minutes(-30), minutes(-20), minutes(-10), minutes(0)}, store.SubjectTask, false},
		{"duplicate", []store.ReminderTime{minutes(-60), minutes(-60)}, store.SubjectTask, false},
		{"duplicate, date-only", []store.ReminderTime{daysAt(-1, "08:00"), daysAt(-1, "08:00")}, store.SubjectTask, true},
		{"more than 30 days before", []store.ReminderTime{minutes(-30*24*60 - 1)}, store.SubjectTask, false},
		{"more than 30 days after", []store.ReminderTime{minutes(30*24*60 + 1)}, store.SubjectTask, false},
		{"more than 30 days before, date-only", []store.ReminderTime{daysAt(-31, "08:00")}, store.SubjectTask, true},
		{"more than 30 days after, date-only", []store.ReminderTime{daysAt(31, "08:00")}, store.SubjectTask, true},
		{"after an Appointment starts", []store.ReminderTime{minutes(1)}, store.SubjectAppointment, false},
		{"days at a time on an Appointment", []store.ReminderTime{daysAt(-1, "08:00")}, store.SubjectAppointment, false},
		{"minutes on a date-only Task", []store.ReminderTime{minutes(-60)}, store.SubjectTask, true},
		{"minutes and days at once", []store.ReminderTime{{Minutes: -60, Days: -1, At: "08:00"}}, store.SubjectTask, false},
		{"days without a time of day", []store.ReminderTime{{Days: -1}}, store.SubjectTask, true},
		{"time of day out of range", []store.ReminderTime{daysAt(-1, "24:00")}, store.SubjectTask, true},
		{"time of day not HH:MM", []store.ReminderTime{daysAt(-1, "8am")}, store.SubjectTask, true},
	}
	for _, c := range cases {
		if err := store.ValidateReminderTimes(c.times, c.subject, c.dateOnly); err == nil {
			t.Errorf("%s: accepted %+v", c.name, c.times)
		}
	}
}

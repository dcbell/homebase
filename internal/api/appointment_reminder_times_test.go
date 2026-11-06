package api_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"homebase/internal/apitest"
	"homebase/internal/store"
)

// minutesBefore is a timed Reminder Time, as the API takes it, the given
// number of minutes before the start.
func minutesBefore(minutes int) map[string]any {
	return map[string]any{"minutes": -minutes}
}

// appointmentBody is an hour-long Appointment starting at starts, with the
// given Attendees unless nil, and Reminder Times unless nil.
func appointmentBody(starts time.Time, attendees []int64, reminderTimes []map[string]any) map[string]any {
	body := map[string]any{
		"title":     "Dentist",
		"starts_at": starts.Format(time.RFC3339),
		"ends_at":   starts.Add(time.Hour).Format(time.RFC3339),
	}
	if attendees != nil {
		body["attendee_ids"] = attendees
	}
	if reminderTimes != nil {
		body["reminder_times"] = reminderTimes
	}
	return body
}

func createAppointmentWithReminderTimes(t *testing.T, c *apitest.Client, starts time.Time, attendees []int64, reminderTimes []map[string]any) store.Event {
	t.Helper()
	return apitest.Decode[store.Event](t, c.Post("/api/v1/events", appointmentBody(starts, attendees, reminderTimes)).Expect(http.StatusCreated))
}

func TestAppointmentRemindsEveryAttendeeAtItsReminderTimes(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	owner, member := household.Owner.User.ID, household.Member.User.ID
	createAppointmentWithReminderTimes(t, household.Owner, at(17, 9, 0), []int64{owner, member},
		[]map[string]any{minutesBefore(2 * 60), minutesBefore(15)})

	sendRemindersAt(t, h, at(17, 6, 59))
	expectAppointmentReminders(t, household.Member)

	sendRemindersAt(t, h, at(17, 9, 0))
	for _, c := range []*apitest.Client{household.Owner, household.Member} {
		got := reminderTitlesOf(t, c, "appointment_reminder")
		if want := []string{"Appointment in 2 hours", "Appointment in 15 minutes"}; fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("%s Reminder titles = %q, want %q", c.User.Name, got, want)
		}
	}
}

func TestWholeHouseholdAppointmentRemindsEveryMemberAtItsReminderTimes(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	appt := createAppointmentWithReminderTimes(t, household.Owner, at(17, 9, 0), nil,
		[]map[string]any{minutesBefore(30)})

	sendRemindersAt(t, h, at(17, 9, 0))
	expectAppointmentReminders(t, household.Owner, stages(appt.ID, "due_soon")...)
	expectAppointmentReminders(t, household.Member, stages(appt.ID, "due_soon")...)
}

func TestAppointmentWithoutReminderTimesGetsTheDefaults(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	appt := createAppointmentWithReminderTimes(t, household.Owner, at(17, 9, 0), []int64{household.Member.User.ID}, nil)

	want := []store.ReminderTime{{Minutes: -24 * 60}, {Minutes: -60}, {Minutes: 0}}
	if fmt.Sprint(appt.ReminderTimes) != fmt.Sprint(want) {
		t.Errorf("Reminder Times = %v, want %v", appt.ReminderTimes, want)
	}
	sendRemindersAt(t, h, at(17, 9, 0))
	expectAppointmentReminders(t, household.Member, stages(appt.ID, "due_soon", "due_soon", "due_now")...)
}

func TestAppointmentWithNoReminderTimesSendsNoReminders(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	createAppointmentWithReminderTimes(t, household.Owner, at(17, 9, 0), []int64{household.Member.User.ID}, []map[string]any{})

	sendRemindersAt(t, h, at(17, 9, 0))
	expectAppointmentReminders(t, household.Member)
}

func TestAppointmentReminderTimeAfterTheStartIsRejected(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	after := []map[string]any{{"minutes": 30}}

	household.Owner.Post("/api/v1/events", appointmentBody(at(17, 9, 0), nil, after)).Expect(http.StatusBadRequest)

	appt := createAppointmentAt(t, household.Owner, at(17, 9, 0))
	household.Owner.Patch(fmt.Sprintf("/api/v1/events/%d", appt.ID), appointmentBody(at(17, 9, 0), nil, after)).Expect(http.StatusBadRequest)
}

func TestEditingAppointmentReminderTimesReschedulesItsReminders(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	appt := createAppointmentAt(t, household.Owner, at(17, 9, 0), household.Member.User.ID)

	sendRemindersAt(t, h, at(16, 9, 0))
	household.Owner.Patch(fmt.Sprintf("/api/v1/events/%d", appt.ID),
		appointmentBody(at(17, 9, 0), nil, []map[string]any{minutesBefore(3 * 60)})).Expect(http.StatusOK)

	sendRemindersAt(t, h, at(17, 9, 0))
	got := reminderTitlesOf(t, household.Member, "appointment_reminder")
	if want := []string{"Appointment in 24 hours", "Appointment in 3 hours"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("Reminder titles = %q, want %q", got, want)
	}
}

func TestReschedulingAppointmentMovesRemindersAtItsReminderTimes(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	appt := createAppointmentWithReminderTimes(t, household.Owner, at(17, 9, 0), []int64{household.Member.User.ID},
		[]map[string]any{minutesBefore(2 * 60)})

	// Leaving Reminder Times out of the update keeps them.
	editAppointment(t, household.Owner, appt.ID, at(18, 9, 0), nil)

	sendRemindersAt(t, h, at(17, 9, 0))
	expectAppointmentReminders(t, household.Member)
	sendRemindersAt(t, h, at(18, 7, 0))
	got := reminderTitlesOf(t, household.Member, "appointment_reminder")
	if want := []string{"Appointment in 2 hours"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("Reminder titles = %q, want %q", got, want)
	}
	read := apitest.Decode[store.Event](t, household.Member.Get(fmt.Sprintf("/api/v1/events/%d", appt.ID)).Expect(http.StatusOK))
	if want := []store.ReminderTime{{Minutes: -120}}; fmt.Sprint(read.ReminderTimes) != fmt.Sprint(want) {
		t.Errorf("Reminder Times = %v, want %v", read.ReminderTimes, want)
	}
}

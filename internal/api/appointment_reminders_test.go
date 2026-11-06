package api_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"homebase/internal/apitest"
	"homebase/internal/store"
)

// appointmentRemindersOf returns the Appointment Reminders a Member has
// received, oldest first, as "stage appointment-id" strings.
func appointmentRemindersOf(t *testing.T, c *apitest.Client) []string {
	t.Helper()
	got := []string{}
	list := notificationsOf(t, c)
	for i := len(list.Notifications) - 1; i >= 0; i-- {
		n := list.Notifications[i]
		if n.Kind == "appointment_reminder" {
			got = append(got, fmt.Sprintf("%s %d", n.Stage, n.SubjectID))
		}
	}
	return got
}

func expectAppointmentReminders(t *testing.T, c *apitest.Client, want ...string) {
	t.Helper()
	got := appointmentRemindersOf(t, c)
	if len(want) == 0 {
		want = []string{}
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("%s Appointment Reminders = %v, want %v", c.User.Name, got, want)
	}
}

func createAppointmentAt(t *testing.T, c *apitest.Client, starts time.Time, attendees ...int64) store.Event {
	t.Helper()
	body := map[string]any{
		"title":     "Dentist",
		"starts_at": starts.Format(time.RFC3339),
		"ends_at":   starts.Add(time.Hour).Format(time.RFC3339),
	}
	if attendees != nil {
		body["attendee_ids"] = attendees
	}
	return apitest.Decode[store.Event](t, c.Post("/api/v1/events", body).Expect(http.StatusCreated))
}

// stages names an Appointment's Reminders the way appointmentRemindersOf does.
func stages(id int64, names ...string) []string {
	out := []string{}
	for _, name := range names {
		out = append(out, fmt.Sprintf("%s %d", name, id))
	}
	return out
}

func TestAppointmentGetsTwoDueSoonRemindersAndADueNow(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	appt := createAppointmentAt(t, household.Owner, at(17, 9, 0), household.Member.User.ID)

	sendRemindersAt(t, h, at(16, 8, 59))
	expectAppointmentReminders(t, household.Member)

	sendRemindersAt(t, h, at(16, 9, 0))
	expectAppointmentReminders(t, household.Member, stages(appt.ID, "due_soon")...)

	sendRemindersAt(t, h, at(17, 7, 59))
	expectAppointmentReminders(t, household.Member, stages(appt.ID, "due_soon")...)

	sendRemindersAt(t, h, at(17, 8, 0))
	expectAppointmentReminders(t, household.Member, stages(appt.ID, "due_soon", "due_soon")...)

	sendRemindersAt(t, h, at(17, 9, 0))
	expectAppointmentReminders(t, household.Member, stages(appt.ID, "due_soon", "due_soon", "due_now")...)

	sendRemindersAt(t, h, at(20, 9, 0))
	expectAppointmentReminders(t, household.Member, stages(appt.ID, "due_soon", "due_soon", "due_now")...)
	expectAppointmentReminders(t, household.Owner)
}

func TestAppointmentWithoutAttendeesRemindsEveryMember(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	appt := createAppointmentAt(t, household.Owner, at(17, 9, 0))

	sendRemindersAt(t, h, at(17, 9, 0))
	expectAppointmentReminders(t, household.Owner, stages(appt.ID, "due_soon", "due_soon", "due_now")...)
	expectAppointmentReminders(t, household.Member, stages(appt.ID, "due_soon", "due_soon", "due_now")...)
}

func TestDeletingAppointmentCancelsItsPendingReminders(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	appt := createAppointmentAt(t, household.Owner, at(17, 9, 0), household.Member.User.ID)

	sendRemindersAt(t, h, at(16, 9, 0))
	household.Owner.Delete(fmt.Sprintf("/api/v1/events/%d", appt.ID)).Expect(http.StatusOK)

	sendRemindersAt(t, h, at(18, 9, 0))
	expectAppointmentReminders(t, household.Member, stages(appt.ID, "due_soon")...)
}

func TestChangingAppointmentTimeReschedulesReminders(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	member := household.Member.User.ID
	appt := createAppointmentAt(t, household.Owner, at(17, 9, 0), member)

	sendRemindersAt(t, h, at(16, 9, 0))
	editAppointment(t, household.Owner, appt.ID, at(18, 15, 0), nil)

	sendRemindersAt(t, h, at(17, 9, 0))
	expectAppointmentReminders(t, household.Member, stages(appt.ID, "due_soon")...)

	sendRemindersAt(t, h, at(18, 15, 0))
	expectAppointmentReminders(t, household.Member, stages(appt.ID, "due_soon", "due_soon", "due_soon", "due_now")...)
}

func TestRemovingAttendeeCancelsTheirPendingReminders(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	owner, member := household.Owner.User.ID, household.Member.User.ID
	appt := createAppointmentAt(t, household.Owner, at(17, 9, 0), owner, member)

	sendRemindersAt(t, h, at(16, 9, 0))
	editAppointment(t, household.Owner, appt.ID, at(17, 9, 0), []int64{owner})

	sendRemindersAt(t, h, at(17, 9, 0))
	expectAppointmentReminders(t, household.Owner, stages(appt.ID, "due_soon", "due_soon", "due_now")...)
	expectAppointmentReminders(t, household.Member, stages(appt.ID, "due_soon")...)
}

func TestAddingAttendeeSchedulesOnlyRemindersStillAhead(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	owner, member := household.Owner.User.ID, household.Member.User.ID
	appt := createAppointmentAt(t, household.Owner, at(17, 9, 0), owner)

	sendRemindersAt(t, h, at(16, 9, 0))
	h.Clock.Set(at(16, 12, 0))
	editAppointment(t, household.Owner, appt.ID, at(17, 9, 0), []int64{owner, member})

	sendRemindersAt(t, h, at(17, 9, 0))
	expectAppointmentReminders(t, household.Owner, stages(appt.ID, "due_soon", "due_soon", "due_now")...)
	expectAppointmentReminders(t, household.Member, stages(appt.ID, "due_soon", "due_now")...)
}

func TestAppointmentRemindersWhoseTimeHasPassedAreSkipped(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	// Now is the 15th at 12:00: Due Soon (15th 11:30) has passed, the
	// hour-before Reminder (16th 10:30) has not.
	appt := createAppointmentAt(t, household.Owner, at(16, 11, 30), household.Member.User.ID)

	sendRemindersAt(t, h, at(16, 11, 30))
	expectAppointmentReminders(t, household.Member, stages(appt.ID, "due_soon", "due_now")...)
}

func TestAppointmentReminderDescribesTheAppointment(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	appt := createAppointmentAt(t, household.Owner, at(17, 9, 0), household.Member.User.ID)

	sendRemindersAt(t, h, at(17, 8, 0))
	list := notificationsOf(t, household.Member)
	n := list.Notifications[0]
	if n.Kind != "appointment_reminder" || n.Stage != "due_soon" || n.SubjectType != "appointment" || n.SubjectID != appt.ID {
		t.Fatalf("latest Notification = %+v, want the hour-before Due Soon Reminder for Appointment %d", n, appt.ID)
	}
	if n.Title != "Appointment in 1 hour" || n.Body != "Dentist" {
		t.Errorf("title, body = %q, %q; want %q, %q", n.Title, n.Body, "Appointment in 1 hour", "Dentist")
	}
}

func TestAppointmentReminderTitlesSayHowSoonItStarts(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	createAppointmentAt(t, household.Owner, at(17, 9, 0), household.Member.User.ID)

	sendRemindersAt(t, h, at(20, 9, 0))
	got := reminderTitlesOf(t, household.Member, "appointment_reminder")
	want := []string{"Appointment in 24 hours", "Appointment in 1 hour", "Appointment starting now"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("Reminder titles = %q, want %q", got, want)
	}
}

// editAppointment moves an Appointment to starts and, unless attendees is
// nil, replaces its Attendees.
func editAppointment(t *testing.T, c *apitest.Client, id int64, starts time.Time, attendees []int64) {
	t.Helper()
	body := map[string]any{
		"title":     "Dentist",
		"starts_at": starts.Format(time.RFC3339),
		"ends_at":   starts.Add(time.Hour).Format(time.RFC3339),
	}
	if attendees != nil {
		body["attendee_ids"] = attendees
	}
	c.Patch(fmt.Sprintf("/api/v1/events/%d", id), body).Expect(http.StatusOK)
}

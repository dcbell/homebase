package api_test

import (
	"fmt"
	"net/http"
	"testing"

	"homebase/internal/apitest"
	"homebase/internal/store"
)

// setPersonalAppointmentReminderTimes sets the caller's Personal Reminder
// Times on an Appointment, as the API's JSON takes them.
func setPersonalAppointmentReminderTimes(c *apitest.Client, appointmentID int64, reminderTimes []map[string]any) *apitest.Response {
	return c.Do(http.MethodPut, fmt.Sprintf("/api/v1/events/%d/personal-reminder-times", appointmentID), map[string]any{
		"reminder_times": reminderTimes,
	})
}

// useSharedAppointmentReminderTimes removes the caller's Personal Reminder
// Times on an Appointment, so its shared Reminder Times apply to them again.
func useSharedAppointmentReminderTimes(c *apitest.Client, appointmentID int64) *apitest.Response {
	return c.Delete(fmt.Sprintf("/api/v1/events/%d/personal-reminder-times", appointmentID))
}

// expectUsingSharedOnAppointment checks that the Appointment, as c reads it,
// shows c is using its shared Reminder Times.
func expectUsingSharedOnAppointment(t *testing.T, c *apitest.Client, appointmentID int64) {
	t.Helper()
	got := getAppointment(t, c, appointmentID).PersonalReminderTimes
	if got == nil || !got.UsingShared || len(got.ReminderTimes) != 0 {
		t.Errorf("%s Personal Reminder Times = %+v, want using shared", c.User.Name, got)
	}
}

// expectAppointmentReminderTitles checks the titles of the Appointment
// Reminders a Member has received, oldest first.
func expectAppointmentReminderTitles(t *testing.T, c *apitest.Client, want ...string) {
	t.Helper()
	got := reminderTitlesOf(t, c, "appointment_reminder")
	if len(want) == 0 {
		want = []string{}
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("%s Appointment Reminder titles = %q, want %q", c.User.Name, got, want)
	}
}

func TestTwoAttendeesWithDifferentPersonalReminderTimesEachGetTheirOwn(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	owner, member := household.Owner.User.ID, household.Member.User.ID
	appt := createAppointmentWithReminderTimes(t, household.Owner, at(17, 9, 0), []int64{owner, member},
		[]map[string]any{minutesBefore(2 * 60)})

	saved := apitest.Decode[store.Event](t, setPersonalAppointmentReminderTimes(household.Owner, appt.ID,
		[]map[string]any{minutesBefore(30)}).Expect(http.StatusOK))
	want := store.PersonalReminderTimes{ReminderTimes: []store.ReminderTime{{Minutes: -30}}}
	if saved.PersonalReminderTimes == nil || fmt.Sprint(*saved.PersonalReminderTimes) != fmt.Sprint(want) {
		t.Errorf("Personal Reminder Times after setting = %+v, want %+v", saved.PersonalReminderTimes, want)
	}
	setPersonalAppointmentReminderTimes(household.Member, appt.ID,
		[]map[string]any{minutesBefore(3 * 60), minutesBefore(15)}).Expect(http.StatusOK)

	// Each reads back only their own.
	if got := getAppointment(t, household.Owner, appt.ID).PersonalReminderTimes; got == nil || fmt.Sprint(*got) != fmt.Sprint(want) {
		t.Errorf("Owner Personal Reminder Times read back = %+v, want %+v", got, want)
	}
	memberWant := store.PersonalReminderTimes{ReminderTimes: []store.ReminderTime{{Minutes: -180}, {Minutes: -15}}}
	if got := getAppointment(t, household.Member, appt.ID).PersonalReminderTimes; got == nil || fmt.Sprint(*got) != fmt.Sprint(memberWant) {
		t.Errorf("Member Personal Reminder Times read back = %+v, want %+v", got, memberWant)
	}

	// Later changes to the shared Reminder Times, or saving the Appointment
	// for the same Attendees, leave them alone.
	household.Owner.Patch(fmt.Sprintf("/api/v1/events/%d", appt.ID),
		appointmentBody(at(17, 9, 0), []int64{member, owner}, []map[string]any{minutesBefore(60)})).Expect(http.StatusOK)

	sendRemindersAt(t, h, at(17, 9, 0))
	expectAppointmentReminderTitles(t, household.Owner, "Appointment in 30 minutes")
	expectAppointmentReminderTitles(t, household.Member, "Appointment in 3 hours", "Appointment in 15 minutes")
}

func TestAMemberCanPersonaliseAWholeHouseholdAppointmentAndOthersKeepTheShared(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	appt := createAppointmentWithReminderTimes(t, household.Owner, at(17, 9, 0), nil,
		[]map[string]any{minutesBefore(2 * 60)})
	expectUsingSharedOnAppointment(t, household.Member, appt.ID)

	setPersonalAppointmentReminderTimes(household.Member, appt.ID, []map[string]any{minutesBefore(30)}).Expect(http.StatusOK)
	expectUsingSharedOnAppointment(t, household.Owner, appt.ID)

	sendRemindersAt(t, h, at(17, 9, 0))
	expectAppointmentReminderTitles(t, household.Owner, "Appointment in 2 hours")
	expectAppointmentReminderTitles(t, household.Member, "Appointment in 30 minutes")
}

func TestAnEmptyPersonalListMeansNoAppointmentRemindersForThatMember(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	appt := createAppointmentAt(t, household.Owner, at(17, 9, 0))

	saved := apitest.Decode[store.Event](t, setPersonalAppointmentReminderTimes(household.Member, appt.ID, []map[string]any{}).Expect(http.StatusOK))
	if got := saved.PersonalReminderTimes; got == nil || got.UsingShared || len(got.ReminderTimes) != 0 {
		t.Errorf("Personal Reminder Times = %+v, want an empty personal list", got)
	}

	sendRemindersAt(t, h, at(17, 9, 0))
	expectAppointmentReminders(t, household.Member)
	expectAppointmentReminders(t, household.Owner, stages(appt.ID, "due_soon", "due_soon", "due_now")...)
}

func TestRemovingPersonalAppointmentReminderTimesGoesBackToTheSharedOnes(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	appt := createAppointmentWithReminderTimes(t, household.Owner, at(17, 9, 0), []int64{household.Member.User.ID},
		[]map[string]any{minutesBefore(2 * 60)})

	setPersonalAppointmentReminderTimes(household.Member, appt.ID, []map[string]any{}).Expect(http.StatusOK)
	saved := apitest.Decode[store.Event](t, useSharedAppointmentReminderTimes(household.Member, appt.ID).Expect(http.StatusOK))
	if got := saved.PersonalReminderTimes; got == nil || !got.UsingShared {
		t.Errorf("Personal Reminder Times after removing = %+v, want using shared", got)
	}

	sendRemindersAt(t, h, at(17, 9, 0))
	expectAppointmentReminderTitles(t, household.Member, "Appointment in 2 hours")
}

func TestAMemberWhoIsNotAnAttendeeCannotSetPersonalAppointmentReminderTimes(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	appt := createAppointmentWithReminderTimes(t, household.Owner, at(17, 9, 0), []int64{household.Owner.User.ID},
		[]map[string]any{minutesBefore(2 * 60)})
	setPersonalAppointmentReminderTimes(household.Owner, appt.ID, []map[string]any{minutesBefore(30)}).Expect(http.StatusOK)

	if got := getAppointment(t, household.Member, appt.ID).PersonalReminderTimes; got != nil {
		t.Errorf("a Member who isn't an Attendee read Personal Reminder Times %+v, want none", got)
	}
	setPersonalAppointmentReminderTimes(household.Member, appt.ID, []map[string]any{minutesBefore(15)}).Expect(http.StatusForbidden)
	useSharedAppointmentReminderTimes(household.Member, appt.ID).Expect(http.StatusForbidden)
	setPersonalAppointmentReminderTimes(household.Member, 999999, []map[string]any{}).Expect(http.StatusNotFound)

	sendRemindersAt(t, h, at(17, 9, 0))
	expectAppointmentReminderTitles(t, household.Owner, "Appointment in 30 minutes")
	expectAppointmentReminders(t, household.Member)
}

func TestInvalidPersonalAppointmentReminderTimesAreRejected(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	appt := createAppointmentAt(t, household.Owner, at(17, 9, 0))

	setPersonalAppointmentReminderTimes(household.Member, appt.ID, []map[string]any{{"minutes": 30}}).Expect(http.StatusBadRequest)
	setPersonalAppointmentReminderTimes(household.Member, appt.ID, []map[string]any{{"days": -1, "at": "08:00"}}).Expect(http.StatusBadRequest)
	setPersonalAppointmentReminderTimes(household.Member, appt.ID, []map[string]any{minutesBefore(60), minutesBefore(60)}).Expect(http.StatusBadRequest)
	household.Member.Do(http.MethodPut, fmt.Sprintf("/api/v1/events/%d/personal-reminder-times", appt.ID), map[string]any{}).Expect(http.StatusBadRequest)
	expectUsingSharedOnAppointment(t, household.Member, appt.ID)
}

func TestRemovingAnAttendeeThrowsAwayTheirPersonalReminderTimes(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	owner, member := household.Owner.User.ID, household.Member.User.ID
	appt := createAppointmentWithReminderTimes(t, household.Owner, at(17, 9, 0), []int64{owner, member},
		[]map[string]any{minutesBefore(30)})
	setPersonalAppointmentReminderTimes(household.Member, appt.ID, []map[string]any{minutesBefore(2 * 60)}).Expect(http.StatusOK)

	editAppointment(t, household.Owner, appt.ID, at(17, 9, 0), []int64{owner})
	sendRemindersAt(t, h, at(17, 7, 30))
	expectAppointmentReminders(t, household.Member)

	// Added back, they start from the shared Reminder Times.
	editAppointment(t, household.Owner, appt.ID, at(17, 9, 0), []int64{owner, member})
	expectUsingSharedOnAppointment(t, household.Member, appt.ID)
	sendRemindersAt(t, h, at(17, 9, 0))
	expectAppointmentReminderTitles(t, household.Member, "Appointment in 30 minutes")
}

func TestAWholeHouseholdAppointmentGainingAttendeesDropsPersonalReminderTimesOfOthers(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	owner := household.Owner.User.ID
	appt := createAppointmentWithReminderTimes(t, household.Owner, at(17, 9, 0), nil,
		[]map[string]any{minutesBefore(30)})
	setPersonalAppointmentReminderTimes(household.Owner, appt.ID, []map[string]any{minutesBefore(45)}).Expect(http.StatusOK)
	setPersonalAppointmentReminderTimes(household.Member, appt.ID, []map[string]any{minutesBefore(2 * 60)}).Expect(http.StatusOK)

	editAppointment(t, household.Owner, appt.ID, at(17, 9, 0), []int64{owner})
	sendRemindersAt(t, h, at(17, 7, 30))
	expectAppointmentReminders(t, household.Member)

	// For the whole Household again, the Member is back on the shared ones;
	// the Owner, an Attendee throughout, keeps their own.
	editAppointment(t, household.Owner, appt.ID, at(17, 9, 0), []int64{})
	expectUsingSharedOnAppointment(t, household.Member, appt.ID)
	sendRemindersAt(t, h, at(17, 9, 0))
	expectAppointmentReminderTitles(t, household.Owner, "Appointment in 45 minutes")
	expectAppointmentReminderTitles(t, household.Member, "Appointment in 30 minutes")
}

func TestAMemberWhoLeavesTheHouseholdLosesTheirPersonalAppointmentReminderTimes(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	appt := createAppointmentWithReminderTimes(t, household.Owner, at(17, 9, 0), nil,
		[]map[string]any{minutesBefore(2 * 60)})
	setPersonalAppointmentReminderTimes(household.Member, appt.ID, []map[string]any{}).Expect(http.StatusOK)

	household.Owner.Delete(fmt.Sprintf("/api/v1/members/%d", household.Member.User.ID)).Expect(http.StatusOK)
	member := readd(t, h, household, household.Member.User)

	expectUsingSharedOnAppointment(t, member, appt.ID)
	sendRemindersAt(t, h, at(17, 9, 0))
	expectAppointmentReminderTitles(t, member, "Appointment in 2 hours")
}

package api_test

import (
	"net/http"
	"testing"

	"homebase/internal/apitest"
	"homebase/internal/store"
)

func addMember(t *testing.T, h *apitest.Harness, household apitest.Household, email, name string) *apitest.Client {
	t.Helper()
	user := apitest.Decode[store.User](t, household.Owner.Post("/api/v1/members", map[string]any{
		"email": email,
		"name":  name,
		"role":  "member",
	}).Expect(http.StatusCreated))
	return h.SignIn(user, household.ID)
}

func TestMemberAddedLaterGetsRemindersForWholeHouseholdAppointments(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	appt := createAppointmentAt(t, household.Owner, at(17, 9, 0))
	createAppointmentAt(t, household.Owner, at(17, 10, 0), household.Owner.User.ID)

	h.Clock.Set(at(16, 12, 0))
	newcomer := addMember(t, h, household, "newcomer@example.test", "Newcomer")

	sendRemindersAt(t, h, at(17, 10, 0))
	// Due Soon (16th 09:00) had passed when they joined, so it is skipped;
	// the Appointment with Attendees is not theirs.
	expectAppointmentReminders(t, newcomer, stages(appt.ID, "hour_before", "due_now")...)
	expectAppointmentReminders(t, household.Member, stages(appt.ID, "due_soon", "hour_before", "due_now")...)
}

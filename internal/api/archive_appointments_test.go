package api_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"homebase/internal/apitest"
	"homebase/internal/store"
)

func TestDeletingAppointmentArchivesIt(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	appt := createAppointmentAt(t, household.Owner, at(17, 9, 0))

	household.Owner.Delete(fmt.Sprintf("/api/v1/events/%d", appt.ID)).Expect(http.StatusOK)

	got := apitest.Decode[store.Event](t, household.Owner.Get(fmt.Sprintf("/api/v1/events/%d", appt.ID)).Expect(http.StatusOK))
	if got.Status != "archived" || got.Title != "Dentist" {
		t.Errorf("status, title = %q, %q; want archived, Dentist", got.Status, got.Title)
	}
}

// keptAndArchivedAppointments creates two Appointments in January 2030 and
// archives the second, returning both.
func keptAndArchivedAppointments(t *testing.T, c *apitest.Client) (kept, archived store.Event) {
	t.Helper()
	kept = createAppointmentAt(t, c, at(17, 9, 0))
	archived = createAppointmentAt(t, c, at(18, 9, 0))
	c.Delete(fmt.Sprintf("/api/v1/events/%d", archived.ID)).Expect(http.StatusOK)
	return kept, archived
}

func TestArchivingAppointmentCancelsItsPendingReminders(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	kept, _ := keptAndArchivedAppointments(t, household.Owner)

	sendRemindersAt(t, h, at(20, 9, 0))
	expectAppointmentReminders(t, household.Member, stages(kept.ID, "due_soon", "hour_before", "due_now")...)
}

func TestMemberAddedLaterGetsNoRemindersForArchivedAppointments(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	kept, _ := keptAndArchivedAppointments(t, household.Owner)

	h.Clock.Set(at(15, 12, 0))
	newcomer := addMember(t, h, household, "newcomer@example.test", "Newcomer")

	sendRemindersAt(t, h, at(20, 9, 0))
	expectAppointmentReminders(t, newcomer, stages(kept.ID, "due_soon", "hour_before", "due_now")...)
}

func TestEditingArchivedAppointmentSchedulesNoReminders(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	_, archived := keptAndArchivedAppointments(t, household.Owner)

	editAppointment(t, household.Owner, archived.ID, at(19, 9, 0), []int64{household.Member.User.ID})

	sendRemindersAt(t, h, at(20, 9, 0))
	for _, c := range []*apitest.Client{household.Owner, household.Member} {
		for _, r := range appointmentRemindersOf(t, c) {
			if strings.HasSuffix(r, fmt.Sprintf(" %d", archived.ID)) {
				t.Errorf("%s got Reminder %q for the archived Appointment", c.User.Name, r)
			}
		}
	}
}

func eventIDs(events []store.Event) []int64 {
	ids := []int64{}
	for _, e := range events {
		ids = append(ids, e.ID)
	}
	return ids
}

func TestArchivedAppointmentsAreLeftOffTheAppointmentList(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	kept, _ := keptAndArchivedAppointments(t, household.Owner)

	list := apitest.Decode[[]store.Event](t, household.Owner.Get("/api/v1/events").Expect(http.StatusOK))
	if got := eventIDs(list); fmt.Sprint(got) != fmt.Sprint([]int64{kept.ID}) {
		t.Errorf("Appointment list = %v, want [%d]", got, kept.ID)
	}
}

func TestArchivedAppointmentsAreLeftOffTheDashboard(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	kept, _ := keptAndArchivedAppointments(t, household.Owner)

	dashboard := apitest.Decode[store.Dashboard](t, household.Owner.Get("/api/v1/dashboard").Expect(http.StatusOK))
	if got := eventIDs(dashboard.Events); fmt.Sprint(got) != fmt.Sprint([]int64{kept.ID}) {
		t.Errorf("dashboard Appointments = %v, want [%d]", got, kept.ID)
	}
}

func TestArchivedAppointmentsAreLeftOffTheCalendar(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	kept, _ := keptAndArchivedAppointments(t, household.Owner)

	calendar := apitest.Decode[store.CalendarMonth](t, household.Owner.Get("/api/v1/calendar?month=2030-01").Expect(http.StatusOK))
	got := []int64{}
	for _, day := range calendar.Days {
		for _, entry := range day.Entries {
			if entry.Type == "appointment" {
				got = append(got, entry.ID)
			}
		}
	}
	if fmt.Sprint(got) != fmt.Sprint([]int64{kept.ID}) {
		t.Errorf("calendar Appointments = %v, want [%d]", got, kept.ID)
	}
}

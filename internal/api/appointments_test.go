package api_test

import (
	"fmt"
	"net/http"
	"testing"

	"homebase/internal/apitest"
	"homebase/internal/store"
)

func appointment(attendees ...int64) map[string]any {
	body := map[string]any{
		"title":     "Dentist",
		"starts_at": "2030-01-20T09:00:00Z",
		"ends_at":   "2030-01-20T10:00:00Z",
		"location":  "Main St",
	}
	if attendees != nil {
		body["attendee_ids"] = attendees
	}
	return body
}

func TestAppointmentAttendeesAreSavedAndReturned(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()

	created := apitest.Decode[store.Event](t, household.Owner.Post("/api/v1/events",
		appointment(household.Member.User.ID)).Expect(http.StatusCreated))

	got := apitest.Decode[store.Event](t, household.Owner.Get(fmt.Sprintf("/api/v1/events/%d", created.ID)).Expect(http.StatusOK))
	if len(got.Attendees) != 1 || got.Attendees[0].ID != household.Member.User.ID || got.Attendees[0].Name != household.Member.User.Name {
		t.Errorf("attendees = %+v, want only %s", got.Attendees, household.Member.User.Name)
	}
	if len(got.AttendeeIDs) != 1 || got.AttendeeIDs[0] != household.Member.User.ID {
		t.Errorf("attendee_ids = %v, want [%d]", got.AttendeeIDs, household.Member.User.ID)
	}
}

func TestOnlyHouseholdMembersCanBeAttendees(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	ours := h.SeedHousehold()
	theirs := h.SeedHousehold()

	ours.Owner.Post("/api/v1/events", appointment(theirs.Member.User.ID)).Expect(http.StatusBadRequest)

	created := apitest.Decode[store.Event](t, ours.Owner.Post("/api/v1/events", appointment()).Expect(http.StatusCreated))
	ours.Owner.Patch(fmt.Sprintf("/api/v1/events/%d", created.ID), appointment(ours.Member.User.ID, theirs.Owner.User.ID)).Expect(http.StatusBadRequest)

	got := apitest.Decode[store.Event](t, ours.Owner.Get(fmt.Sprintf("/api/v1/events/%d", created.ID)).Expect(http.StatusOK))
	if len(got.AttendeeIDs) != 0 {
		t.Errorf("attendee_ids = %v after rejected update, want none", got.AttendeeIDs)
	}
}

func TestAppointmentWithoutAttendeesIsForTheWholeHousehold(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()

	created := apitest.Decode[store.Event](t, household.Owner.Post("/api/v1/events", appointment()).Expect(http.StatusCreated))

	raw := apitest.Decode[map[string]any](t, household.Owner.Get(fmt.Sprintf("/api/v1/events/%d", created.ID)).Expect(http.StatusOK))
	if ids, ok := raw["attendee_ids"].([]any); !ok || len(ids) != 0 {
		t.Errorf("attendee_ids = %#v, want an empty list", raw["attendee_ids"])
	}
}

func TestEditingAppointmentReplacesOrKeepsAttendees(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	owner, member := household.Owner.User.ID, household.Member.User.ID
	created := apitest.Decode[store.Event](t, household.Owner.Post("/api/v1/events", appointment(member)).Expect(http.StatusCreated))
	path := fmt.Sprintf("/api/v1/events/%d", created.ID)

	replaced := apitest.Decode[store.Event](t, household.Owner.Patch(path, appointment(owner)).Expect(http.StatusOK))
	if len(replaced.AttendeeIDs) != 1 || replaced.AttendeeIDs[0] != owner {
		t.Errorf("after replacing, attendee_ids = %v, want [%d]", replaced.AttendeeIDs, owner)
	}

	kept := apitest.Decode[store.Event](t, household.Owner.Patch(path, appointment()).Expect(http.StatusOK))
	if len(kept.AttendeeIDs) != 1 || kept.AttendeeIDs[0] != owner {
		t.Errorf("after omitting attendee_ids, attendee_ids = %v, want [%d] kept", kept.AttendeeIDs, owner)
	}

	cleared := apitest.Decode[store.Event](t, household.Owner.Patch(path, appointment([]int64{}...)).Expect(http.StatusOK))
	if len(cleared.AttendeeIDs) != 0 {
		t.Errorf("after clearing, attendee_ids = %v, want none", cleared.AttendeeIDs)
	}
}

func TestAddingAnotherMemberAsAttendeeNotifiesThem(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()

	created := apitest.Decode[store.Event](t, household.Owner.Post("/api/v1/events",
		appointment(household.Owner.User.ID, household.Member.User.ID)).Expect(http.StatusCreated))

	got := notificationsOf(t, household.Member)
	if len(got.Notifications) != 1 {
		t.Fatalf("Member got %d Notifications, want 1: %+v", len(got.Notifications), got.Notifications)
	}
	n := got.Notifications[0]
	if n.Kind != "assignment" || n.SubjectType != "appointment" || n.SubjectID != created.ID {
		t.Errorf("notification = %s %s %d, want assignment appointment %d", n.Kind, n.SubjectType, n.SubjectID, created.ID)
	}
	if want := fmt.Sprintf("/events/%d", created.ID); n.URL != want {
		t.Errorf("url = %q, want %q", n.URL, want)
	}
	if want := "Owner 1 assigned you an Appointment"; n.Title != want {
		t.Errorf("title = %q, want %q", n.Title, want)
	}
	if n.Body != "Dentist" {
		t.Errorf("body = %q, want the Appointment title", n.Body)
	}

	if owner := notificationsOf(t, household.Owner); len(owner.Notifications) != 0 {
		t.Errorf("Owner added themselves and got %+v, want no Notifications", owner.Notifications)
	}
}

func TestEditingAppointmentNotifiesOnlyNewlyAddedAttendees(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	created := apitest.Decode[store.Event](t, household.Member.Post("/api/v1/events", appointment()).Expect(http.StatusCreated))
	path := fmt.Sprintf("/api/v1/events/%d", created.ID)

	household.Member.Patch(path, appointment(household.Owner.User.ID)).Expect(http.StatusOK)
	household.Member.Patch(path, appointment(household.Owner.User.ID)).Expect(http.StatusOK)
	household.Member.Patch(path, appointment()).Expect(http.StatusOK)

	if got := notificationsOf(t, household.Owner); len(got.Notifications) != 1 {
		t.Errorf("Owner got %d Notifications, want 1 for being added once: %+v", len(got.Notifications), got.Notifications)
	}
}

func TestCalendarEntriesShowAttendees(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	household.Owner.Post("/api/v1/events", appointment(household.Member.User.ID)).Expect(http.StatusCreated)

	calendar := apitest.Decode[store.CalendarMonth](t, household.Owner.Get("/api/v1/calendar?month=2030-01").Expect(http.StatusOK))
	for _, day := range calendar.Days {
		for _, entry := range day.Entries {
			if entry.Type != "appointment" {
				continue
			}
			if len(entry.Attendees) != 1 || entry.Attendees[0].Name != household.Member.User.Name {
				t.Errorf("attendees = %+v, want %s", entry.Attendees, household.Member.User.Name)
			}
			return
		}
	}
	t.Fatal("no appointment entry on the calendar")
}

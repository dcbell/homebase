package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"homebase/internal/config"
	"homebase/internal/store"
)

// fakeAPI answers the API calls the Appointment pages make and records what
// the web service sends.
type fakeAPI struct {
	event    store.Event
	payloads map[string]map[string]any
}

func (f *fakeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.payloads[r.Method+" "+r.URL.Path] = body
		_ = json.NewEncoder(w).Encode(f.event)
		return
	}
	switch r.URL.Path {
	case "/api/v1/dashboard":
		_ = json.NewEncoder(w).Encode(store.Dashboard{Members: []store.User{{ID: 1, Name: "Owner"}, {ID: 2, Name: "Member"}}})
	default:
		_ = json.NewEncoder(w).Encode(f.event)
	}
}

func newAppointmentWebServer(t *testing.T, event store.Event) (http.Handler, *fakeAPI) {
	t.Helper()
	api := &fakeAPI{event: event, payloads: map[string]map[string]any{}}
	upstream := httptest.NewServer(api)
	t.Cleanup(upstream.Close)
	s := newWebServer(config.Config{APIInternalURL: upstream.URL, SessionCookieName: "homebase_session"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return s.routes(), api
}

func postForm(h http.Handler, path string, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestAppointmentFormsSendCheckedAttendees(t *testing.T) {
	h, api := newAppointmentWebServer(t, store.Event{ID: 5})
	form := url.Values{"title": {"Dentist"}, "starts_at": {"2030-01-20T09:00"}, "ends_at": {"2030-01-20T10:00"}, "attendee_ids": {"2"}}

	postForm(h, "/events", form)
	if got := api.payloads["POST /api/v1/events"]["attendee_ids"]; !jsonIDsEqual(got, 2) {
		t.Errorf("create sent attendee_ids %#v, want [2]", got)
	}

	form.Del("attendee_ids")
	postForm(h, "/events/5", form)
	if got := api.payloads["PATCH /api/v1/events/5"]["attendee_ids"]; !jsonIDsEqual(got) {
		t.Errorf("edit with none checked sent attendee_ids %#v, want [] to make it whole-Household", got)
	}
}

func TestAppointmentPageShowsAttendees(t *testing.T) {
	h, _ := newAppointmentWebServer(t, store.Event{ID: 5, Title: "Dentist", AttendeeIDs: []int64{2}, Attendees: []store.Attendee{{ID: 2, Name: "Member"}}})

	body := get(t, h, "/events/5").Body.String()
	if !strings.Contains(body, "For Member") {
		t.Errorf("page does not say who the Appointment is for")
	}
	if !strings.Contains(body, `name="attendee_ids" value="2" checked`) {
		t.Errorf("edit form does not check the current Attendee")
	}
	if strings.Contains(body, `name="attendee_ids" value="1" checked`) {
		t.Errorf("edit form checks a Member who is not an Attendee")
	}
}

func TestAddAppointmentFormOffersEveryMemberAsAttendee(t *testing.T) {
	h, _ := newAppointmentWebServer(t, store.Event{})

	body := get(t, h, "/").Body.String()
	for _, want := range []string{`name="attendee_ids" value="1">Owner`, `name="attendee_ids" value="2">Member`} {
		if !strings.Contains(body, want) {
			t.Errorf("dashboard is missing unchecked Attendee option %s", want)
		}
	}
}

func jsonIDsEqual(got any, want ...float64) bool {
	list, ok := got.([]any)
	if !ok || len(list) != len(want) {
		return false
	}
	for i := range want {
		if list[i] != want[i] {
			return false
		}
	}
	return true
}

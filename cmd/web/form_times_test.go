package main

import (
	"net/url"
	"testing"
	"time"

	"homebase/internal/store"
)

// inTimezone runs the test with APP_TIMEZONE set to loc, as cfg.ApplyTimezone
// does for the web service.
func inTimezone(t *testing.T, loc *time.Location) {
	t.Helper()
	previous := time.Local
	time.Local = loc
	t.Cleanup(func() { time.Local = previous })
}

var fiveHoursBehindUTC = time.FixedZone("UTC-5", -5*60*60)

func TestFormTimesAreReadInTheApplicationTimezone(t *testing.T) {
	inTimezone(t, fiveHoursBehindUTC)
	h, api := newAppointmentWebServer(t, store.Event{ID: 5})

	cases := []struct {
		name, path, field, value, payloadKey, want string
	}{
		{"task due date and time", "/tasks", "due_at", "2030-01-20T09:00", "POST /api/v1/tasks", "2030-01-20T09:00:00-05:00"},
		{"task due date", "/tasks", "due_at", "2030-01-20", "POST /api/v1/tasks", "2030-01-20T00:00:00-05:00"},
		{"edited task due", "/tasks/7", "due_at", "2030-01-20T09:00", "PATCH /api/v1/tasks/7", "2030-01-20T09:00:00-05:00"},
		{"project task due", "/projects/3/tasks", "due_at", "2030-01-20", "POST /api/v1/tasks", "2030-01-20T00:00:00-05:00"},
		{"list item due", "/lists/4/items", "due_at", "2030-01-20", "POST /api/v1/lists/4/items", "2030-01-20T00:00:00-05:00"},
		{"appointment start", "/events", "starts_at", "2030-01-20T09:00", "POST /api/v1/events", "2030-01-20T09:00:00-05:00"},
		{"appointment end", "/events", "ends_at", "2030-01-20T10:00", "POST /api/v1/events", "2030-01-20T10:00:00-05:00"},
		{"edited appointment start", "/events/5", "starts_at", "2030-01-20T09:00", "PATCH /api/v1/events/5", "2030-01-20T09:00:00-05:00"},
		{"project due date", "/projects", "due_date", "2030-01-20", "POST /api/v1/projects", "2030-01-20T00:00:00-05:00"},
		{"routine next due", "/routines", "next_due_at", "2030-01-20", "POST /api/v1/routines", "2030-01-20T00:00:00-05:00"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			form := url.Values{"title": {"Thing"}, "starts_at": {"2030-01-20T09:00"}, "ends_at": {"2030-01-20T10:00"}}
			form.Set(c.field, c.value)
			delete(api.payloads, c.payloadKey)
			postForm(h, c.path, form)
			if got := api.payloads[c.payloadKey][c.field]; got != c.want {
				t.Errorf("%s sent %s = %#v, want %q", c.path, c.field, got, c.want)
			}
		})
	}
}

func TestEditingKeepsADisplayedTimeUnchanged(t *testing.T) {
	inTimezone(t, fiveHoursBehindUTC)
	h, api := newAppointmentWebServer(t, store.Event{ID: 5})
	stored := time.Date(2030, time.January, 20, 14, 0, 0, 0, time.UTC)

	// The edit form shows the stored time in the application timezone...
	shown := formatDateTimeInput(stored)
	// ...and saving the form unchanged must send the same instant back.
	postForm(h, "/events/5", url.Values{"title": {"Dentist"}, "starts_at": {shown}, "ends_at": {formatDateTimeInput(stored.Add(time.Hour))}})
	sent, _ := api.payloads["PATCH /api/v1/events/5"]["starts_at"].(string)
	got, err := time.Parse(time.RFC3339, sent)
	if err != nil || !got.Equal(stored) {
		t.Errorf("saving %q unchanged sent starts_at %q, want the stored instant %s", shown, sent, stored)
	}
}

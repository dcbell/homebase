package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// appointmentPageWith serves the Appointment page for an Appointment the API
// returns as event.
func appointmentPageWith(t *testing.T, event string) string {
	t.Helper()
	_, web := newRecordingAPI(t, map[string]cannedResponse{
		"GET /api/v1/dashboard": {http.StatusOK, `{"current_user":{"id":1,"name":"Owner"}}`},
		"GET /api/v1/events/5":  {http.StatusOK, event},
	})
	return serve(web.routes(), browserRequest(http.MethodGet, "/events/5", "")).Body.String()
}

func TestPersonalAppointmentReminderTimesShowOnlyWhenTheAPIOffersThem(t *testing.T) {
	const event = `"id":5,"title":"Dentist","attendee_ids":[2],"reminder_times":[{"minutes":-120}]`

	// The API leaves them out for a Member who isn't an Attendee.
	if page := appointmentPageWith(t, `{`+event+`}`); strings.Contains(page, "event-personal-reminders") || strings.Contains(page, "Use your own Reminder Times") {
		t.Errorf("Appointment page offers Personal Reminder Times the API didn't:\n%s", page)
	}

	// Using the shared ones: "Use your own Reminder Times" starts from them.
	page := appointmentPageWith(t, `{`+event+`,"personal_reminder_times":{"using_shared":true,"reminder_times":[]}}`)
	if got, _ := reminderSetsIn(t, page, "event-personal-reminders"); fmt.Sprint(got) != fmt.Sprint(map[string][]string{"timed": {"2 hours before", "", "", "", ""}}) {
		t.Errorf("Personal Reminder Times rows = %q, want the shared ones", got)
	}
	if !strings.Contains(page, "Use your own Reminder Times") || strings.Contains(page, "event-use-shared-reminders") {
		t.Errorf("Appointment page using shared Reminder Times should offer Use your own Reminder Times only:\n%s", page)
	}

	// With their own: those show, and "Use shared" goes back.
	page = appointmentPageWith(t, `{`+event+`,"personal_reminder_times":{"using_shared":false,"reminder_times":[{"minutes":-30}]}}`)
	if got, _ := reminderSetsIn(t, page, "event-personal-reminders"); fmt.Sprint(got) != fmt.Sprint(map[string][]string{"timed": {"30 minutes before", "", "", "", ""}}) {
		t.Errorf("Personal Reminder Times rows = %q, want their own", got)
	}
	if !strings.Contains(page, `id="event-use-shared-reminders"`) || !strings.Contains(page, "Use shared") {
		t.Errorf("Appointment page with Personal Reminder Times has no Use shared:\n%s", page)
	}
}

func TestPersonalAppointmentReminderTimesChoicesAreSentToTheAPI(t *testing.T) {
	api, web := newRecordingAPI(t, map[string]cannedResponse{
		"PUT /api/v1/events/5/personal-reminder-times":    {http.StatusOK, `{"id":5}`},
		"DELETE /api/v1/events/5/personal-reminder-times": {http.StatusOK, `{"id":5}`},
	})
	post := func(path string, form url.Values) *http.Response {
		req := browserRequest(http.MethodPost, path, form.Encode())
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return serve(web.routes(), req).Result()
	}

	post("/events/5/personal-reminder-times", url.Values{
		"reminder_times":     {"1"},
		"reminder_amount":    {"2", "", "30"},
		"reminder_unit":      {"hours", "minutes", "minutes"},
		"reminder_direction": {"before", "before", "before"},
	})
	post("/events/5/personal-reminder-times", url.Values{
		"reminder_times":     {"1"},
		"reminder_amount":    {"", ""},
		"reminder_unit":      {"minutes", "minutes"},
		"reminder_direction": {"before", "at"},
	})
	res := post("/events/5/personal-reminder-times/shared", url.Values{})
	if got := res.Header.Get("Location"); !strings.HasPrefix(got, "/events/5") || strings.Contains(got, "error") {
		t.Errorf("Use shared redirected to %q, want back to the Appointment", got)
	}

	calls := api.Calls()
	if len(calls) != 3 {
		t.Fatalf("API calls = %+v, want two sets and a Use shared", calls)
	}
	if calls[0].Method != http.MethodPut || calls[0].Path != "/api/v1/events/5/personal-reminder-times" {
		t.Errorf("Use my own called %s %s, want PUT /api/v1/events/5/personal-reminder-times", calls[0].Method, calls[0].Path)
	}
	if got, want := sentReminderTimes(t, calls[0].Body), `[{"minutes":-120},{"minutes":-30}]`; got != want {
		t.Errorf("Use my own sent reminder_times %s, want %s", got, want)
	}
	if got := sentReminderTimes(t, calls[1].Body); got != `[]` {
		t.Errorf("Use my own with every row cleared sent reminder_times %s, want [] for no Reminders", got)
	}
	if calls[2].Method != http.MethodDelete || calls[2].Path != "/api/v1/events/5/personal-reminder-times" {
		t.Errorf("Use shared called %s %s, want DELETE /api/v1/events/5/personal-reminder-times", calls[2].Method, calls[2].Path)
	}
}

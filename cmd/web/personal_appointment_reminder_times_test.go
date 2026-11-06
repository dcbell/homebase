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
	if page := appointmentPageWith(t, `{`+event+`}`); strings.Contains(page, "Your Reminder Times") || strings.Contains(page, "event-personal-reminders") {
		t.Errorf("Appointment page offers Personal Reminder Times the API didn't:\n%s", page)
	}

	// Using the shared ones: they show read-only, and "Customize for me"
	// opens the editor seeded from them.
	page := appointmentPageWith(t, `{`+event+`,"personal_reminder_times":{"using_shared":true,"reminder_times":[]}}`)
	panel := personalReminderPanel(t, page)
	if got, want := readOnlyReminderSummaries(panel), []string{"2 hours before the start"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("read-only Reminder Times = %q, want the shared ones %q", got, want)
	}
	if !strings.Contains(panel, "You get this Appointment's Reminder Times.") || !strings.Contains(panel, ">Customize for me<") {
		t.Errorf("panel using shared Reminder Times should say so and offer Customize for me:\n%s", panel)
	}
	if got, _ := reminderSetsIn(t, page, "event-personal-reminders"); fmt.Sprint(got) != fmt.Sprint(map[string][]string{"timed": {"2 hours before the start | 2 hours before"}}) {
		t.Errorf("Personal Reminder Times frames = %q, want the shared ones", got)
	}
	if strings.Contains(page, `id="event-use-shared-reminders"`) {
		t.Errorf("panel using shared Reminder Times offers to go back to them:\n%s", panel)
	}

	// With their own: the editor shows them, with Save and "Use shared".
	page = appointmentPageWith(t, `{`+event+`,"personal_reminder_times":{"using_shared":false,"reminder_times":[{"minutes":-30}]}}`)
	panel = personalReminderPanel(t, page)
	if got, _ := reminderSetsIn(t, page, "event-personal-reminders"); fmt.Sprint(got) != fmt.Sprint(map[string][]string{"timed": {"30 minutes before the start | 30 minutes before"}}) {
		t.Errorf("Personal Reminder Times frames = %q, want their own", got)
	}
	if !strings.Contains(panel, "These replace this Appointment's Reminder Times for you. Only you see them.") || !strings.Contains(panel, `form="event-use-shared-reminders">Use shared</button>`) || !strings.Contains(page, `id="event-use-shared-reminders"`) {
		t.Errorf("customized panel should say so and offer Use shared:\n%s", panel)
	}
	if strings.Contains(panel, "Customize for me") {
		t.Errorf("customized panel offers Customize for me:\n%s", panel)
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
		t.Errorf("Use your own Reminder Times called %s %s, want PUT /api/v1/events/5/personal-reminder-times", calls[0].Method, calls[0].Path)
	}
	if got, want := sentReminderTimes(t, calls[0].Body), `[{"minutes":-120},{"minutes":-30}]`; got != want {
		t.Errorf("Use your own Reminder Times sent reminder_times %s, want %s", got, want)
	}
	if got := sentReminderTimes(t, calls[1].Body); got != `[]` {
		t.Errorf("Use your own Reminder Times with every row cleared sent reminder_times %s, want [] for no Reminders", got)
	}
	if calls[2].Method != http.MethodDelete || calls[2].Path != "/api/v1/events/5/personal-reminder-times" {
		t.Errorf("Use shared called %s %s, want DELETE /api/v1/events/5/personal-reminder-times", calls[2].Method, calls[2].Path)
	}
}

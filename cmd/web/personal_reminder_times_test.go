package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// taskPageWith serves the Task page for a Task the API returns as task.
func taskPageWith(t *testing.T, task string) string {
	t.Helper()
	_, web := newRecordingAPI(t, map[string]cannedResponse{
		"GET /api/v1/dashboard":               {http.StatusOK, `{"current_user":{"id":1,"name":"Owner"}}`},
		"GET /api/v1/reminder-times/defaults": {http.StatusOK, taskReminderDefaults},
		"GET /api/v1/tasks/7":                 {http.StatusOK, task},
		"GET /api/v1/documents":               {http.StatusOK, `[]`},
		"GET /api/v1/contacts":                {http.StatusOK, `[]`},
		"GET /api/v1/assets":                  {http.StatusOK, `[]`},
		"GET /api/v1/related-documents":       {http.StatusOK, `[]`},
		"GET /api/v1/related-contacts":        {http.StatusOK, `[]`},
		"GET /api/v1/related-assets":          {http.StatusOK, `[]`},
	})
	return serve(web.routes(), browserRequest(http.MethodGet, "/tasks/7", "")).Body.String()
}

func TestPersonalReminderTimesShowOnlyWhenTheAPIOffersThem(t *testing.T) {
	const task = `"id":7,"title":"Mow","due_at":"2030-01-20T18:00:00Z","reminder_times":[{"minutes":-120}]`

	// The API leaves them out for anyone but the assignee.
	if page := taskPageWith(t, `{`+task+`}`); strings.Contains(page, "task-personal-reminders") || strings.Contains(page, "Use your own Reminder Times") {
		t.Errorf("Task page offers Personal Reminder Times the API didn't:\n%s", page)
	}

	// Using the shared ones: "Use your own Reminder Times" starts from them.
	page := taskPageWith(t, `{`+task+`,"personal_reminder_times":{"using_shared":true,"reminder_times":[]}}`)
	sets, _ := reminderSetsIn(t, page, "task-personal-reminders")
	if want := map[string][]string{"timed": {"2 hours before", "", "", "", ""}}; fmt.Sprint(sets) != fmt.Sprint(want) {
		t.Errorf("Personal Reminder Times rows = %q, want the shared ones %q", sets, want)
	}
	if !strings.Contains(page, "Use your own Reminder Times") || strings.Contains(page, "task-use-shared-reminders") {
		t.Errorf("Task page using shared Reminder Times should offer Use your own Reminder Times only:\n%s", page)
	}

	// With their own: those show, and "Use shared" goes back.
	page = taskPageWith(t, `{`+task+`,"personal_reminder_times":{"using_shared":false,"reminder_times":[{"minutes":-30}]}}`)
	sets, _ = reminderSetsIn(t, page, "task-personal-reminders")
	if want := map[string][]string{"timed": {"30 minutes before", "", "", "", ""}}; fmt.Sprint(sets) != fmt.Sprint(want) {
		t.Errorf("Personal Reminder Times rows = %q, want %q", sets, want)
	}
	if !strings.Contains(page, `id="task-use-shared-reminders"`) || !strings.Contains(page, "Use shared") {
		t.Errorf("Task page with Personal Reminder Times has no Use shared:\n%s", page)
	}
}

func TestPersonalReminderTimesChoicesAreSentToTheAPI(t *testing.T) {
	api, web := newRecordingAPI(t, map[string]cannedResponse{
		"PUT /api/v1/tasks/7/personal-reminder-times":    {http.StatusOK, `{"id":7}`},
		"DELETE /api/v1/tasks/7/personal-reminder-times": {http.StatusOK, `{"id":7}`},
	})
	post := func(path string, form url.Values) *http.Response {
		req := browserRequest(http.MethodPost, path, form.Encode())
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return serve(web.routes(), req).Result()
	}

	post("/tasks/7/personal-reminder-times", url.Values{
		"reminder_times":     {"1"},
		"reminder_amount":    {"30", "", "1"},
		"reminder_unit":      {"minutes", "minutes", "days"},
		"reminder_direction": {"before", "before", "after"},
		"reminder_at":        {"", "", "08:00"},
	})
	post("/tasks/7/personal-reminder-times", url.Values{
		"reminder_times":     {"1"},
		"reminder_amount":    {"", ""},
		"reminder_direction": {"before", "before"},
	})
	res := post("/tasks/7/personal-reminder-times/shared", url.Values{})
	if got := res.Header.Get("Location"); !strings.HasPrefix(got, "/tasks/7") || strings.Contains(got, "error") {
		t.Errorf("Use shared redirected to %q, want back to the Task", got)
	}

	calls := api.Calls()
	if len(calls) != 3 {
		t.Fatalf("API calls = %+v, want two sets and a Use shared", calls)
	}
	if calls[0].Method != http.MethodPut || calls[0].Path != "/api/v1/tasks/7/personal-reminder-times" {
		t.Errorf("Use my own called %s %s, want PUT /api/v1/tasks/7/personal-reminder-times", calls[0].Method, calls[0].Path)
	}
	if got, want := sentReminderTimes(t, calls[0].Body), `[{"minutes":-30},{"days":1,"at":"08:00"}]`; got != want {
		t.Errorf("Use my own sent reminder_times %s, want %s", got, want)
	}
	if got := sentReminderTimes(t, calls[1].Body); got != `[]` {
		t.Errorf("Use my own with every row cleared sent reminder_times %s, want [] for no Reminders", got)
	}
	if calls[2].Method != http.MethodDelete || calls[2].Path != "/api/v1/tasks/7/personal-reminder-times" {
		t.Errorf("Use shared called %s %s, want DELETE /api/v1/tasks/7/personal-reminder-times", calls[2].Method, calls[2].Path)
	}
}

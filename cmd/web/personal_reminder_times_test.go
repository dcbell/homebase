package main

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
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
	if page := taskPageWith(t, `{`+task+`}`); strings.Contains(page, "Your Reminder Times") || strings.Contains(page, "task-personal-reminders") {
		t.Errorf("Task page offers Personal Reminder Times the API didn't:\n%s", page)
	}

	// Using the shared ones: they show read-only, and "Customize for me"
	// opens the editor seeded from them.
	page := taskPageWith(t, `{`+task+`,"personal_reminder_times":{"using_shared":true,"reminder_times":[]}}`)
	panel := personalReminderPanel(t, page)
	if got, want := readOnlyReminderSummaries(panel), []string{"2 hours before"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("read-only Reminder Times = %q, want the shared ones %q", got, want)
	}
	if !strings.Contains(panel, "You get this Task's Reminder Times.") || !strings.Contains(panel, ">Customize for me<") {
		t.Errorf("panel using shared Reminder Times should say so and offer Customize for me:\n%s", panel)
	}
	sets, _ := reminderSetsIn(t, page, "task-personal-reminders")
	if want := map[string][]string{"timed": {"2 hours before | 2 hours before"}}; fmt.Sprint(sets) != fmt.Sprint(want) {
		t.Errorf("Personal Reminder Times frames = %q, want the shared ones %q", sets, want)
	}
	if strings.Contains(page, `id="task-use-shared-reminders"`) {
		t.Errorf("panel using shared Reminder Times offers to go back to them:\n%s", panel)
	}

	// With their own: the editor shows them, with Save and "Use shared".
	page = taskPageWith(t, `{`+task+`,"personal_reminder_times":{"using_shared":false,"reminder_times":[{"minutes":-30}]}}`)
	panel = personalReminderPanel(t, page)
	sets, _ = reminderSetsIn(t, page, "task-personal-reminders")
	if want := map[string][]string{"timed": {"30 minutes before | 30 minutes before"}}; fmt.Sprint(sets) != fmt.Sprint(want) {
		t.Errorf("Personal Reminder Times frames = %q, want %q", sets, want)
	}
	if !strings.Contains(panel, "These replace this Task's Reminder Times for you. Only you see them.") || !strings.Contains(panel, `<button type="submit">Save</button>`) {
		t.Errorf("customized panel should say so and offer Save:\n%s", panel)
	}
	if !strings.Contains(panel, `form="task-use-shared-reminders">Use shared</button>`) || !strings.Contains(page, `id="task-use-shared-reminders"`) {
		t.Errorf("customized panel has no Use shared:\n%s", panel)
	}
	if strings.Contains(panel, "Customize for me") || len(readOnlyReminderSummaries(panel)) != 0 {
		t.Errorf("customized panel shows the read-only shared view:\n%s", panel)
	}
}

// personalReminderPanel returns the "Your Reminder Times" panel of a page.
func personalReminderPanel(t *testing.T, page string) string {
	t.Helper()
	panel := regexp.MustCompile(`(?s)<section class="panel personal-reminder-times".*?</section>`).FindString(page)
	if panel == "" {
		t.Fatalf("page has no Your Reminder Times panel:\n%s", page)
	}
	return panel
}

// readOnlyReminderSummaries are the summaries of a panel's read-only frames.
func readOnlyReminderSummaries(panel string) []string {
	got := []string{}
	for _, m := range regexp.MustCompile(`<div class="rt-frame static">([^<]*)</div>`).FindAllStringSubmatch(panel, -1) {
		got = append(got, m[1])
	}
	return got
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
		t.Errorf("Use your own Reminder Times called %s %s, want PUT /api/v1/tasks/7/personal-reminder-times", calls[0].Method, calls[0].Path)
	}
	if got, want := sentReminderTimes(t, calls[0].Body), `[{"minutes":-30},{"days":1,"at":"08:00"}]`; got != want {
		t.Errorf("Use your own Reminder Times sent reminder_times %s, want %s", got, want)
	}
	if got := sentReminderTimes(t, calls[1].Body); got != `[]` {
		t.Errorf("Use your own Reminder Times with every row cleared sent reminder_times %s, want [] for no Reminders", got)
	}
	if calls[2].Method != http.MethodDelete || calls[2].Path != "/api/v1/tasks/7/personal-reminder-times" {
		t.Errorf("Use shared called %s %s, want DELETE /api/v1/tasks/7/personal-reminder-times", calls[2].Method, calls[2].Path)
	}
}

func TestPersonalReminderTimesSaveOnlyTheTimeOfDayThatAppliesToEachReminderTime(t *testing.T) {
	got := postReminderTimes(t, "/tasks/7/personal-reminder-times", "PUT /api/v1/tasks/7/personal-reminder-times", url.Values{}, "timed",
		[4]string{"1", "hours", "before", "08:00"},
		[4]string{"1", "days", "before", "08:00"},
		[4]string{"0", "days", "at", "08:00"},
		[4]string{"0", "days", "at", ""},
	)
	if want := `[{"minutes":-60},{"days":-1,"at":"08:00"},{"days":0,"at":"08:00"},{"minutes":0}]`; got != want {
		t.Errorf("timed Task's Personal Reminder Times sent %s, want %s", got, want)
	}

	got = postReminderTimes(t, "/tasks/7/personal-reminder-times", "PUT /api/v1/tasks/7/personal-reminder-times", url.Values{}, "date_only",
		[4]string{"1", "days", "before", "18:00"},
		[4]string{"0", "days", "at", "08:00"},
	)
	if want := `[{"days":-1,"at":"18:00"},{"days":0,"at":"08:00"}]`; got != want {
		t.Errorf("date-only Task's Personal Reminder Times sent %s, want %s", got, want)
	}
}

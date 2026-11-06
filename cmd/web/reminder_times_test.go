package main

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

const taskReminderDefaults = `{
	"timed_task": [{"minutes": -2880}, {"minutes": -90}, {"minutes": 0}, {"days": 1, "at": "09:00"}],
	"date_only_task": [{"days": -2, "at": "07:30"}, {"days": 0, "at": "07:30"}]
}`

// reminderSetsIn returns the Reminder Time frames of the form with the
// given id, keyed by the kind of due date each set is for, and which set is
// shown. Frames read as "summary | amount unit direction at", from the
// summary the frame shows and the fields it submits.
func reminderSetsIn(t *testing.T, page, id string) (sets map[string][]string, shown string) {
	t.Helper()
	form := regexp.MustCompile(`(?s)id="` + id + `".*?</form>`).FindString(page)
	if form == "" {
		t.Fatalf("page has no #%s form:\n%s", id, page)
	}
	sets = map[string][]string{}
	fieldset := regexp.MustCompile(`(?s)<fieldset class="reminder-times" data-reminder-kind="(\w+)"([^>]*)>(.*?)</fieldset>`)
	newFrame := regexp.MustCompile(`(?s)<template.*?</template>`)
	summary := regexp.MustCompile(`<span class="rt-summary"[^>]*>([^<]*)</span>`)
	value := func(html, name string) string {
		if m := regexp.MustCompile(`(?s)<select [^>]*name="` + name + `"[^>]*>(.*?)</select>`).FindStringSubmatch(html); m != nil {
			if s := regexp.MustCompile(`<option value="(\w+)" selected>`).FindStringSubmatch(m[1]); s != nil {
				return s[1]
			}
			return ""
		}
		if m := regexp.MustCompile(`name="` + name + `"[^>]*value="([^"]*)"`).FindStringSubmatch(html); m != nil {
			return m[1]
		}
		return ""
	}
	for _, f := range fieldset.FindAllStringSubmatch(form, -1) {
		if !strings.Contains(f[2], "disabled") {
			shown = f[1]
		}
		frames := strings.Split(newFrame.ReplaceAllString(f[3], ""), `<div class="rt-frame"`)[1:]
		rows := []string{}
		for _, frame := range frames {
			parts := []string{}
			for _, p := range []string{value(frame, "reminder_amount"), value(frame, "reminder_unit"), value(frame, "reminder_direction"), value(frame, "reminder_at")} {
				if p != "" {
					parts = append(parts, p)
				}
			}
			text := ""
			if m := summary.FindStringSubmatch(frame); m != nil {
				text = m[1]
			}
			rows = append(rows, text+" | "+strings.Join(parts, " "))
		}
		sets[f[1]] = rows
	}
	return sets, shown
}

func TestAddTaskFormsShowTheAPIsDefaultReminderTimes(t *testing.T) {
	_, web := newRecordingAPI(t, map[string]cannedResponse{
		"GET /api/v1/dashboard":               {http.StatusOK, `{"current_user":{"id":1,"name":"Owner"}}`},
		"GET /api/v1/reminder-times/defaults": {http.StatusOK, taskReminderDefaults},
		"GET /api/v1/projects/3":              {http.StatusOK, `{"id":3,"title":"Garden"}`},
		"GET /api/v1/projects/3/folders":      {http.StatusOK, `[]`},
		"GET /api/v1/documents":               {http.StatusOK, `[]`},
		"GET /api/v1/contacts":                {http.StatusOK, `[]`},
		"GET /api/v1/assets":                  {http.StatusOK, `[]`},
		"GET /api/v1/related-documents":       {http.StatusOK, `[]`},
		"GET /api/v1/related-contacts":        {http.StatusOK, `[]`},
		"GET /api/v1/related-assets":          {http.StatusOK, `[]`},
	})
	timed := []string{"2 days before | 2 days before", "90 minutes before | 90 minutes before", "At the due time | 0 minutes at", "1 day after at 09:00 | 1 days after 09:00"}
	dateOnly := []string{"2 days before at 07:30 | 2 days before 07:30", "On the due date at 07:30 | 0 days at 07:30"}

	// A due time of day is picked on these forms, so the timed defaults
	// show, and the date-only ones are ready for a due time of midnight.
	for path, id := range map[string]string{"/": "add-task-form", "/tasks": "task-create-form"} {
		page := serve(web.routes(), browserRequest(http.MethodGet, path, "")).Body.String()
		sets, shown := reminderSetsIn(t, page, id)
		if fmt.Sprint(sets["timed"]) != fmt.Sprint(timed) || fmt.Sprint(sets["date_only"]) != fmt.Sprint(dateOnly) || shown != "timed" {
			t.Errorf("%s Reminder Times = %q, showing %q; want timed %q and date-only %q, showing timed", path, sets, shown, timed, dateOnly)
		}
	}

	// A Project's Tasks are due on a date.
	page := serve(web.routes(), browserRequest(http.MethodGet, "/projects/3", "")).Body.String()
	sets, shown := reminderSetsIn(t, page, "project-task-create-form")
	if fmt.Sprint(sets) != fmt.Sprint(map[string][]string{"date_only": dateOnly}) || shown != "date_only" {
		t.Errorf("Project Task Reminder Times = %q, showing %q; want only date-only %q", sets, shown, dateOnly)
	}
}

func TestTaskEditFormShowsItsReminderTimes(t *testing.T) {
	_, web := newRecordingAPI(t, map[string]cannedResponse{
		"GET /api/v1/dashboard":               {http.StatusOK, `{"current_user":{"id":1,"name":"Owner"}}`},
		"GET /api/v1/reminder-times/defaults": {http.StatusOK, taskReminderDefaults},
		"GET /api/v1/tasks/7":                 {http.StatusOK, `{"id":7,"title":"Mow","due_at":"2030-01-20T18:00:00Z","reminder_times":[{"minutes":-120},{"days":3,"at":"08:00"}]}`},
		"GET /api/v1/documents":               {http.StatusOK, `[]`},
		"GET /api/v1/contacts":                {http.StatusOK, `[]`},
		"GET /api/v1/assets":                  {http.StatusOK, `[]`},
		"GET /api/v1/related-documents":       {http.StatusOK, `[]`},
		"GET /api/v1/related-contacts":        {http.StatusOK, `[]`},
		"GET /api/v1/related-assets":          {http.StatusOK, `[]`},
	})

	page := serve(web.routes(), browserRequest(http.MethodGet, "/tasks/7", "")).Body.String()

	// Moving it to a date with no time of day shows the date-only defaults,
	// as the API resets them on that switch.
	sets, shown := reminderSetsIn(t, page, "task-edit")
	want := map[string][]string{
		"timed":     {"2 hours before | 2 hours before", "3 days after at 08:00 | 3 days after 08:00"},
		"date_only": {"2 days before at 07:30 | 2 days before 07:30", "On the due date at 07:30 | 0 days at 07:30"},
	}
	if fmt.Sprint(sets) != fmt.Sprint(want) || shown != "timed" {
		t.Errorf("Reminder Times = %q, showing %q; want %q, showing timed", sets, shown, want)
	}
}

func TestTaskFormsSendReminderTimesToTheAPIAsStructuredValues(t *testing.T) {
	api, web := newRecordingAPI(t, map[string]cannedResponse{
		"POST /api/v1/tasks":    {http.StatusCreated, `{"id":7}`},
		"PATCH /api/v1/tasks/7": {http.StatusOK, `{"id":7}`},
	})
	form := url.Values{
		"title": {"Pay the water bill"}, "due_at": {"2030-01-20T18:00"},
		"reminder_times":     {"1"},
		"reminder_amount":    {"2", "", "30", "0", "1"},
		"reminder_unit":      {"hours", "minutes", "minutes", "minutes", "days"},
		"reminder_direction": {"before", "before", "after", "at", "after"},
		"reminder_at":        {"", "", "", "", "08:00"},
	}
	post := func(path string, form url.Values) {
		req := browserRequest(http.MethodPost, path, form.Encode())
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		serve(web.routes(), req)
	}

	post("/tasks", form)
	cleared := url.Values{}
	for k, v := range form {
		cleared[k] = v
	}
	cleared["reminder_amount"] = []string{"", "", "", "", ""}
	cleared["reminder_direction"] = []string{"before", "before", "before", "before", "before"}
	post("/tasks/7", cleared)
	// An inline form, such as the due date picker, resends the Task without
	// its Reminder Times, so the API keeps them.
	post("/tasks/7", url.Values{"title": {"Pay the water bill"}, "due_at": {"2030-01-21T18:00"}})

	calls := api.Calls()
	if len(calls) != 3 {
		t.Fatalf("API calls = %+v, want a create and two edits", calls)
	}
	if got, want := sentReminderTimes(t, calls[0].Body), `[{"minutes":-120},{"minutes":30},{"minutes":0},{"days":1,"at":"08:00"}]`; got != want {
		t.Errorf("create sent reminder_times %s, want %s", got, want)
	}
	if got := sentReminderTimes(t, calls[1].Body); got != `[]` {
		t.Errorf("edit with every row cleared sent reminder_times %s, want [] for no Reminders", got)
	}
	if got := sentReminderTimes(t, calls[2].Body); got != "absent" {
		t.Errorf("inline edit sent reminder_times %s, want them left out", got)
	}
}

func TestTaskEditFormFollowsTheAPIOnWhetherTheDueDateIsDateOnly(t *testing.T) {
	_, web := newRecordingAPI(t, map[string]cannedResponse{
		"GET /api/v1/dashboard":               {http.StatusOK, `{"current_user":{"id":1,"name":"Owner"}}`},
		"GET /api/v1/reminder-times/defaults": {http.StatusOK, taskReminderDefaults},
		"GET /api/v1/tasks/7":                 {http.StatusOK, `{"id":7,"title":"Mow","due_at":"2030-01-20T06:00:00Z","date_only":true,"reminder_times":[{"days":-1,"at":"18:00"}]}`},
		"GET /api/v1/documents":               {http.StatusOK, `[]`},
		"GET /api/v1/contacts":                {http.StatusOK, `[]`},
		"GET /api/v1/assets":                  {http.StatusOK, `[]`},
		"GET /api/v1/related-documents":       {http.StatusOK, `[]`},
		"GET /api/v1/related-contacts":        {http.StatusOK, `[]`},
		"GET /api/v1/related-assets":          {http.StatusOK, `[]`},
	})

	page := serve(web.routes(), browserRequest(http.MethodGet, "/tasks/7", "")).Body.String()

	sets, shown := reminderSetsIn(t, page, "task-edit")
	if want := []string{"1 day before at 18:00 | 1 days before 18:00"}; fmt.Sprint(sets["date_only"]) != fmt.Sprint(want) || shown != "date_only" {
		t.Errorf("Reminder Times = %q, showing %q; want date-only %q shown", sets, shown, want)
	}
}

func TestClearingTheNumberOfTheAtTheTimeRowDropsIt(t *testing.T) {
	api, web := newRecordingAPI(t, map[string]cannedResponse{
		"PATCH /api/v1/tasks/7": {http.StatusOK, `{"id":7}`},
	})
	form := url.Values{
		"title": {"Pay the water bill"}, "due_at": {"2030-01-20T18:00"},
		"reminder_times":     {"1"},
		"reminder_amount":    {"1", ""},
		"reminder_unit":      {"hours", "minutes"},
		"reminder_direction": {"before", "at"},
		"reminder_at":        {"", ""},
	}
	req := browserRequest(http.MethodPost, "/tasks/7", form.Encode())
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	serve(web.routes(), req)

	calls := api.Calls()
	if len(calls) != 1 {
		t.Fatalf("API calls = %+v, want one edit", calls)
	}
	if got, want := sentReminderTimes(t, calls[0].Body), `[{"minutes":-60}]`; got != want {
		t.Errorf("edit sent reminder_times %s, want %s without the cleared row", got, want)
	}
}

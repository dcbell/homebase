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
	value := reminderFieldValue
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

// reminderFieldValue is the value a frame's markup submits for the field
// name, the selected option of a select, or "" when it has none.
func reminderFieldValue(html, name string) string {
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

// editorPage serves the page at path with the API returning record there.
func editorPage(t *testing.T, path, record string) string {
	t.Helper()
	_, web := newRecordingAPI(t, map[string]cannedResponse{
		"GET /api/v1/dashboard":               {http.StatusOK, `{"current_user":{"id":1,"name":"Owner"}}`},
		"GET /api/v1/reminder-times/defaults": {http.StatusOK, taskReminderDefaults},
		"GET /api/v1" + path:                  {http.StatusOK, record},
		"GET /api/v1/documents":               {http.StatusOK, `[]`},
		"GET /api/v1/contacts":                {http.StatusOK, `[]`},
		"GET /api/v1/assets":                  {http.StatusOK, `[]`},
		"GET /api/v1/related-documents":       {http.StatusOK, `[]`},
		"GET /api/v1/related-contacts":        {http.StatusOK, `[]`},
		"GET /api/v1/related-assets":          {http.StatusOK, `[]`},
	})
	return serve(web.routes(), browserRequest(http.MethodGet, path, "")).Body.String()
}

// reminderFramesIn returns the markup of each frame in the set for kind on
// the form with the given id.
func reminderFramesIn(t *testing.T, page, id, kind string) []string {
	t.Helper()
	form := regexp.MustCompile(`(?s)id="` + id + `".*?</form>`).FindString(page)
	set := regexp.MustCompile(`(?s)data-reminder-kind="` + kind + `".*?</fieldset>`).FindString(form)
	set = regexp.MustCompile(`(?s)<template.*?</template>`).ReplaceAllString(set, "")
	return strings.Split(set, `<div class="rt-frame"`)[1:]
}

func TestReminderTimesEditorSendsBackWhatItLoadedOrNoneOnceAllAreRemoved(t *testing.T) {
	const times = `[{"minutes":-1440},{"minutes":-90},{"minutes":0},{"days":1,"at":"08:00"}]`
	page := editorPage(t, "/tasks/7", `{"id":7,"title":"Mow","due_at":"2030-01-20T18:00:00Z","reminder_times":`+times+`}`)
	api, web := newRecordingAPI(t, map[string]cannedResponse{
		"PATCH /api/v1/tasks/7": {http.StatusOK, `{"id":7}`},
	})
	post := func(form url.Values) {
		req := browserRequest(http.MethodPost, "/tasks/7", form.Encode())
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		serve(web.routes(), req)
	}

	// The frames as the server renders them, unedited.
	unedited := url.Values{"title": {"Mow"}, "due_at": {"2030-01-20T18:00"}, "reminder_times": {"1"}}
	for _, frame := range reminderFramesIn(t, page, "task-edit", "timed") {
		for _, name := range []string{"reminder_amount", "reminder_unit", "reminder_direction", "reminder_at"} {
			unedited.Add(name, reminderFieldValue(frame, name))
		}
	}
	post(unedited)
	// Every frame removed leaves only the editor's marker.
	post(url.Values{"title": {"Mow"}, "due_at": {"2030-01-20T18:00"}, "reminder_times": {"1"}})

	calls := api.Calls()
	if len(calls) != 2 {
		t.Fatalf("API calls = %+v, want two edits", calls)
	}
	if got := sentReminderTimes(t, calls[0].Body); got != times {
		t.Errorf("unedited form sent reminder_times %s, want the ones it loaded %s", got, times)
	}
	if got := sentReminderTimes(t, calls[1].Body); got != `[]` {
		t.Errorf("form with every frame removed sent reminder_times %s, want [] for no Reminders", got)
	}
}

func TestReminderTimeSummariesSayEachKindInPlainLanguage(t *testing.T) {
	summaries := func(page, id, kind string) []string {
		sets, _ := reminderSetsIn(t, page, id)
		got := []string{}
		for _, frame := range sets[kind] {
			got = append(got, strings.Split(frame, " | ")[0])
		}
		return got
	}

	page := editorPage(t, "/tasks/7", `{"id":7,"title":"Mow","due_at":"2030-01-20T18:00:00Z","reminder_times":[{"minutes":-1440},{"minutes":-60},{"minutes":-1},{"minutes":2880},{"days":1,"at":"08:00"}]}`)
	if got, want := summaries(page, "task-edit", "timed"), []string{"1 day before", "1 hour before", "1 minute before", "2 days after", "1 day after at 08:00"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("timed Task summaries = %q, want %q", got, want)
	}

	page = editorPage(t, "/tasks/7", `{"id":7,"title":"Mow","due_at":"2030-01-20T06:00:00Z","date_only":true,"reminder_times":[{"days":-1,"at":"18:00"},{"days":0,"at":"08:00"},{"days":2,"at":"08:00"}]}`)
	if got, want := summaries(page, "task-edit", "date_only"), []string{"1 day before at 18:00", "On the due date at 08:00", "2 days after at 08:00"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("date-only Task summaries = %q, want %q", got, want)
	}

	page = editorPage(t, "/events/5", `{"id":5,"title":"Dentist","reminder_times":[{"minutes":-60},{"minutes":-120},{"minutes":0}]}`)
	if got, want := summaries(page, "event-edit", "timed"), []string{"1 hour before the start", "2 hours before the start", "At the start"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("Appointment summaries = %q, want %q", got, want)
	}
}

func TestReminderTimeFramesOfferOnlyTheControlsForTheirKind(t *testing.T) {
	pills := func(frame string) []string {
		got := []string{}
		for _, m := range regexp.MustCompile(`data-reminder-pick="(\w+)"[^>]*>([^<]*)<`).FindAllStringSubmatch(frame, -1) {
			got = append(got, m[1]+"="+m[2])
		}
		return got
	}

	page := editorPage(t, "/tasks/7", `{"id":7,"title":"Mow","due_at":"2030-01-20T18:00:00Z","reminder_times":[{"minutes":-60}]}`)
	timed := reminderFramesIn(t, page, "task-edit", "timed")[0]
	if got, want := pills(timed), []string{"before=Before", "at=At due time", "after=After"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("timed Task pills = %q, want %q", got, want)
	}
	if !strings.Contains(timed, `<select name="reminder_unit"`) || !strings.Contains(timed, `name="reminder_at"`) {
		t.Errorf("timed Task frame should pick a unit, and a time of day for days:\n%s", timed)
	}
	dateOnly := reminderFramesIn(t, page, "task-edit", "date_only")[0]
	if got, want := pills(dateOnly), []string{"before=Before", "at=On the day", "after=After"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("date-only Task pills = %q, want %q", got, want)
	}
	if strings.Contains(dateOnly, `<select name="reminder_unit"`) || !strings.Contains(dateOnly, `name="reminder_unit" type="hidden" value="days"`) || !strings.Contains(dateOnly, `name="reminder_at"`) {
		t.Errorf("date-only Task frame should count days, with no unit picker, at a time of day:\n%s", dateOnly)
	}

	page = editorPage(t, "/events/5", `{"id":5,"title":"Dentist","reminder_times":[{"minutes":-60}]}`)
	appointment := reminderFramesIn(t, page, "event-edit", "timed")[0]
	if got, want := pills(appointment), []string{"before=Before", "at=At the start"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("Appointment pills = %q, want %q", got, want)
	}
	if strings.Contains(appointment, `name="reminder_at"`) {
		t.Errorf("Appointment frame offers a time of day:\n%s", appointment)
	}
}

func TestReminderTimesEditorAddsUpToFiveAndSaysWhenThereAreNone(t *testing.T) {
	set := func(page string) string {
		form := regexp.MustCompile(`(?s)id="task-edit".*?</form>`).FindString(page)
		return regexp.MustCompile(`(?s)data-reminder-kind="timed".*?</fieldset>`).FindString(form)
	}
	hidden := func(set, marker string) bool {
		tag := regexp.MustCompile(`<[^>]*` + marker + `[^>]*>`).FindString(set)
		if tag == "" {
			t.Fatalf("set has no %s:\n%s", marker, set)
		}
		return strings.Contains(tag, " hidden")
	}

	none := set(editorPage(t, "/tasks/7", `{"id":7,"title":"Mow","due_at":"2030-01-20T18:00:00Z","reminder_times":[]}`))
	if hidden(none, "data-reminder-empty") || hidden(none, "data-reminder-add") || !strings.Contains(none, "No reminders") {
		t.Errorf("a Task with no Reminder Times should say No reminders and offer to add one:\n%s", none)
	}

	five := set(editorPage(t, "/tasks/7", `{"id":7,"title":"Mow","due_at":"2030-01-20T18:00:00Z","reminder_times":[{"minutes":-5},{"minutes":-4},{"minutes":-3},{"minutes":-2},{"minutes":-1}]}`))
	if !hidden(five, "data-reminder-empty") || !hidden(five, "data-reminder-add") {
		t.Errorf("a Task with five Reminder Times should hide No reminders and Add reminder:\n%s", five)
	}
}

// postReminderTimes posts form to path with the shown set's kind and its
// Reminder Time rows, each "amount unit direction at", and returns the
// Reminder Times the API receives on apiRoute.
func postReminderTimes(t *testing.T, path, apiRoute string, form url.Values, kind string, rows ...[4]string) string {
	t.Helper()
	api, web := newRecordingAPI(t, map[string]cannedResponse{apiRoute: {http.StatusOK, `{"id":7}`}})
	form.Set("reminder_times", "1")
	form.Set("reminder_kind", kind)
	for _, row := range rows {
		form.Add("reminder_amount", row[0])
		form.Add("reminder_unit", row[1])
		form.Add("reminder_direction", row[2])
		form.Add("reminder_at", row[3])
	}
	req := browserRequest(http.MethodPost, path, form.Encode())
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	serve(web.routes(), req)
	calls := api.Calls()
	if len(calls) != 1 {
		t.Fatalf("API calls = %+v, want one to %s", calls, apiRoute)
	}
	return sentReminderTimes(t, calls[0].Body)
}

func TestEveryReminderTimesSetSubmitsItsKind(t *testing.T) {
	kindField := func(page, id, kind string) string {
		form := regexp.MustCompile(`(?s)id="` + id + `".*?</form>`).FindString(page)
		set := regexp.MustCompile(`(?s)<fieldset class="reminder-times" data-reminder-kind="` + kind + `".*?</fieldset>`).FindString(form)
		set = regexp.MustCompile(`(?s)<template.*?</template>`).ReplaceAllString(set, "")
		return regexp.MustCompile(`<input type="hidden" name="reminder_kind"[^>]*>`).FindString(set)
	}

	page := editorPage(t, "/tasks/7", `{"id":7,"title":"Mow","due_at":"2030-01-20T18:00:00Z","reminder_times":[{"minutes":-60}]}`)
	// Only the shown set is enabled, so only its kind is sent.
	for _, kind := range []string{"timed", "date_only"} {
		if got, want := kindField(page, "task-edit", kind), `<input type="hidden" name="reminder_kind" value="`+kind+`">`; got != want {
			t.Errorf("Task %s set's kind field = %q, want %q", kind, got, want)
		}
	}
	page = editorPage(t, "/events/5", `{"id":5,"title":"Dentist","reminder_times":[{"minutes":-60}]}`)
	if got, want := kindField(page, "event-edit", "timed"), `<input type="hidden" name="reminder_kind" value="timed">`; got != want {
		t.Errorf("Appointment set's kind field = %q, want %q", got, want)
	}
}

func TestTaskFormsSaveOnlyTheTimeOfDayThatAppliesToEachReminderTime(t *testing.T) {
	timedTask := func() url.Values {
		return url.Values{"title": {"Pay the water bill"}, "due_at": {"2030-01-20T18:00"}}
	}

	got := postReminderTimes(t, "/tasks/7", "PATCH /api/v1/tasks/7", timedTask(), "timed",
		[4]string{"1", "hours", "before", "08:00"},
		[4]string{"30", "minutes", "after", "08:00"},
		[4]string{"1", "days", "before", "08:00"},
		[4]string{"1", "days", "before", ""},
		[4]string{"0", "hours", "at", "08:00"},
	)
	if want := `[{"minutes":-60},{"minutes":30},{"days":-1,"at":"08:00"},{"minutes":-1440},{"days":0,"at":"08:00"}]`; got != want {
		t.Errorf("timed Task sent reminder_times %s, want %s", got, want)
	}

	got = postReminderTimes(t, "/tasks/7", "PATCH /api/v1/tasks/7", timedTask(), "timed",
		[4]string{"0", "days", "at", ""},
		[4]string{"5", "hours", "at", ""},
	)
	if want := `[{"minutes":0},{"minutes":0}]`; got != want {
		t.Errorf("timed Task at the due time sent reminder_times %s, want %s", got, want)
	}

	got = postReminderTimes(t, "/tasks", "POST /api/v1/tasks", url.Values{"title": {"Mow"}, "due_at": {"2030-01-20"}}, "date_only",
		[4]string{"1", "days", "before", "18:00"},
		[4]string{"0", "days", "at", "08:00"},
		[4]string{"2", "days", "after", "07:30"},
	)
	if want := `[{"days":-1,"at":"18:00"},{"days":0,"at":"08:00"},{"days":2,"at":"07:30"}]`; got != want {
		t.Errorf("date-only Task sent reminder_times %s, want %s", got, want)
	}
}

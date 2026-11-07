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
	unedited := url.Values{"title": {"Mow"}, "due_at": {"2030-01-20T18:00"}, "reminder_times": {"1"}, "reminder_kind": {"timed"}}
	for _, frame := range reminderFramesIn(t, page, "task-edit", "timed") {
		for _, name := range []string{"reminder_amount", "reminder_unit", "reminder_direction", "reminder_at"} {
			unedited.Add(name, reminderFieldValue(frame, name))
		}
	}
	post(unedited)
	// Every frame removed leaves only the editor's marker.
	post(url.Values{"title": {"Mow"}, "due_at": {"2030-01-20T18:00"}, "reminder_times": {"1"}, "reminder_kind": {"timed"}})

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

	// atOption is what the no-JS direction select calls the "at" direction.
	atOption := func(frame string) string {
		return regexp.MustCompile(`<option value="at"[^>]*>([^<]*)<`).FindStringSubmatch(frame)[1]
	}
	const blankHint = "blank = at the due time"

	page := editorPage(t, "/tasks/7", `{"id":7,"title":"Mow","due_at":"2030-01-20T18:00:00Z","reminder_times":[{"minutes":-60}]}`)
	timed := reminderFramesIn(t, page, "task-edit", "timed")[0]
	if got, want := pills(timed), []string{"before=Before", "at=On the due date", "after=After"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("timed Task pills = %q, want %q", got, want)
	}
	if got, want := atOption(timed), "on the due date"; got != want {
		t.Errorf("timed Task select says %q for at, want %q", got, want)
	}
	if !regexp.MustCompile(`(?s)data-reminder-at.*` + blankHint + `.*</label>`).MatchString(timed) {
		t.Errorf("timed Task time of day has no %q hint:\n%s", blankHint, timed)
	}
	if !strings.Contains(timed, `<select name="reminder_unit"`) || !strings.Contains(timed, `name="reminder_at"`) {
		t.Errorf("timed Task frame should pick a unit, and a time of day for days:\n%s", timed)
	}
	dateOnly := reminderFramesIn(t, page, "task-edit", "date_only")[0]
	if got, want := pills(dateOnly), []string{"before=Before", "at=On the day", "after=After"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("date-only Task pills = %q, want %q", got, want)
	}
	if got, want := atOption(dateOnly), "on the day"; got != want {
		t.Errorf("date-only Task select says %q for at, want %q", got, want)
	}
	if strings.Contains(dateOnly, blankHint) {
		t.Errorf("date-only Task time of day can't be blank, but says %q:\n%s", blankHint, dateOnly)
	}
	if strings.Contains(dateOnly, `<select name="reminder_unit"`) || !strings.Contains(dateOnly, `name="reminder_unit" type="hidden" value="days"`) || !strings.Contains(dateOnly, `name="reminder_at"`) {
		t.Errorf("date-only Task frame should count days, with no unit picker, at a time of day:\n%s", dateOnly)
	}

	page = editorPage(t, "/events/5", `{"id":5,"title":"Dentist","reminder_times":[{"minutes":-60}]}`)
	appointment := reminderFramesIn(t, page, "event-edit", "timed")[0]
	if got, want := pills(appointment), []string{"before=Before", "at=At the start"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("Appointment pills = %q, want %q", got, want)
	}
	if got, want := atOption(appointment), "at the start"; got != want {
		t.Errorf("Appointment select says %q for at, want %q", got, want)
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

// reminderFrameShape reads a frame as "summary | controls", each control it
// has marked "+" when it applies to the row or "-" when it doesn't: the
// amount, and the time of day, with "required" when it must be filled in.
func reminderFrameShape(t *testing.T, frame string) string {
	t.Helper()
	text := ""
	if m := regexp.MustCompile(`<span class="rt-summary"[^>]*>([^<]*)</span>`).FindStringSubmatch(frame); m != nil {
		text = m[1]
	}
	marks := []string{}
	for _, control := range []struct{ name, marker string }{{"amount", "data-reminder-amount"}, {"at", "data-reminder-at"}} {
		tag := regexp.MustCompile(`<[^>]*` + control.marker + `[^>]*>`).FindString(frame)
		if tag == "" {
			continue
		}
		applies := regexp.MustCompile(`data-applies="(\w+)"`).FindStringSubmatch(tag)
		switch {
		case applies == nil:
			t.Fatalf("%s control has no applies marker: %s", control.name, tag)
		case applies[1] == "true":
			marks = append(marks, "+"+control.name)
		default:
			marks = append(marks, "-"+control.name)
		}
	}
	if regexp.MustCompile(`<input name="reminder_at"[^>]* required`).MatchString(frame) {
		marks = append(marks, "required")
	}
	return text + " | " + strings.Join(marks, " ")
}

// reminderShapesIn reads each frame of the set for kind on the form with
// the given id as reminderFrameShape does.
func reminderShapesIn(t *testing.T, page, id, kind string) []string {
	t.Helper()
	shapes := []string{}
	for _, frame := range reminderFramesIn(t, page, id, kind) {
		shapes = append(shapes, reminderFrameShape(t, frame))
	}
	return shapes
}

func TestReminderTimeFramesMarkWhichControlsApplyToTheirShape(t *testing.T) {
	page := editorPage(t, "/tasks/7", `{"id":7,"title":"Mow","due_at":"2030-01-20T18:00:00Z","reminder_times":[{"minutes":-60},{"minutes":0},{"days":0,"at":"08:00"},{"days":-1,"at":"08:00"},{"minutes":-1440}]}`)
	want := []string{
		"1 hour before | +amount -at",
		"At the due time | -amount +at",
		"On the due date at 08:00 | -amount +at",
		"1 day before at 08:00 | +amount +at",
		"1 day before | +amount +at",
	}
	if got := reminderShapesIn(t, page, "task-edit", "timed"); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("timed Task frames = %q, want %q", got, want)
	}

	page = editorPage(t, "/tasks/7", `{"id":7,"title":"Mow","due_at":"2030-01-20T06:00:00Z","date_only":true,"reminder_times":[{"days":-1,"at":"18:00"},{"days":0,"at":"08:00"},{"days":2,"at":"07:30"}]}`)
	want = []string{
		"1 day before at 18:00 | +amount +at required",
		"On the due date at 08:00 | -amount +at required",
		"2 days after at 07:30 | +amount +at required",
	}
	if got := reminderShapesIn(t, page, "task-edit", "date_only"); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("date-only Task frames = %q, want %q", got, want)
	}

	page = editorPage(t, "/events/5", `{"id":5,"title":"Dentist","reminder_times":[{"minutes":-60},{"minutes":0}]}`)
	want = []string{"1 hour before the start | +amount", "At the start | -amount"}
	if got := reminderShapesIn(t, page, "event-edit", "timed"); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("Appointment frames = %q, want %q", got, want)
	}

	page = editorPage(t, "/routines/8", `{"id":8,"title":"Take the bins out","cadence":"weekly","next_due_at":"2030-01-20T19:00:00Z","reminder_times":[{"minutes":-120},{"days":0,"at":"07:00"}]}`)
	want = []string{"2 hours before | +amount -at", "On the due date at 07:00 | -amount +at"}
	if got := reminderShapesIn(t, page, "routine-edit", "timed"); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("timed Routine frames = %q, want %q", got, want)
	}
}

func TestAddFormsMarkWhichControlsApplyToEachReminderTime(t *testing.T) {
	_, web := newRecordingAPI(t, map[string]cannedResponse{
		"GET /api/v1/dashboard":               {http.StatusOK, `{"current_user":{"id":1,"name":"Owner"}}`},
		"GET /api/v1/reminder-times/defaults": {http.StatusOK, `{"timed_task":[{"minutes":-90},{"minutes":0},{"days":0,"at":"08:00"}],"date_only_task":[{"days":-2,"at":"07:30"},{"days":0,"at":"07:30"}],"appointment":[{"minutes":-60},{"minutes":0}]}`},
	})
	page := serve(web.routes(), browserRequest(http.MethodGet, "/", "")).Body.String()
	// addedFrame is the frame "Add reminder" adds to the set for kind.
	addedFrame := func(id, kind string) string {
		form := regexp.MustCompile(`(?s)id="` + id + `".*?</form>`).FindString(page)
		set := regexp.MustCompile(`(?s)data-reminder-kind="` + kind + `".*?</fieldset>`).FindString(form)
		m := regexp.MustCompile(`(?s)<template data-reminder-new>(.*?)</template>`).FindStringSubmatch(set)
		if m == nil {
			t.Fatalf("#%s %s set has no frame to add:\n%s", id, kind, set)
		}
		return m[1]
	}

	for _, c := range []struct {
		id, kind string
		want     []string
		added    string
	}{
		{"add-task-form", "timed", []string{"90 minutes before | +amount -at", "At the due time | -amount +at", "On the due date at 08:00 | -amount +at"}, "1 hour before | +amount -at"},
		{"add-task-form", "date_only", []string{"2 days before at 07:30 | +amount +at required", "On the due date at 07:30 | -amount +at required"}, "1 day before at 08:00 | +amount +at required"},
		{"add-appointment-form", "timed", []string{"1 hour before the start | +amount", "At the start | -amount"}, "1 hour before the start | +amount"},
	} {
		if got := reminderShapesIn(t, page, c.id, c.kind); fmt.Sprint(got) != fmt.Sprint(c.want) {
			t.Errorf("#%s %s frames = %q, want %q", c.id, c.kind, got, c.want)
		}
		if got := reminderFrameShape(t, addedFrame(c.id, c.kind)); got != c.added {
			t.Errorf("#%s %s added frame = %q, want %q", c.id, c.kind, got, c.added)
		}
	}
}

func TestControlsThatDoNotApplyAreHiddenOnlyWhileTheScriptRuns(t *testing.T) {
	page := editorPage(t, "/tasks/7", `{"id":7,"title":"Mow","due_at":"2030-01-20T18:00:00Z","reminder_times":[{"minutes":-60}]}`)
	style := regexp.MustCompile(`(?s)<style>.*?</style>`).FindString(page)
	rules := regexp.MustCompile(`[^{}]*\[data-applies="false"\][^{}]*\{[^}]*\}`).FindAllString(style, -1)
	if len(rules) == 0 {
		t.Fatalf("stylesheet never hides a control that doesn't apply:\n%s", style)
	}
	for _, rule := range rules {
		for _, selector := range strings.Split(rule[:strings.Index(rule, "{")], ",") {
			if strings.Contains(selector, `[data-applies="false"]`) && !strings.HasPrefix(strings.TrimSpace(selector), ".live ") {
				t.Errorf("selector %q hides a control that doesn't apply without the script, want it under .live", strings.TrimSpace(selector))
			}
		}
	}
}

// rowRouteFrame gets one Reminder Time frame from the web server's
// row route with the given query, failing unless it is served without
// calling the API.
func rowRouteFrame(t *testing.T, query url.Values) string {
	t.Helper()
	api, web := newRecordingAPI(t, map[string]cannedResponse{})
	rec := serve(web.routes(), browserRequest(http.MethodGet, "/reminder-times/row?"+query.Encode(), ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET row %s = %d, want 200:\n%s", query.Encode(), rec.Code, rec.Body)
	}
	if calls := api.Calls(); len(calls) != 0 {
		t.Errorf("GET row %s called the API: %+v", query.Encode(), calls)
	}
	return rec.Body.String()
}

// rowQuery is the row route's query for a row in a set of kind, of an
// Appointment or not, from its fields "amount unit direction at".
func rowQuery(kind string, appointment bool, row [4]string) url.Values {
	q := url.Values{"reminder_kind": {kind}, "reminder_amount": {row[0]}, "reminder_unit": {row[1]}, "reminder_direction": {row[2]}, "reminder_at": {row[3]}}
	if appointment {
		q.Set("appointment", "1")
	}
	return q
}

func TestReminderTimeRowRouteRendersTheFrameThePageDoes(t *testing.T) {
	trim := func(frame string) string {
		return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(frame), `<div class="rt-frame"`))
	}
	for _, c := range []struct {
		path, record, id, kind string
		appointment            bool
		row                    [4]string
	}{
		{"/tasks/7", `{"id":7,"title":"Mow","due_at":"2030-01-20T18:00:00Z","reminder_times":[{"days":0,"at":"08:00"},{"minutes":-60}]}`, "task-edit", "timed", false, [4]string{"0", "days", "at", "08:00"}},
		{"/tasks/7", `{"id":7,"title":"Mow","due_at":"2030-01-20T18:00:00Z","reminder_times":[{"minutes":-120},{"minutes":-60}]}`, "task-edit", "timed", false, [4]string{"2", "hours", "before", ""}},
		{"/tasks/7", `{"id":7,"title":"Mow","due_at":"2030-01-20T06:00:00Z","date_only":true,"reminder_times":[{"days":-1,"at":"18:00"},{"days":0,"at":"08:00"}]}`, "task-edit", "date_only", false, [4]string{"1", "days", "before", "18:00"}},
		{"/events/5", `{"id":5,"title":"Dentist","reminder_times":[{"minutes":0},{"minutes":-60}]}`, "event-edit", "timed", true, [4]string{"0", "minutes", "at", ""}},
	} {
		page := editorPage(t, c.path, c.record)
		want := trim(reminderFramesIn(t, page, c.id, c.kind)[0])
		if got := trim(rowRouteFrame(t, rowQuery(c.kind, c.appointment, c.row))); got != want {
			t.Errorf("row %s %v =\n%s\nwant the %s page's frame\n%s", c.kind, c.row, got, c.path, want)
		}
	}
}

func TestReminderTimeRowRouteShowsARowAsItIsNowShaped(t *testing.T) {
	for _, c := range []struct {
		kind        string
		appointment bool
		row         [4]string
		want        string
	}{
		// Switched from days to hours: the time of day is kept but doesn't
		// apply, so it's left out of the summary.
		{"timed", false, [4]string{"1", "hours", "before", "08:00"}, "1 hour before | +amount -at"},
		{"timed", false, [4]string{"1", "days", "before", "08:00"}, "1 day before at 08:00 | +amount +at"},
		{"timed", false, [4]string{"3", "days", "at", "08:00"}, "On the due date at 08:00 | -amount +at"},
		{"timed", false, [4]string{"3", "hours", "at", ""}, "At the due time | -amount +at"},
		{"date_only", false, [4]string{"2", "days", "after", "07:30"}, "2 days after at 07:30 | +amount +at required"},
		{"date_only", false, [4]string{"0", "days", "at", "07:30"}, "On the due date at 07:30 | -amount +at required"},
		{"timed", true, [4]string{"2", "hours", "before", ""}, "2 hours before the start | +amount"},
		{"timed", true, [4]string{"0", "minutes", "at", ""}, "At the start | -amount"},
		// An Appointment's Reminder Times are never after the start.
		{"timed", true, [4]string{"2", "hours", "after", ""}, "2 hours before the start | +amount"},
		// A unit or direction the set doesn't offer is a new row's.
		{"timed", false, [4]string{"1", "weeks", "sideways", ""}, "1 hour before | +amount -at"},
	} {
		frame := rowRouteFrame(t, rowQuery(c.kind, c.appointment, c.row))
		if got := reminderFrameShape(t, frame); got != c.want {
			t.Errorf("row %s appointment=%v %v = %q, want %q", c.kind, c.appointment, c.row, got, c.want)
		}
		if !c.appointment {
			if got := reminderFieldValue(frame, "reminder_at"); got != c.row[3] {
				t.Errorf("row %s %v keeps time of day %q, want %q", c.kind, c.row, got, c.row[3])
			}
		}
	}
}

func TestReminderTimeDirectionPillsSayWhichIsPicked(t *testing.T) {
	for _, c := range []struct {
		kind        string
		appointment bool
		direction   string
		want        string
	}{
		{"timed", false, "before", "before=true at=false after=false"},
		{"timed", false, "at", "before=false at=true after=false"},
		{"date_only", false, "after", "before=false at=false after=true"},
		{"timed", true, "at", "before=false at=true"},
		{"timed", true, "after", "before=true at=false"},
	} {
		frame := rowRouteFrame(t, rowQuery(c.kind, c.appointment, [4]string{"1", "days", c.direction, "08:00"}))
		pills := []string{}
		for _, m := range regexp.MustCompile(`<button [^>]*data-reminder-pick="(\w+)"[^>]*>`).FindAllStringSubmatch(frame, -1) {
			pressed := regexp.MustCompile(`aria-pressed="(\w+)"`).FindStringSubmatch(m[0])
			if pressed == nil {
				pressed = []string{"", "missing"}
			}
			pills = append(pills, m[1]+"="+pressed[1])
		}
		if got := strings.Join(pills, " "); got != c.want {
			t.Errorf("%s appointment=%v %s pills = %q, want %q", c.kind, c.appointment, c.direction, got, c.want)
		}
	}
}

// The page's script swaps in frames the server renders: it never words a
// summary itself, nor decides which controls apply to a row (ADR-0005).
func TestReminderTimesEditorScriptLeavesWordingAndControlsToTheServer(t *testing.T) {
	page := editorPage(t, "/tasks/7", `{"id":7,"title":"Mow","due_at":"2030-01-20T18:00:00Z","reminder_times":[{"days":0,"at":"08:00"}]}`)
	scripts := strings.Join(regexp.MustCompile(`(?s)<script[^>]*>(.*?)</script>`).FindAllString(page, -1), "\n")
	if !strings.Contains(scripts, "/reminder-times/row") {
		t.Fatalf("page script never asks the row route for a frame")
	}
	for _, wording := range []string{"At the due time", "On the due date", "At the start", "the start", "data-applies", "data-reminder-summary", "data-reminder-amount", "data-reminder-at"} {
		if strings.Contains(scripts, wording) {
			t.Errorf("page script contains %q, want the server alone to word frames and say which controls apply", wording)
		}
	}
}

// A blank amount drops a row when the form is saved, so a row picked to be
// on the due date (or at the start) before any amount was typed comes back
// with 0, and is kept.
func TestPickingTheDueDateForARowWithNoAmountGivesItZero(t *testing.T) {
	for _, c := range []struct {
		kind        string
		appointment bool
		row         [4]string
		amount      string
	}{
		{"timed", false, [4]string{"", "hours", "at", "08:00"}, "0"},
		{"date_only", false, [4]string{"", "days", "at", "08:00"}, "0"},
		{"timed", true, [4]string{"", "minutes", "at", ""}, "0"},
		{"timed", false, [4]string{"", "hours", "before", ""}, ""},
	} {
		frame := rowRouteFrame(t, rowQuery(c.kind, c.appointment, c.row))
		if got := reminderFieldValue(frame, "reminder_amount"); got != c.amount {
			t.Errorf("row %s appointment=%v %v has amount %q, want %q", c.kind, c.appointment, c.row, got, c.amount)
		}
	}
}

// A blank time of day means at the due time only for a row on the due date
// of a timed set; on a row that counts days it would mean no time of day.
func TestBlankTimeOfDayHintAppliesOnlyOnTheDueDate(t *testing.T) {
	for _, c := range []struct {
		kind string
		row  [4]string
		want string
	}{
		{"timed", [4]string{"0", "hours", "at", ""}, `data-applies="true"`},
		{"timed", [4]string{"1", "days", "before", "08:00"}, `data-applies="false"`},
		{"timed", [4]string{"1", "days", "after", ""}, `data-applies="false"`},
		{"date_only", [4]string{"0", "days", "at", "08:00"}, "no hint"},
	} {
		frame := rowRouteFrame(t, rowQuery(c.kind, false, c.row))
		got := "no hint"
		if hint := regexp.MustCompile(`<span class="hint"[^>]*>blank = at the due time`).FindString(frame); hint != "" {
			got = "no marker"
			if m := regexp.MustCompile(`data-applies="\w+"`).FindString(hint); m != "" {
				got = m
			}
		}
		if got != c.want {
			t.Errorf("row %s %v hint = %s, want %s", c.kind, c.row, got, c.want)
		}
	}
}

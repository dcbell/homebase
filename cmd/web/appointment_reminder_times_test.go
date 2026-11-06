package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// reminderSlotsIn returns the Reminder Time rows of the form inside the
// element with the given id, as "amount unit" strings, empty rows included.
func reminderSlotsIn(t *testing.T, page, id string) []string {
	t.Helper()
	form := regexp.MustCompile(`(?s)id="` + id + `".*?</form>`).FindString(page)
	if form == "" {
		t.Fatalf("page has no #%s form:\n%s", id, page)
	}
	slots := []string{}
	row := regexp.MustCompile(`(?s)name="reminder_before"[^>]*value="(\d*)".*?<select name="reminder_unit"[^>]*>(.*?)</select>`)
	selected := regexp.MustCompile(`<option value="(\w+)" selected>`)
	for _, m := range row.FindAllStringSubmatch(form, -1) {
		unit := ""
		if s := selected.FindStringSubmatch(m[2]); s != nil {
			unit = s[1]
		}
		slots = append(slots, strings.TrimSpace(m[1]+" "+unit))
	}
	return slots
}

// sentReminderTimes returns the reminder_times a forwarded body carried, as
// JSON, or "absent".
func sentReminderTimes(t *testing.T, body string) string {
	t.Helper()
	var sent map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &sent); err != nil {
		t.Fatalf("forwarded body %q: %v", body, err)
	}
	raw, ok := sent["reminder_times"]
	if !ok {
		return "absent"
	}
	return string(raw)
}

func TestAddAppointmentFormShowsTheAPIsDefaultReminderTimes(t *testing.T) {
	_, web := newRecordingAPI(t, map[string]cannedResponse{
		"GET /api/v1/dashboard":               {http.StatusOK, `{"current_user":{"id":1,"name":"Owner"}}`},
		"GET /api/v1/reminder-times/defaults": {http.StatusOK, `{"appointment":[{"minutes":-2880},{"minutes":-180},{"minutes":-45},{"minutes":0}]}`},
	})

	page := serve(web.routes(), browserRequest(http.MethodGet, "/", "")).Body.String()

	got := reminderSlotsIn(t, page, "add-appointment-form")
	if want := []string{"2 days", "3 hours", "45 minutes", "0 minutes", ""}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("Reminder Time rows = %q, want %q", got, want)
	}
}

func TestAppointmentEditFormShowsItsReminderTimes(t *testing.T) {
	_, web := newRecordingAPI(t, map[string]cannedResponse{
		"GET /api/v1/dashboard": {http.StatusOK, `{"current_user":{"id":1,"name":"Owner"}}`},
		"GET /api/v1/events/5":  {http.StatusOK, `{"id":5,"title":"Dentist","reminder_times":[{"minutes":-90}]}`},
	})

	page := serve(web.routes(), browserRequest(http.MethodGet, "/events/5", "")).Body.String()

	got := reminderSlotsIn(t, page, "event-edit")
	if want := []string{"90 minutes", "", "", "", ""}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("Reminder Time rows = %q, want %q", got, want)
	}
}

func TestAppointmentFormsSendReminderTimesToTheAPI(t *testing.T) {
	api, web := newRecordingAPI(t, map[string]cannedResponse{
		"POST /api/v1/events":    {http.StatusCreated, `{"id":5}`},
		"PATCH /api/v1/events/5": {http.StatusOK, `{"id":5}`},
	})
	form := url.Values{
		"title": {"Dentist"}, "starts_at": {"2030-01-20T09:00"}, "ends_at": {"2030-01-20T10:00"},
		"reminder_times":  {"1"},
		"reminder_before": {"2", "", "30", "1", "0"},
		"reminder_unit":   {"hours", "minutes", "minutes", "days", "minutes"},
	}
	post := func(path string) {
		req := browserRequest(http.MethodPost, path, form.Encode())
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		serve(web.routes(), req)
	}

	post("/events")
	form["reminder_before"] = []string{"", "", "", "", ""}
	post("/events/5")

	calls := api.Calls()
	if len(calls) != 2 {
		t.Fatalf("API calls = %+v, want a create and an edit", calls)
	}
	if got, want := sentReminderTimes(t, calls[0].Body), `[{"minutes":-120},{"minutes":-30},{"minutes":-1440},{"minutes":0}]`; got != want {
		t.Errorf("create sent reminder_times %s, want %s", got, want)
	}
	if got := sentReminderTimes(t, calls[1].Body); got != `[]` {
		t.Errorf("edit with every row cleared sent reminder_times %s, want [] for no Reminders", got)
	}
}

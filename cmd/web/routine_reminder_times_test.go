package main

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
)

func TestRoutineAndMaintenanceFormsShowTheirReminderTimes(t *testing.T) {
	_, web := newRecordingAPI(t, map[string]cannedResponse{
		"GET /api/v1/dashboard":               {http.StatusOK, `{"current_user":{"id":1,"name":"Owner"}}`},
		"GET /api/v1/reminder-times/defaults": {http.StatusOK, taskReminderDefaults},
		"GET /api/v1/routines/5":              {http.StatusOK, `{"id":5,"title":"Water the plants","cadence":"weekly","reminder_times":[{"days":-2,"at":"19:00"}]}`},
		"GET /api/v1/assets/4":                {http.StatusOK, `{"id":4,"name":"Boiler"}`},
		"GET /api/v1/assets/4/maintenance":    {http.StatusOK, `[{"id":6,"asset_id":4,"title":"Service the boiler","cadence":"yearly","reminder_times":[]}]`},
		"GET /api/v1/documents":               {http.StatusOK, `[]`},
		"GET /api/v1/contacts":                {http.StatusOK, `[]`},
		"GET /api/v1/related-documents":       {http.StatusOK, `[]`},
		"GET /api/v1/related-contacts":        {http.StatusOK, `[]`},
	})
	defaults := map[string][]string{"date_only": {"2 days before 07:30", "days at 07:30", "", "", ""}}

	// Routines and Maintenance Schedules are due on a date, so their create
	// forms show only the date-only defaults.
	for _, form := range []struct{ path, id string }{
		{"/", "add-routine"},
		{"/routines", "add-routine-index"},
		{"/assets/4", "add-asset-maintenance"},
	} {
		page := serve(web.routes(), browserRequest(http.MethodGet, form.path, "")).Body.String()
		sets, shown := taskReminderSetsIn(t, page, form.id)
		if fmt.Sprint(sets) != fmt.Sprint(defaults) || shown != "date_only" {
			t.Errorf("%s #%s Reminder Times = %q, showing %q; want only date-only %q", form.path, form.id, sets, shown, defaults)
		}
	}

	// Edit forms show each one's own.
	page := serve(web.routes(), browserRequest(http.MethodGet, "/routines/5", "")).Body.String()
	sets, shown := taskReminderSetsIn(t, page, "routine-edit")
	if want := map[string][]string{"date_only": {"2 days before 19:00", "", "", "", ""}}; fmt.Sprint(sets) != fmt.Sprint(want) || shown != "date_only" {
		t.Errorf("Routine edit Reminder Times = %q, showing %q; want %q", sets, shown, want)
	}
	page = serve(web.routes(), browserRequest(http.MethodGet, "/assets/4", "")).Body.String()
	sets, shown = taskReminderSetsIn(t, page, "edit-asset-maintenance-6")
	if want := map[string][]string{"date_only": {"", "", "", "", ""}}; fmt.Sprint(sets) != fmt.Sprint(want) || shown != "date_only" {
		t.Errorf("Maintenance Schedule edit Reminder Times = %q, showing %q; want %q", sets, shown, want)
	}
}

func TestRoutineAndMaintenanceFormsSendReminderTimesToTheAPI(t *testing.T) {
	api, web := newRecordingAPI(t, map[string]cannedResponse{
		"POST /api/v1/routines":             {http.StatusCreated, `{"id":5}`},
		"PATCH /api/v1/routines/5":          {http.StatusOK, `{"id":5}`},
		"POST /api/v1/assets/4/maintenance": {http.StatusCreated, `{"id":6}`},
		"PATCH /api/v1/asset-maintenance/6": {http.StatusOK, `{"id":6}`},
	})
	form := url.Values{
		"title": {"Water the plants"}, "next_due_at": {"2030-01-20"},
		"reminder_times":     {"1"},
		"reminder_amount":    {"2", "", "1", "", ""},
		"reminder_unit":      {"days", "days", "days", "days", "days"},
		"reminder_direction": {"before", "at", "after", "before", "before"},
		"reminder_at":        {"19:00", "08:00", "09:00", "", ""},
	}
	post := func(path string, form url.Values) {
		req := browserRequest(http.MethodPost, path, form.Encode())
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		serve(web.routes(), req)
	}

	post("/routines", form)
	post("/routines/5", form)
	post("/assets/4/maintenance", form)
	post("/assets/4/maintenance/6", form)
	// A form without the Reminder Times editor leaves them to the API to keep.
	post("/routines/5", url.Values{"title": {"Water the plants"}})

	calls := api.Calls()
	if len(calls) != 5 {
		t.Fatalf("API calls = %+v, want four saves with Reminder Times and one without", calls)
	}
	want := `[{"days":-2,"at":"19:00"},{"days":0,"at":"08:00"},{"days":1,"at":"09:00"}]`
	for _, call := range calls[:4] {
		if got := sentReminderTimes(t, call.Body); got != want {
			t.Errorf("%s %s sent reminder_times %s, want %s", call.Method, call.Path, got, want)
		}
	}
	if got := sentReminderTimes(t, calls[4].Body); got != "absent" {
		t.Errorf("save without the editor sent reminder_times %s, want them left out", got)
	}
}

package main

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"testing"
)

var settingsAPI = map[string]cannedResponse{
	"GET /api/v1/dashboard":          {http.StatusOK, `{"current_user":{"id":1,"email":"member@example.test","name":"Member"}}`},
	"GET /api/v1/api-tokens":         {http.StatusOK, `[]`},
	"GET /api/v1/push-subscriptions": {http.StatusOK, `[]`},
	"GET /api/v1/push-settings":      {http.StatusOK, `{"assignment":true,"task_reminder":true,"appointment_reminder":true}`},
	"GET /api/v1/calendar-connections": {http.StatusOK, `[
		{"id":4,"name":"Thunderbird","kind":"other","secret_prefix":"hbcal_abc","created_at":"2030-01-10T09:00:00Z","last_used_at":"2030-01-14T18:30:00Z"},
		{"id":3,"name":"Old tablet","kind":"other","secret_prefix":"hbcal_def","created_at":"2029-12-01T09:00:00Z"},
		{"id":2,"name":"Sold phone","kind":"other","secret_prefix":"hbcal_ghi","created_at":"2029-11-01T09:00:00Z","revoked_at":"2029-12-24T09:00:00Z"}
	]`},
}

func withResponses(base map[string]cannedResponse, extra map[string]cannedResponse) map[string]cannedResponse {
	all := map[string]cannedResponse{}
	for k, v := range base {
		all[k] = v
	}
	for k, v := range extra {
		all[k] = v
	}
	return all
}

// calendarSection returns the Settings page's Calendar section.
func calendarSection(t *testing.T, page string) string {
	t.Helper()
	section := regexp.MustCompile(`(?s)<section[^>]*data-calendar-connections[^>]*>.*?</section>`).FindString(page)
	if section == "" {
		t.Fatalf("Settings page has no Calendar section:\n%s", page)
	}
	return section
}

func TestSettingsListsCalendarConnections(t *testing.T) {
	_, web := newRecordingAPI(t, settingsAPI)

	rec := serve(web.routes(), browserRequest(http.MethodGet, "/settings", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	section := calendarSection(t, rec.Body.String())
	for _, want := range []string{"Thunderbird", "Old tablet", "Sold phone", "Other device", "Not used yet", `action="/settings/calendar-connections"`} {
		if !strings.Contains(section, want) {
			t.Errorf("Calendar section has no %q:\n%s", want, section)
		}
	}
	if !strings.Contains(section, `action="/settings/calendar-connections/4/disconnect"`) || !strings.Contains(section, `action="/settings/calendar-connections/3/disconnect"`) {
		t.Errorf("active connections have no Disconnect button:\n%s", section)
	}
	if strings.Contains(section, `/settings/calendar-connections/2/disconnect`) {
		t.Errorf("a disconnected connection still has a Disconnect button")
	}
}

func TestConnectingAnOtherDeviceForwardsToTheAPIAndShowsTheDetailsOnce(t *testing.T) {
	api, web := newRecordingAPI(t, withResponses(settingsAPI, map[string]cannedResponse{
		"POST /api/v1/calendar-connections": {http.StatusCreated, `{
			"connection":{"id":5,"name":"Pixel","kind":"other","created_at":"2030-01-15T12:00:00Z"},
			"server_url":"https://homebase.example.test/caldav/",
			"username":"member@example.test",
			"secret":"hbcal_supersecretvalue"
		}`},
	}))
	req := browserRequest(http.MethodPost, "/settings/calendar-connections", "name=Pixel")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := serve(web.routes(), req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var created *apiCall
	for _, call := range api.Calls() {
		if call.Method == http.MethodPost && call.Path == "/api/v1/calendar-connections" {
			created = &call
		}
	}
	if created == nil {
		t.Fatalf("API calls = %+v, want a POST /api/v1/calendar-connections", api.Calls())
	}
	var sent map[string]string
	if err := json.Unmarshal([]byte(created.Body), &sent); err != nil || sent["name"] != "Pixel" || sent["kind"] != "other" {
		t.Errorf("POST body = %s, want name Pixel and kind other", created.Body)
	}
	section := calendarSection(t, rec.Body.String())
	for _, want := range []string{"https://homebase.example.test/caldav/", "member@example.test", "hbcal_supersecretvalue"} {
		if !strings.Contains(section, want) {
			t.Errorf("Calendar section doesn't show %q:\n%s", want, section)
		}
	}

	if !strings.Contains(rec.Body.String(), savedToast) {
		t.Errorf("page has no %s", savedToast)
	}

	again := serve(web.routes(), browserRequest(http.MethodGet, "/settings", ""))
	if strings.Contains(again.Body.String(), "hbcal_supersecretvalue") {
		t.Errorf("the secret is shown again on the next visit")
	}
}

func TestAFailedConnectionShowsTheAPIError(t *testing.T) {
	_, web := newRecordingAPI(t, withResponses(settingsAPI, map[string]cannedResponse{
		"POST /api/v1/calendar-connections": {http.StatusBadRequest, `{"error":"name is required"}`},
	}))
	req := browserRequest(http.MethodPost, "/settings/calendar-connections", "name=")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := serve(web.routes(), req)

	if !strings.Contains(rec.Body.String(), "name is required") {
		t.Errorf("page doesn't show the API error:\n%s", rec.Body.String())
	}
}

func TestDisconnectingACalendarConnectionReturnsToSettingsWithSaved(t *testing.T) {
	api, web := newRecordingAPI(t, map[string]cannedResponse{
		"DELETE /api/v1/calendar-connections/4": {http.StatusOK, `{"ok":true}`},
	})

	rec := serve(web.routes(), browserRequest(http.MethodPost, "/settings/calendar-connections/4/disconnect", ""))

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/settings?saved=1#calendar" {
		t.Errorf("got %d to %q, want a redirect to /settings?saved=1#calendar", rec.Code, rec.Header().Get("Location"))
	}
	calls := api.Calls()
	if len(calls) != 1 || calls[0].Method != http.MethodDelete || calls[0].Path != "/api/v1/calendar-connections/4" {
		t.Errorf("API calls = %+v, want one DELETE /api/v1/calendar-connections/4", calls)
	}
}

func TestSettingsOffersIPhoneSetupWithTheRefreshAndRepeatingEventCaveats(t *testing.T) {
	_, web := newRecordingAPI(t, withResponses(settingsAPI, map[string]cannedResponse{
		"GET /api/v1/calendar-connections": {http.StatusOK, `[
			{"id":6,"name":"Dave's iPhone","kind":"ios","secret_prefix":"hbcal_xyz","created_at":"2030-01-10T09:00:00Z"}
		]`},
	}))

	rec := serve(web.routes(), browserRequest(http.MethodGet, "/settings", ""))

	section := calendarSection(t, rec.Body.String())
	for _, want := range []string{"Set up on iPhone", `name="kind" value="ios"`, "15 minutes", "repeating events", "Routines", "Dave&#39;s iPhone", "iPhone · Created"} {
		if !strings.Contains(section, want) {
			t.Errorf("Calendar section has no %q:\n%s", want, section)
		}
	}
}

func TestSettingUpAnIPhoneForwardsToTheAPIAndSendsTheBrowserToTheProfile(t *testing.T) {
	api, web := newRecordingAPI(t, withResponses(settingsAPI, map[string]cannedResponse{
		"POST /api/v1/calendar-connections": {http.StatusCreated, `{
			"connection":{"id":7,"name":"Dave's iPhone","kind":"ios","created_at":"2030-01-15T12:00:00Z"},
			"profile_url":"https://homebase.example.test/api/v1/calendar-profiles/onetimetoken"
		}`},
	}))
	req := browserRequest(http.MethodPost, "/settings/calendar-connections", "name=Dave%27s+iPhone&kind=ios")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := serve(web.routes(), req)

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "https://homebase.example.test/api/v1/calendar-profiles/onetimetoken" {
		t.Errorf("got %d to %q, want a 303 to the profile link", rec.Code, rec.Header().Get("Location"))
	}
	var sent map[string]string
	for _, call := range api.Calls() {
		if call.Method == http.MethodPost && call.Path == "/api/v1/calendar-connections" {
			_ = json.Unmarshal([]byte(call.Body), &sent)
		}
	}
	if sent["name"] != "Dave's iPhone" || sent["kind"] != "ios" {
		t.Errorf("POST body = %v, want name Dave's iPhone and kind ios", sent)
	}
}

func TestAFailedIPhoneSetupShowsTheAPIError(t *testing.T) {
	_, web := newRecordingAPI(t, withResponses(settingsAPI, map[string]cannedResponse{
		"POST /api/v1/calendar-connections": {http.StatusBadRequest, `{"error":"name is required"}`},
	}))
	req := browserRequest(http.MethodPost, "/settings/calendar-connections", "name=&kind=ios")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := serve(web.routes(), req)

	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "name is required") {
		t.Errorf("got %d, want the Settings page with the API error:\n%s", rec.Code, rec.Body.String())
	}
}

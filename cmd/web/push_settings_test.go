package main

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"testing"
)

func TestSettingsShowsAPushSwitchPerKind(t *testing.T) {
	_, web := newRecordingAPI(t, map[string]cannedResponse{
		"GET /api/v1/dashboard":          {http.StatusOK, `{"current_user":{"id":1,"email":"member@example.test","name":"Member"}}`},
		"GET /api/v1/api-tokens":         {http.StatusOK, `[]`},
		"GET /api/v1/push-subscriptions": {http.StatusOK, `[]`},
		"GET /api/v1/push-settings":      {http.StatusOK, `{"assignment":true,"task_reminder":false,"appointment_reminder":true}`},
	})

	rec := serve(web.routes(), browserRequest(http.MethodGet, "/settings", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	page := rec.Body.String()
	if !strings.Contains(page, `action="/settings/push-settings"`) {
		t.Errorf("Settings page has no Push settings form")
	}
	for kind, checked := range map[string]bool{"assignment": true, "task_reminder": false, "appointment_reminder": true} {
		input := regexp.MustCompile(`<input[^>]*name="` + kind + `"[^>]*>`).FindString(page)
		if input == "" {
			t.Errorf("Settings page has no %s switch", kind)
			continue
		}
		if strings.Contains(input, "checked") != checked {
			t.Errorf("%s switch = %s, want checked=%v", kind, input, checked)
		}
	}
}

func TestSavingPushSettingsSendsEveryKindAndReturnsToSettings(t *testing.T) {
	api, web := newRecordingAPI(t, map[string]cannedResponse{
		"PATCH /api/v1/push-settings": {http.StatusOK, `{"assignment":true,"task_reminder":false,"appointment_reminder":true}`},
	})
	req := browserRequest(http.MethodPost, "/settings/push-settings", "assignment=on&appointment_reminder=on")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := serve(web.routes(), req)

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/settings?saved=1" {
		t.Errorf("got %d to %q, want a redirect to /settings?saved=1", rec.Code, rec.Header().Get("Location"))
	}
	calls := api.Calls()
	if len(calls) != 1 || calls[0].Method != http.MethodPatch || calls[0].Path != "/api/v1/push-settings" {
		t.Fatalf("API calls = %+v, want one PATCH /api/v1/push-settings", calls)
	}
	var sent map[string]bool
	if err := json.Unmarshal([]byte(calls[0].Body), &sent); err != nil {
		t.Fatalf("PATCH body %q: %v", calls[0].Body, err)
	}
	want := map[string]bool{"assignment": true, "task_reminder": false, "appointment_reminder": true}
	if len(sent) != len(want) || sent["assignment"] != true || sent["task_reminder"] != false || sent["appointment_reminder"] != true {
		t.Errorf("PATCH body = %v, want %v", sent, want)
	}
}

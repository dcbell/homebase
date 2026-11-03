package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"homebase/internal/config"
)

// apiCall is one request the fake API received.
type apiCall struct {
	Method    string
	Path      string
	Body      string
	Cookie    string
	UserAgent string
}

// fakeAPI answers with canned responses keyed by "METHOD /path" and records
// every call.
type fakeAPI struct {
	mu        sync.Mutex
	calls     []apiCall
	responses map[string]fakeResponse
}

type fakeResponse struct {
	status int
	body   string
}

func newFakeAPI(t *testing.T, responses map[string]fakeResponse) (*fakeAPI, *webServer) {
	t.Helper()
	api := &fakeAPI{responses: responses}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		cookie := ""
		if c, err := r.Cookie("homebase_session"); err == nil {
			cookie = c.Value
		}
		api.mu.Lock()
		api.calls = append(api.calls, apiCall{Method: r.Method, Path: r.URL.RequestURI(), Body: string(body), Cookie: cookie, UserAgent: r.UserAgent()})
		resp, ok := api.responses[r.Method+" "+r.URL.Path]
		api.mu.Unlock()
		if !ok {
			resp = fakeResponse{http.StatusNotFound, `{"error":"route not found"}`}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.status)
		_, _ = io.WriteString(w, resp.body)
	}))
	t.Cleanup(srv.Close)
	web := newWebServer(config.Config{APIInternalURL: srv.URL, SessionCookieName: "homebase_session"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return api, web
}

func (a *fakeAPI) Calls() []apiCall {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]apiCall(nil), a.calls...)
}

func browserRequest(method, path, body string) *http.Request {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.AddCookie(&http.Cookie{Name: "homebase_session", Value: "session-123"})
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) Safari/604.1")
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

func serve(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestVAPIDPublicKeyIsForwardedFromTheAPI(t *testing.T) {
	api, web := newFakeAPI(t, map[string]fakeResponse{
		"GET /api/v1/push/vapid-public-key": {http.StatusOK, `{"public_key":"BPublicKey"}`},
	})

	rec := serve(web.routes(), browserRequest(http.MethodGet, "/push/vapid-public-key", ""))

	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"public_key":"BPublicKey"}` {
		t.Fatalf("got %d %s, want the API's public key", rec.Code, rec.Body)
	}
	if calls := api.Calls(); len(calls) != 1 || calls[0].Cookie != "session-123" {
		t.Errorf("API calls = %+v, want one call carrying the session", calls)
	}
}

func TestRegisteringAPushSubscriptionIsForwardedWithTheBrowsersUserAgent(t *testing.T) {
	api, web := newFakeAPI(t, map[string]fakeResponse{
		"POST /api/v1/push-subscriptions": {http.StatusCreated, `{"id":7,"name":"Safari on iPhone"}`},
	})
	subscription := `{"endpoint":"https://push.example/phone","keys":{"p256dh":"k","auth":"a"}}`

	rec := serve(web.routes(), browserRequest(http.MethodPost, "/push-subscriptions", subscription))

	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"id":7`) {
		t.Fatalf("got %d %s, want the API's 201 and Push Subscription", rec.Code, rec.Body)
	}
	calls := api.Calls()
	if len(calls) != 1 {
		t.Fatalf("API calls = %+v, want one", calls)
	}
	var sent, want map[string]any
	_ = json.Unmarshal([]byte(calls[0].Body), &sent)
	_ = json.Unmarshal([]byte(subscription), &want)
	if calls[0].Cookie != "session-123" || !strings.Contains(calls[0].UserAgent, "iPhone") || sent["endpoint"] != want["endpoint"] {
		t.Errorf("API call = %+v, want the subscription with the session and the browser's User-Agent", calls[0])
	}
}

func TestPushSubscriptionErrorsAreForwardedAsIs(t *testing.T) {
	_, web := newFakeAPI(t, map[string]fakeResponse{
		"POST /api/v1/push-subscriptions": {http.StatusForbidden, `{"error":"registering a push subscription requires a browser session"}`},
	})

	rec := serve(web.routes(), browserRequest(http.MethodPost, "/push-subscriptions", `{}`))

	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "browser session") {
		t.Errorf("got %d %s, want the API's 403 and error", rec.Code, rec.Body)
	}
}

func TestDisablingThisDeviceDeletesItsPushSubscription(t *testing.T) {
	api, web := newFakeAPI(t, map[string]fakeResponse{
		"DELETE /api/v1/push-subscriptions/7": {http.StatusOK, `{"ok":true}`},
	})

	rec := serve(web.routes(), browserRequest(http.MethodDelete, "/push-subscriptions/7", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if calls := api.Calls(); len(calls) != 1 || calls[0].Method != http.MethodDelete || calls[0].Path != "/api/v1/push-subscriptions/7" {
		t.Errorf("API calls = %+v, want DELETE /api/v1/push-subscriptions/7", calls)
	}
}

func TestRemovingADeviceInSettingsReturnsToSettings(t *testing.T) {
	api, web := newFakeAPI(t, map[string]fakeResponse{
		"DELETE /api/v1/push-subscriptions/7": {http.StatusOK, `{"ok":true}`},
	})

	rec := serve(web.routes(), browserRequest(http.MethodPost, "/settings/push-subscriptions/7/remove", ""))

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/settings" {
		t.Errorf("got %d to %q, want a redirect to /settings", rec.Code, rec.Header().Get("Location"))
	}
	if calls := api.Calls(); len(calls) != 1 || calls[0].Method != http.MethodDelete {
		t.Errorf("API calls = %+v, want one DELETE", calls)
	}
}

func TestTappingAPushMarksItReadAndReturnsTheUnreadCount(t *testing.T) {
	api, web := newFakeAPI(t, map[string]fakeResponse{
		"POST /api/v1/notifications/42/read": {http.StatusOK, `{"id":42,"url":"/tasks/9"}`},
		"GET /api/v1/notifications":          {http.StatusOK, `{"notifications":[],"unread_count":3}`},
	})

	rec := serve(web.routes(), browserRequest(http.MethodPost, "/notifications/42/read", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s, want 200", rec.Code, rec.Body)
	}
	var got struct {
		URL         string `json:"url"`
		UnreadCount int    `json:"unread_count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || got.UnreadCount != 3 || got.URL != "/tasks/9" {
		t.Errorf("body = %s, want url /tasks/9 and unread_count 3", rec.Body)
	}
	if calls := api.Calls(); len(calls) < 1 || calls[0].Path != "/api/v1/notifications/42/read" || calls[0].Cookie != "session-123" {
		t.Errorf("API calls = %+v, want the mark-read first, with the session", calls)
	}
}

func TestSettingsListsDevicesAndOffersToEnableThisOne(t *testing.T) {
	_, web := newFakeAPI(t, map[string]fakeResponse{
		"GET /api/v1/dashboard":          {http.StatusOK, `{"current_user":{"id":1,"email":"member@example.test","name":"Member"},"notifications":{"notifications":[],"unread_count":4}}`},
		"GET /api/v1/api-tokens":         {http.StatusOK, `[]`},
		"GET /api/v1/push-subscriptions": {http.StatusOK, `[{"id":7,"name":"Safari on iPhone","endpoint":"https://push.example/phone","created_at":"2030-01-10T09:00:00Z","last_used_at":"2030-01-14T18:30:00Z"}]`},
	})

	rec := serve(web.routes(), browserRequest(http.MethodGet, "/settings", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	page := rec.Body.String()
	for _, want := range []string{
		"Enable notifications on this device",
		"Add to Home Screen",
		"blocked",
		"Safari on iPhone",
		"Last used",
		`action="/settings/push-subscriptions/7/remove"`,
		`data-push-endpoint="https://push.example/phone"`,
		`data-app-badge="4"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("Settings page is missing %s", want)
		}
	}
}

func TestServiceWorkerHandlesPushAndTaps(t *testing.T) {
	body := get(t, newTestWebServer(t).routes(), "/sw.js").Body.String()
	for _, want := range []string{
		`addEventListener("push"`,
		`addEventListener("notificationclick"`,
		`setAppBadge`,
		`"/notifications/" + `,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("service worker is missing %s", want)
		}
	}
}

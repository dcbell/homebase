package main

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// recordAction is a web form action whose only API call is apiRoute, with where
// the Member lands when that call succeeds and when it fails.
type recordAction struct {
	name        string
	path        string
	form        url.Values
	apiRoute    string
	successTo   string
	failureBase string
}

const recordActionFailure = "That record is locked."

func recordActions() []recordAction {
	return []recordAction{
		{name: "archive Project", path: "/projects/5/archive", apiRoute: "POST /api/v1/projects/5/archive", successTo: "/projects", failureBase: "/projects/5"},
		{name: "archive Project Folder", path: "/projects/5/folders/6/archive", apiRoute: "POST /api/v1/project-folders/6/archive", successTo: "/projects/5", failureBase: "/projects/5"},
		{name: "archive Routine", path: "/routines/7/archive", apiRoute: "POST /api/v1/routines/7/archive", successTo: "/routines", failureBase: "/routines/7"},
		{name: "archive Asset", path: "/assets/2/archive", apiRoute: "POST /api/v1/assets/2/archive", successTo: "/assets", failureBase: "/assets/2"},
		{name: "archive Contact", path: "/contacts/8/archive", apiRoute: "POST /api/v1/contacts/8/archive", successTo: "/contacts", failureBase: "/contacts/8"},
		{name: "archive Document", path: "/documents/11/archive", apiRoute: "POST /api/v1/documents/11/archive", successTo: "/documents", failureBase: "/documents/11"},
		{name: "archive List", path: "/lists/3/archive", apiRoute: "POST /api/v1/lists/3/archive", successTo: "/lists", failureBase: "/lists/3"},
		{name: "archive List item", path: "/lists/3/items/4/archive", apiRoute: "POST /api/v1/list-items/4/archive", successTo: "/lists/3", failureBase: "/lists/3"},
		{name: "complete List item from the dashboard", path: "/lists/3/items/4/complete", form: url.Values{"return_to": {"/?list_id=3"}}, apiRoute: "PATCH /api/v1/list-items/4/complete", successTo: "/?list_id=3", failureBase: "/?list_id=3"},
		{name: "reopen List item", path: "/lists/3/items/4/reopen", apiRoute: "PATCH /api/v1/list-items/4/reopen", successTo: "/lists/3", failureBase: "/lists/3"},
		{name: "delete Appointment", path: "/events/9/delete", apiRoute: "DELETE /api/v1/events/9", successTo: "/", failureBase: "/events/9"},
		{name: "create Appointment", path: "/events", form: url.Values{"title": {"Dentist"}}, apiRoute: "POST /api/v1/events", successTo: "/", failureBase: "/"},
		{name: "create Project", path: "/projects", form: url.Values{"title": {"Shed"}, "return_to": {"/projects"}}, apiRoute: "POST /api/v1/projects", successTo: "/projects", failureBase: "/projects"},
		{name: "move dashboard tile", path: "/dashboard/tiles/move", form: url.Values{"tile": {"tasks"}, "direction": {"up"}}, apiRoute: "POST /api/v1/dashboard/tiles/move", successTo: "/", failureBase: "/"},
		{name: "unlink Document from its page", path: "/documents/11/related-items/20/remove", apiRoute: "DELETE /api/v1/document-links/20", successTo: "/documents/11", failureBase: "/documents/11"},
		{name: "unlink Document from Project", path: "/projects/5/documents/20/remove", apiRoute: "DELETE /api/v1/document-links/20", successTo: "/projects/5", failureBase: "/projects/5"},
		{name: "unlink Contact from Project", path: "/projects/5/contacts/21/remove", apiRoute: "DELETE /api/v1/contact-links/21", successTo: "/projects/5", failureBase: "/projects/5"},
		{name: "unlink Asset from Project", path: "/projects/5/assets/22/remove", apiRoute: "DELETE /api/v1/asset-links/22", successTo: "/projects/5", failureBase: "/projects/5"},
		{name: "unlink Document from Asset", path: "/assets/2/documents/20/remove", apiRoute: "DELETE /api/v1/document-links/20", successTo: "/assets/2", failureBase: "/assets/2"},
		{name: "unlink Contact from Asset", path: "/assets/2/contacts/21/remove", apiRoute: "DELETE /api/v1/contact-links/21", successTo: "/assets/2", failureBase: "/assets/2"},
	}
}

func postRecordAction(t *testing.T, action recordAction, status int, body string) (string, int) {
	t.Helper()
	_, web := newRecordingAPI(t, map[string]cannedResponse{action.apiRoute: {status, body}})
	req := browserRequest(http.MethodPost, action.path, action.form.Encode())
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := serve(web.routes(), req)
	return rec.Header().Get("Location"), rec.Code
}

func TestRecordActionsRedirectToTheirUsualPlaceOnSuccess(t *testing.T) {
	for _, action := range recordActions() {
		t.Run(action.name, func(t *testing.T) {
			location, code := postRecordAction(t, action, http.StatusOK, `{"ok":true}`)
			if code != http.StatusSeeOther || location != action.successTo {
				t.Errorf("got %d to %q, want a redirect to %q", code, location, action.successTo)
			}
		})
	}
}

func TestAFailedRecordActionShowsTheErrorPanelWhereItLands(t *testing.T) {
	_, web := newRecordingAPI(t, map[string]cannedResponse{
		"DELETE /api/v1/events/9": {http.StatusConflict, `{"error":"` + recordActionFailure + `"}`},
		"GET /api/v1/events/9":    {http.StatusOK, `{"id":9,"title":"Dentist"}`},
		"GET /api/v1/dashboard":   {http.StatusOK, `{}`},
	})
	h := web.routes()

	location := serve(h, browserRequest(http.MethodPost, "/events/9/delete", "")).Header().Get("Location")
	rec := serve(h, browserRequest(http.MethodGet, location, ""))

	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `<p class="panel error">`+recordActionFailure+`</p>`) {
		t.Errorf("GET %s got %d without the error panel saying %q", location, rec.Code, recordActionFailure)
	}
}

func TestRecordActionsReportAPIFailures(t *testing.T) {
	for _, action := range recordActions() {
		t.Run(action.name, func(t *testing.T) {
			location, code := postRecordAction(t, action, http.StatusConflict, `{"error":"`+recordActionFailure+`"}`)
			sep := "?"
			if strings.Contains(action.failureBase, "?") {
				sep = "&"
			}
			want := action.failureBase + sep + "error=" + url.QueryEscape(recordActionFailure)
			if code != http.StatusSeeOther || location != want {
				t.Errorf("got %d to %q, want a redirect to %q", code, location, want)
			}
		})
	}
}

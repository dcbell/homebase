package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// postTaskAction submits a form to the web service as a signed-in Member.
func postTaskAction(h http.Handler, path string, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "homebase_session", Value: "session-123"})
	return serve(h, req)
}

// taskAction is one Task or Project Task action, the API call it makes, and
// where the Member lands afterwards.
type taskAction struct {
	name     string
	path     string
	form     url.Values
	apiCall  string
	redirect string
}

var taskActions = []taskAction{
	{"create a Standalone Task", "/tasks", url.Values{"title": {"Mow"}, "return_to": {"/tasks"}}, "POST /api/v1/tasks", "/tasks"},
	{"complete a Task from the dashboard", "/tasks/complete", url.Values{"id": {"7"}}, "PATCH /api/v1/tasks/7/complete", "/"},
	{"reopen a Task", "/tasks/7/reopen", nil, "PATCH /api/v1/tasks/7/reopen", "/tasks/7"},
	{"archive a Task", "/tasks/7/archive", nil, "POST /api/v1/tasks/7/archive", "/tasks"},
	{"unlink a Document from a Task", "/tasks/7/documents/9/remove", nil, "DELETE /api/v1/document-links/9", "/tasks/7"},
	{"unlink a Contact from a Task", "/tasks/7/contacts/9/remove", nil, "DELETE /api/v1/contact-links/9", "/tasks/7"},
	{"unlink an Asset from a Task", "/tasks/7/assets/9/remove", nil, "DELETE /api/v1/asset-links/9", "/tasks/7"},
	{"complete a Project Task", "/projects/3/tasks/7/complete", nil, "PATCH /api/v1/tasks/7/complete", "/projects/3"},
	{"reopen a Project Task", "/projects/3/tasks/7/reopen", nil, "PATCH /api/v1/tasks/7/reopen", "/projects/3"},
	{"archive a Project Task", "/projects/3/tasks/7/archive", nil, "POST /api/v1/tasks/7/archive", "/projects/3"},
	{"unlink a Document from a Project Task", "/projects/3/tasks/7/documents/9/remove", nil, "DELETE /api/v1/document-links/9", "/projects/3"},
	{"unlink a Contact from a Project Task", "/projects/3/tasks/7/contacts/9/remove", nil, "DELETE /api/v1/contact-links/9", "/projects/3"},
	{"unlink an Asset from a Project Task", "/projects/3/tasks/7/assets/9/remove", nil, "DELETE /api/v1/asset-links/9", "/projects/3"},
}

func TestTaskActionsReportAPIFailures(t *testing.T) {
	for _, action := range taskActions {
		t.Run(action.name, func(t *testing.T) {
			api, web := newRecordingAPI(t, map[string]cannedResponse{
				action.apiCall: {http.StatusConflict, `{"error":"task is locked"}`},
			})

			rec := postTaskAction(web.routes(), action.path, action.form)

			if rec.Code != http.StatusSeeOther {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
			}
			want := action.redirect + "?error=task+is+locked"
			if got := rec.Header().Get("Location"); got != want {
				t.Fatalf("Location = %q, want %q", got, want)
			}
			if calls := api.Calls(); len(calls) != 1 || calls[0].Method+" "+calls[0].Path != action.apiCall {
				t.Fatalf("API calls = %+v, want one %s", calls, action.apiCall)
			}
		})
	}
}

func TestTaskActionsRedirectWithSavedWhenTheAPISucceeds(t *testing.T) {
	for _, action := range taskActions {
		t.Run(action.name, func(t *testing.T) {
			_, web := newRecordingAPI(t, map[string]cannedResponse{
				action.apiCall: {http.StatusOK, `{}`},
			})

			rec := postTaskAction(web.routes(), action.path, action.form)

			if rec.Code != http.StatusSeeOther {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
			}
			if got, want := rec.Header().Get("Location"), action.redirect+"?saved=1"; got != want {
				t.Fatalf("Location = %q, want %q", got, want)
			}
		})
	}
}

func TestCreatingAStandaloneTaskKeepsTheDueFilterWhenReportingAFailure(t *testing.T) {
	_, web := newRecordingAPI(t, map[string]cannedResponse{
		"POST /api/v1/tasks": {http.StatusBadRequest, `{"error":"title is required"}`},
	})

	rec := postTaskAction(web.routes(), "/tasks", url.Values{"return_to": {"/tasks?due=today"}})

	if got, want := rec.Header().Get("Location"), "/tasks?due=today&error=title+is+required"; got != want {
		t.Fatalf("Location = %q, want %q", got, want)
	}
}

func TestTaskActionDestinationsShowTheErrorPanel(t *testing.T) {
	_, web := newRecordingAPI(t, map[string]cannedResponse{
		"GET /api/v1/dashboard":          {http.StatusOK, `{}`},
		"GET /api/v1/tasks/7":            {http.StatusOK, `{"id":7,"title":"Mow"}`},
		"GET /api/v1/projects/3":         {http.StatusOK, `{"id":3,"title":"Garden"}`},
		"GET /api/v1/projects/3/folders": {http.StatusOK, `[]`},
		"GET /api/v1/documents":          {http.StatusOK, `[]`},
		"GET /api/v1/contacts":           {http.StatusOK, `[]`},
		"GET /api/v1/assets":             {http.StatusOK, `[]`},
		"GET /api/v1/related-documents":  {http.StatusOK, `[]`},
		"GET /api/v1/related-contacts":   {http.StatusOK, `[]`},
		"GET /api/v1/related-assets":     {http.StatusOK, `[]`},
		"GET /api/v1/lists":              {http.StatusOK, `[]`},
	})
	h := web.routes()

	for _, destination := range []string{"/", "/tasks", "/tasks/7", "/projects/3"} {
		t.Run(destination, func(t *testing.T) {
			rec := serve(h, browserRequest(http.MethodGet, destination+"?error=task+is+locked", ""))

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), `<p class="panel error">task is locked</p>`) {
				t.Fatalf("page does not show the error panel with the message")
			}
		})
	}
}

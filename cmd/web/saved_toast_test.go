package main

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestASuccessfulSaveRedirectsWithSavedKeepingTheQuery(t *testing.T) {
	_, web := newRecordingAPI(t, map[string]cannedResponse{
		"POST /api/v1/tasks": {http.StatusOK, `{"id":7}`},
	})

	rec := postTaskAction(web.routes(), "/tasks", url.Values{"title": {"Mow"}, "return_to": {"/tasks?due=today"}})

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got, want := rec.Header().Get("Location"), "/tasks?due=today&saved=1"; got != want {
		t.Fatalf("Location = %q, want %q", got, want)
	}
}

func TestSavingATaskFromItsPageStaysThereWithSaved(t *testing.T) {
	_, web := newRecordingAPI(t, map[string]cannedResponse{
		"PATCH /api/v1/tasks/7": {http.StatusOK, `{"id":7}`},
	})

	rec := postTaskAction(web.routes(), "/tasks/7", url.Values{"title": {"Mow the lawn"}, "status": {"open"}})

	if got, want := rec.Header().Get("Location"), "/tasks/7?saved=1"; got != want {
		t.Fatalf("Location = %q, want %q", got, want)
	}
}

func TestASuccessfulSaveKeepsTheFragment(t *testing.T) {
	_, web := newRecordingAPI(t, map[string]cannedResponse{
		"POST /api/v1/tasks": {http.StatusOK, `{"id":7}`},
	})

	rec := postTaskAction(web.routes(), "/tasks", url.Values{"title": {"Mow"}, "return_to": {"/tasks?due=today#add"}})

	if got, want := rec.Header().Get("Location"), "/tasks?due=today&saved=1#add"; got != want {
		t.Fatalf("Location = %q, want %q", got, want)
	}
}

func TestARedirectThatAlreadySaysSavedKeepsOneSaved(t *testing.T) {
	_, web := newRecordingAPI(t, map[string]cannedResponse{
		"POST /api/v1/tasks": {http.StatusOK, `{"id":7}`},
	})

	rec := postTaskAction(web.routes(), "/tasks", url.Values{"title": {"Mow"}, "return_to": {"/tasks?saved=1#add"}})

	if got, want := rec.Header().Get("Location"), "/tasks?saved=1#add"; got != want {
		t.Fatalf("Location = %q, want %q", got, want)
	}
}

func TestAFailedSaveRedirectsWithTheErrorAndNoSaved(t *testing.T) {
	_, web := newRecordingAPI(t, map[string]cannedResponse{
		"POST /api/v1/tasks": {http.StatusBadRequest, `{"error":"title is required"}`},
	})

	rec := postTaskAction(web.routes(), "/tasks", url.Values{"return_to": {"/tasks?due=today"}})

	if got, want := rec.Header().Get("Location"), "/tasks?due=today&error=title+is+required"; got != want {
		t.Fatalf("Location = %q, want %q", got, want)
	}
}

func TestPostsThatAreNotSavesNeverGetSaved(t *testing.T) {
	posts := []struct {
		name    string
		path    string
		form    url.Values
		apiCall string
		body    string
	}{
		{"logout", "/logout", nil, "POST /auth/logout", `{}`},
		{"mark all notifications read", "/notifications/read-all", nil, "POST /api/v1/notifications/read-all", `{}`},
		{"open a notification", "/notifications/42/open", nil, "POST /api/v1/notifications/42/read", `{"id":42,"url":"/tasks/9"}`},
		{"move a dashboard tile", "/dashboard/tiles/move", url.Values{"tile": {"tasks"}, "direction": {"up"}}, "POST /api/v1/dashboard/tiles/move", `{}`},
	}
	for _, post := range posts {
		t.Run(post.name, func(t *testing.T) {
			_, web := newRecordingAPI(t, map[string]cannedResponse{
				post.apiCall: {http.StatusOK, post.body},
			})

			rec := postTaskAction(web.routes(), post.path, post.form)

			if rec.Code != http.StatusSeeOther {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
			}
			if got := rec.Header().Get("Location"); strings.Contains(got, "saved") {
				t.Fatalf("Location = %q, want no saved", got)
			}
		})
	}
}

const savedToast = `<div class="toast" role="status">Saved ✓</div>`

func getTasksPage(t *testing.T, query string) string {
	t.Helper()
	_, web := newRecordingAPI(t, map[string]cannedResponse{
		"GET /api/v1/dashboard": {http.StatusOK, tasksDashboard},
	})
	rec := serve(web.routes(), browserRequest(http.MethodGet, "/tasks"+query, ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	return rec.Body.String()
}

func TestAPageAfterASaveShowsTheSavedToast(t *testing.T) {
	body := getTasksPage(t, "?due=today&saved=1")

	if !strings.Contains(body, savedToast) {
		t.Errorf("page has no %s", savedToast)
	}
}

func TestAPageWithoutSavedShowsNoToast(t *testing.T) {
	body := getTasksPage(t, "?due=today")

	if strings.Contains(body, "Saved ✓") {
		t.Errorf("page shows the Saved toast without saved=1")
	}
}

func TestAPageWithAnErrorShowsOnlyTheError(t *testing.T) {
	body := getTasksPage(t, "?error=title+is+required&saved=1")

	if !strings.Contains(body, `<p class="panel error">title is required</p>`) {
		t.Errorf("page has no error panel")
	}
	if strings.Contains(body, "Saved ✓") {
		t.Errorf("page shows the Saved toast alongside the error")
	}
}

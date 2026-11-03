package main

import (
	"net/http"
	"regexp"
	"strings"
	"testing"
)

const tasksDashboard = `{
	"current_user": {"id": 1, "email": "member@example.com", "name": "Member"},
	"tasks": [
		{"id": 11, "title": "Fix the gate", "status": "open", "priority": "normal"},
		{"id": 12, "title": "Paint the shed", "status": "done", "priority": "normal"}
	]
}`

func TestTasksListOpensOnOpenTasks(t *testing.T) {
	_, web := newRecordingAPI(t, map[string]cannedResponse{
		"GET /api/v1/dashboard": {http.StatusOK, tasksDashboard},
	})

	rec := serve(web.routes(), browserRequest(http.MethodGet, "/tasks", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	statusFilter := regexp.MustCompile(`(?s)<select data-filter-kind[^>]*>.*?</select>`).FindString(body)
	if !strings.Contains(statusFilter, `<option value="open" selected>`) {
		t.Errorf("Status filter = %q, want open selected", statusFilter)
	}
	if strings.Contains(statusFilter, `<option value="" selected>`) || strings.Contains(statusFilter, `<option value="done" selected>`) {
		t.Errorf("Status filter = %q, want only open selected", statusFilter)
	}
	if card := taskCard(t, body, "Paint the shed"); !strings.Contains(card, " hidden") {
		t.Errorf("done Task card = %q, want it hidden on first load", card)
	}
	if card := taskCard(t, body, "Fix the gate"); strings.Contains(card, " hidden") {
		t.Errorf("open Task card = %q, want it shown on first load", card)
	}
}

// taskCard returns the opening tag of the Tasks list card for the titled Task.
func taskCard(t *testing.T, body, title string) string {
	t.Helper()
	re := regexp.MustCompile(`<article class="item task-index-card"[^>]*data-filter-text="` + regexp.QuoteMeta(title) + `[^"]*"[^>]*>`)
	card := re.FindString(body)
	if card == "" {
		t.Fatalf("no Tasks list card for %q", title)
	}
	return card
}

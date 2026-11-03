package api_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"homebase/internal/apitest"
	"homebase/internal/store"
)

func createProjectTask(t *testing.T, household apitest.Household, due time.Time) (store.Project, store.Task) {
	t.Helper()
	project := apitest.Decode[store.Project](t, household.Owner.Post("/api/v1/projects", map[string]any{"title": "Kitchen remodel"}).Expect(http.StatusCreated))
	task := apitest.Decode[store.Task](t, household.Owner.Post("/api/v1/tasks", map[string]any{
		"title":       "Pick tiles",
		"project_id":  project.ID,
		"assigned_to": household.Member.User.ID,
		"due_at":      due.Format(time.RFC3339),
	}).Expect(http.StatusCreated))
	return project, task
}

func TestArchivingProjectCancelsItsTasksReminders(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	project, _ := createProjectTask(t, household, at(17, 18, 0))

	household.Owner.Post(fmt.Sprintf("/api/v1/projects/%d/archive", project.ID), nil).Expect(http.StatusOK)

	sendRemindersAt(t, h, at(30, 8, 0))
	expectReminders(t, household.Member)
}

func TestArchivingProjectThroughEditCancelsItsTasksReminders(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	project, _ := createProjectTask(t, household, at(17, 18, 0))

	household.Owner.Patch(fmt.Sprintf("/api/v1/projects/%d", project.ID), map[string]any{
		"title":  project.Title,
		"status": "archived",
	}).Expect(http.StatusOK)

	sendRemindersAt(t, h, at(30, 8, 0))
	expectReminders(t, household.Member)
}

func TestRestoringArchivedProjectRestoresRemindersStillAhead(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	project, task := createProjectTask(t, household, at(17, 18, 0))
	household.Owner.Post(fmt.Sprintf("/api/v1/projects/%d/archive", project.ID), nil).Expect(http.StatusOK)

	sendRemindersAt(t, h, at(16, 18, 0))
	household.Owner.Patch(fmt.Sprintf("/api/v1/projects/%d", project.ID), map[string]any{
		"title":  project.Title,
		"status": "active",
	}).Expect(http.StatusOK)

	sendRemindersAt(t, h, at(18, 8, 0))
	expectReminders(t, household.Member, fmt.Sprintf("due_now %d", task.ID), fmt.Sprintf("overdue %d", task.ID))
}

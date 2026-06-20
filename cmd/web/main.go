package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"homebase/internal/config"
	"homebase/internal/store"
)

type webServer struct {
	cfg       config.Config
	logger    *slog.Logger
	client    *http.Client
	templates *template.Template
}

type pageData struct {
	Title                 string
	Error                 string
	Dashboard             store.Dashboard
	Project               store.Project
	Task                  store.Task
	Event                 store.Event
	Routine               store.Routine
	Document              store.Document
	Contact               store.Contact
	Asset                 store.Asset
	List                  store.HouseholdList
	Lists                 []store.HouseholdList
	ListItems             []store.ListItem
	ProjectIndex          bool
	TaskIndex             bool
	RoutineIndex          bool
	MemberIndex           bool
	ListIndex             bool
	ContactIndex          bool
	AssetIndex            bool
	DocumentIndex         bool
	SettingsPage          bool
	DashboardPage         bool
	Calendar              *store.CalendarMonth
	CalendarPage          bool
	CalendarView          string
	CalendarFocus         time.Time
	DueFilter             string
	Projects              []store.Project
	Members               []store.User
	Tasks                 []store.Task
	Routines              []store.Routine
	ProjectFolders        []store.ProjectFolder
	Contacts              []store.Contact
	Documents             []store.Document
	Assets                []store.Asset
	AssetMaintenanceItems []store.AssetMaintenanceItem
	APITokens             []store.APIToken
	CreatedAPIToken       store.APITokenWithSecret
	RelatedItems          []store.RelatedItem
	RelatedDocs           []store.RelatedDocument
	RelatedContacts       []store.RelatedContact
	RelatedAssets         []store.RelatedAsset
	TaskDocuments         map[int64][]store.RelatedDocument
	TaskContacts          map[int64][]store.RelatedContact
	TaskAssets            map[int64][]store.RelatedAsset
	Modules               map[string][]store.ModuleItem
	APIBaseURL            string
	LoginURL              string
	LogoutURL             string
	Now                   time.Time
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := config.Load()
	if err := cfg.ApplyTimezone(); err != nil {
		logger.Error("load application timezone", "timezone", cfg.Timezone, "error", err)
		os.Exit(1)
	}

	s := &webServer{
		cfg:    cfg,
		logger: logger,
		client: &http.Client{Timeout: cfg.HTTPClientTimeout},
	}
	s.templates = template.Must(template.New("app").Funcs(template.FuncMap{
		"date":                  formatDate,
		"datetime":              formatDateTime,
		"dateInput":             formatDateInput,
		"datetimeInput":         formatDateTimeInput,
		"datetimeInputPtr":      formatDateTimeInputPtr,
		"selectedID":            selectedID,
		"selectedString":        selectedString,
		"idValue":               idValue,
		"initials":              initials,
		"count":                 count,
		"order":                 tileOrder,
		"tileActive":            tileActive,
		"activeTasks":           activeTasks,
		"activeStandaloneTasks": activeStandaloneTasks,
		"activeProjectTasks":    activeProjectTasks,
		"taskContext":           taskContext,
		"standaloneTasks":       standaloneTasks,
		"projectTasks":          projectTasks,
		"tasksInFolder":         tasksInFolder,
		"tasksWithoutFolder":    tasksWithoutFolder,
		"openTaskCount":         openTaskCount,
		"doneTaskCount":         doneTaskCount,
		"folderDue":             folderDue,
		"folderStatus":          folderStatus,
		"openListItemCount":     openListItemCount,
		"doneListItemCount":     doneListItemCount,
		"routineTasks":          routineTasks,
		"assetTasks":            assetTasks,
		"taskStatCount":         taskStatCount,
		"projectStatCount":      projectStatCount,
		"taskDueBucket":         taskDueBucket,
		"projectDueBucket":      projectDueBucket,
		"eventsForDayOffset":    eventsForDayOffset,
		"monthTitle":            monthTitle,
		"dayNumber":             dayNumber,
		"dateTitle":             dateTitle,
		"dateShort":             dateShort,
		"weekdayShort":          weekdayShort,
		"entryTime":             entryTime,
		"calendarDay":           calendarDay,
		"calendarWeek":          calendarWeek,
		"addDays":               addDays,
		"addMonths":             addMonths,
		"calendarStep":          calendarStep,
		"dashboardCalendarURL":  dashboardCalendarURL,
		"calendarPageURL":       calendarPageURL,
		"dictRelated":           dictRelated,
		"dict":                  dict,
		"dictRelatedPrefix":     dictRelatedPrefix,
		"dictDocumentInfo":      dictDocumentInfo,
		"dictRelatedContacts":   dictRelatedContacts,
		"dictContactInfo":       dictContactInfo,
		"dictRelatedAssets":     dictRelatedAssets,
		"dictAssetInfo":         dictAssetInfo,
		"dictAttach":            dictAttach,
		"dictAttachContact":     dictAttachContact,
		"dictAttachAsset":       dictAttachAsset,
		"taskRelatedDocs":       taskRelatedDocs,
		"hasTaskDocuments":      hasTaskDocuments,
		"taskRelatedContacts":   taskRelatedContacts,
		"hasTaskContacts":       hasTaskContacts,
		"taskRelatedAssets":     taskRelatedAssets,
		"hasTaskAssets":         hasTaskAssets,
		"documentOpenURL":       documentOpenURL,
		"documentSourceLabel":   documentSourceLabel,
		"fileSize":              fileSize,
		"money":                 formatMoney,
	}).Parse(appTemplate))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /favicon.svg", favicon)
	mux.HandleFunc("GET /docs", s.swaggerDocs)
	mux.HandleFunc("GET /docs/openapi.yaml", s.openapi)
	mux.HandleFunc("GET /openapi.yaml", s.openapi)
	mux.HandleFunc("GET /", s.dashboard)
	mux.HandleFunc("GET /calendar", s.calendar)
	mux.HandleFunc("GET /search", s.search)
	mux.HandleFunc("GET /settings", s.settings)
	mux.HandleFunc("POST /settings/api-tokens", s.createAPIToken)
	mux.HandleFunc("POST /settings/api-tokens/{id}/revoke", s.revokeAPIToken)
	mux.HandleFunc("POST /dashboard/tiles/move", s.moveDashboardTile)
	mux.HandleFunc("POST /dashboard/tiles/order", s.setDashboardTileOrder)
	mux.HandleFunc("POST /members", s.addMember)
	mux.HandleFunc("POST /members/{id}", s.updateMember)
	mux.HandleFunc("POST /members/{id}/remove", s.removeMember)
	mux.HandleFunc("GET /members", s.memberIndex)
	mux.HandleFunc("GET /projects", s.projectIndex)
	mux.HandleFunc("GET /projects/{id}", s.projectDetail)
	mux.HandleFunc("POST /projects/{id}/folders", s.createProjectFolder)
	mux.HandleFunc("POST /projects/{projectID}/folders/{folderID}", s.updateProjectFolder)
	mux.HandleFunc("POST /projects/{projectID}/folders/{folderID}/archive", s.archiveProjectFolder)
	mux.HandleFunc("POST /projects/{id}/tasks", s.createProjectTask)
	mux.HandleFunc("POST /projects/{projectID}/tasks/{taskID}/documents", s.attachProjectTaskDocument)
	mux.HandleFunc("POST /projects/{projectID}/tasks/{taskID}/documents/{linkID}/remove", s.unlinkProjectTaskDocument)
	mux.HandleFunc("POST /projects/{projectID}/tasks/{taskID}/contacts", s.attachProjectTaskContact)
	mux.HandleFunc("POST /projects/{projectID}/tasks/{taskID}/contacts/{linkID}/remove", s.unlinkProjectTaskContact)
	mux.HandleFunc("POST /projects/{projectID}/tasks/{taskID}/assets", s.attachProjectTaskAsset)
	mux.HandleFunc("POST /projects/{projectID}/tasks/{taskID}/assets/{linkID}/remove", s.unlinkProjectTaskAsset)
	mux.HandleFunc("POST /projects/{projectID}/tasks/{taskID}", s.updateProjectTask)
	mux.HandleFunc("POST /projects/{projectID}/tasks/{taskID}/complete", s.completeProjectTask)
	mux.HandleFunc("POST /projects/{projectID}/tasks/{taskID}/reopen", s.reopenProjectTask)
	mux.HandleFunc("POST /projects/{projectID}/tasks/{taskID}/archive", s.archiveProjectTask)
	mux.HandleFunc("POST /projects/{id}", s.updateProject)
	mux.HandleFunc("POST /projects/{id}/archive", s.archiveProject)
	mux.HandleFunc("POST /projects", s.createProject)
	mux.HandleFunc("GET /tasks", s.taskIndex)
	mux.HandleFunc("GET /tasks/{id}", s.taskDetail)
	mux.HandleFunc("POST /tasks/{id}", s.updateTask)
	mux.HandleFunc("POST /tasks/{id}/archive", s.archiveTask)
	mux.HandleFunc("POST /tasks/{id}/reopen", s.reopenTask)
	mux.HandleFunc("POST /tasks", s.createTask)
	mux.HandleFunc("GET /events/{id}", s.eventDetail)
	mux.HandleFunc("POST /events/{id}", s.updateEvent)
	mux.HandleFunc("POST /events/{id}/delete", s.deleteEvent)
	mux.HandleFunc("POST /events", s.createEvent)
	mux.HandleFunc("GET /routines", s.routineIndex)
	mux.HandleFunc("GET /routines/{id}", s.routineDetail)
	mux.HandleFunc("POST /routines/{id}", s.updateRoutine)
	mux.HandleFunc("POST /routines/{id}/archive", s.archiveRoutine)
	mux.HandleFunc("POST /routines/{id}/generate-task", s.generateRoutineTask)
	mux.HandleFunc("POST /routines", s.createRoutine)
	mux.HandleFunc("GET /lists", s.listIndex)
	mux.HandleFunc("POST /lists", s.createList)
	mux.HandleFunc("GET /lists/{id}", s.listDetail)
	mux.HandleFunc("POST /lists/{id}/items", s.createListItem)
	mux.HandleFunc("POST /lists/{listID}/items/{itemID}", s.updateListItem)
	mux.HandleFunc("POST /lists/{listID}/items/{itemID}/complete", s.completeListItem)
	mux.HandleFunc("POST /lists/{listID}/items/{itemID}/reopen", s.reopenListItem)
	mux.HandleFunc("POST /lists/{listID}/items/{itemID}/archive", s.archiveListItem)
	mux.HandleFunc("POST /lists/{id}", s.updateList)
	mux.HandleFunc("POST /lists/{id}/archive", s.archiveList)
	mux.HandleFunc("GET /contacts", s.contactIndex)
	mux.HandleFunc("POST /contacts", s.createContact)
	mux.HandleFunc("GET /contacts/{id}", s.contactDetail)
	mux.HandleFunc("POST /contacts/{id}", s.updateContact)
	mux.HandleFunc("POST /contacts/{id}/archive", s.archiveContact)
	mux.HandleFunc("GET /assets", s.assetIndex)
	mux.HandleFunc("POST /assets", s.createAsset)
	mux.HandleFunc("GET /assets/{id}", s.assetDetail)
	mux.HandleFunc("POST /assets/{id}/maintenance", s.createAssetMaintenanceItem)
	mux.HandleFunc("POST /assets/{assetID}/maintenance/{itemID}", s.updateAssetMaintenanceItem)
	mux.HandleFunc("POST /assets/{assetID}/maintenance/{itemID}/archive", s.archiveAssetMaintenanceItem)
	mux.HandleFunc("POST /assets/{assetID}/maintenance/{itemID}/generate-task", s.generateAssetMaintenanceTask)
	mux.HandleFunc("POST /assets/{id}", s.updateAsset)
	mux.HandleFunc("POST /assets/{id}/archive", s.archiveAsset)
	mux.HandleFunc("POST /assets/{id}/documents", s.attachAssetDocument)
	mux.HandleFunc("POST /assets/{assetID}/documents/{linkID}/remove", s.unlinkAssetDocument)
	mux.HandleFunc("POST /assets/{id}/contacts", s.attachAssetContact)
	mux.HandleFunc("POST /assets/{assetID}/contacts/{linkID}/remove", s.unlinkAssetContact)
	mux.HandleFunc("GET /documents", s.documentIndex)
	mux.HandleFunc("POST /documents", s.createDocument)
	mux.HandleFunc("GET /documents/{id}/download", s.downloadDocument)
	mux.HandleFunc("GET /documents/{id}/file/{name}", s.downloadDocument)
	mux.HandleFunc("GET /documents/{id}", s.documentDetail)
	mux.HandleFunc("POST /documents/{id}", s.updateDocument)
	mux.HandleFunc("POST /documents/{id}/archive", s.archiveDocument)
	mux.HandleFunc("POST /documents/{id}/related-items", s.linkDocumentRelatedItem)
	mux.HandleFunc("POST /documents/{documentID}/related-items/{linkID}/remove", s.unlinkDocumentFromDocumentPage)
	mux.HandleFunc("POST /projects/{id}/documents", s.attachProjectDocument)
	mux.HandleFunc("POST /projects/{projectID}/documents/{linkID}/remove", s.unlinkProjectDocument)
	mux.HandleFunc("POST /projects/{id}/contacts", s.attachProjectContact)
	mux.HandleFunc("POST /projects/{projectID}/contacts/{linkID}/remove", s.unlinkProjectContact)
	mux.HandleFunc("POST /projects/{id}/assets", s.attachProjectAsset)
	mux.HandleFunc("POST /projects/{projectID}/assets/{linkID}/remove", s.unlinkProjectAsset)
	mux.HandleFunc("POST /tasks/{id}/documents", s.attachTaskDocument)
	mux.HandleFunc("POST /tasks/{taskID}/documents/{linkID}/remove", s.unlinkTaskDocument)
	mux.HandleFunc("POST /tasks/{id}/contacts", s.attachTaskContact)
	mux.HandleFunc("POST /tasks/{taskID}/contacts/{linkID}/remove", s.unlinkTaskContact)
	mux.HandleFunc("POST /tasks/{id}/assets", s.attachTaskAsset)
	mux.HandleFunc("POST /tasks/{taskID}/assets/{linkID}/remove", s.unlinkTaskAsset)
	mux.HandleFunc("POST /tasks/complete", s.completeTask)
	mux.HandleFunc("POST /logout", s.logout)

	server := &http.Server{
		Addr:              cfg.WebAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("web listening", "addr", cfg.WebAddr, "timezone", cfg.Timezone)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("web server", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("web shutdown", "error", err)
	}
}

func (s *webServer) swaggerDocs(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`<!doctype html>
<html lang="en">
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<title>Homebase API Docs</title>
	<link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
	<style>
		body { margin:0; background:#f6f7fb; }
		.swagger-ui .topbar { display:none; }
	</style>
</head>
<body>
	<div id="swagger-ui"></div>
	<script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
	<script>
		window.addEventListener("load", function () {
			SwaggerUIBundle({
				url: "/docs/openapi.yaml",
				dom_id: "#swagger-ui",
				deepLinking: true,
				presets: [SwaggerUIBundle.presets.apis],
				layout: "BaseLayout"
			});
		});
	</script>
</body>
</html>`))
}

func (s *webServer) openapi(w http.ResponseWriter, r *http.Request) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, s.cfg.APIInternalURL+"/openapi.yaml", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	resp, err := s.client.Do(req)
	if err != nil {
		s.logger.Error("load openapi", "error", err)
		http.Error(w, "failed to load OpenAPI document", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, "failed to load OpenAPI document", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, resp.Body)
}

func (s *webServer) dashboard(w http.ResponseWriter, r *http.Request) {
	pageError := userFacingError(r.URL.Query().Get("error"))
	var dashboard store.Dashboard
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/dashboard", nil, &dashboard); err != nil {
		if errors.Is(err, errUnauthorized) {
			s.render(w, http.StatusOK, pageData{
				Title:      "Homebase",
				Error:      pageError,
				APIBaseURL: s.cfg.APIBaseURL,
				LoginURL:   s.cfg.APIBaseURL + "/auth/oauth/start",
				Now:        time.Now(),
			})
			return
		}
		s.render(w, http.StatusBadGateway, pageData{Title: "Homebase", Error: err.Error(), Now: time.Now()})
		return
	}

	calendarView, calendarFocus := calendarStateFromRequest(r)
	if validDashboardCalendarView(strings.TrimSpace(r.URL.Query().Get("calendar_view"))) {
		http.SetCookie(w, &http.Cookie{
			Name:     "homebase_dashboard_calendar_view",
			Value:    calendarView,
			Path:     "/",
			MaxAge:   60 * 60 * 24 * 365,
			SameSite: http.SameSiteLaxMode,
			Secure:   s.cfg.Env == "production",
		})
	}
	var calendar store.CalendarMonth
	calendarPath := "/api/v1/calendar?month=" + url.QueryEscape(calendarFocus.Format("2006-01"))
	if err := s.apiJSON(r, http.MethodGet, calendarPath, nil, &calendar); err != nil {
		s.logger.Error("load dashboard calendar", "error", err)
	}

	var lists []store.HouseholdList
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/lists", nil, &lists); err != nil {
		s.logger.Error("load dashboard lists", "error", err)
	}
	selectedList := store.HouseholdList{}
	selectedListID := selectedDashboardListID(r, lists)
	if selectedListID != 0 {
		for _, list := range lists {
			if list.ID == selectedListID {
				selectedList = list
				break
			}
		}
		if selectedList.ID != 0 && strings.TrimSpace(r.URL.Query().Get("list_id")) != "" {
			http.SetCookie(w, &http.Cookie{
				Name:     "homebase_dashboard_list_id",
				Value:    strconv.FormatInt(selectedList.ID, 10),
				Path:     "/",
				MaxAge:   60 * 60 * 24 * 365,
				SameSite: http.SameSiteLaxMode,
				Secure:   s.cfg.Env == "production",
			})
		}
	}
	var listItems []store.ListItem
	if selectedList.ID != 0 {
		if err := s.apiJSON(r, http.MethodGet, "/api/v1/lists/"+strconv.FormatInt(selectedList.ID, 10)+"/items", nil, &listItems); err != nil {
			s.logger.Error("load dashboard list items", "error", err)
		}
	}

	s.render(w, http.StatusOK, pageData{
		Title:         "Homebase",
		Error:         pageError,
		Dashboard:     dashboard,
		DashboardPage: true,
		Calendar:      &calendar,
		CalendarView:  calendarView,
		CalendarFocus: calendarFocus,
		Lists:         lists,
		List:          selectedList,
		ListItems:     listItems,
		LoginURL:      s.cfg.APIBaseURL + "/auth/oauth/start",
		LogoutURL:     "/logout",
		Now:           time.Now(),
	})
}

func (s *webServer) calendar(w http.ResponseWriter, r *http.Request) {
	var dashboard store.Dashboard
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/dashboard", nil, &dashboard); err != nil {
		if errors.Is(err, errUnauthorized) {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		s.render(w, http.StatusBadGateway, pageData{Title: "Calendar", Error: err.Error(), Now: time.Now()})
		return
	}

	path := "/api/v1/calendar"
	if month := strings.TrimSpace(r.URL.Query().Get("month")); month != "" {
		path += "?month=" + url.QueryEscape(month)
	}
	var calendar store.CalendarMonth
	if err := s.apiJSON(r, http.MethodGet, path, nil, &calendar); err != nil {
		s.render(w, http.StatusBadGateway, pageData{Title: "Calendar", Error: err.Error(), Dashboard: dashboard, Now: time.Now()})
		return
	}

	s.render(w, http.StatusOK, pageData{
		Title:         "Calendar",
		Error:         userFacingError(r.URL.Query().Get("error")),
		Dashboard:     dashboard,
		Calendar:      &calendar,
		CalendarPage:  true,
		CalendarView:  "month",
		CalendarFocus: calendar.Month,
		LoginURL:      s.cfg.APIBaseURL + "/auth/oauth/start",
		LogoutURL:     "/logout",
		Now:           time.Now(),
	})
}

func (s *webServer) moveDashboardTile(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err == nil {
		payload := map[string]any{
			"tile":      r.FormValue("tile"),
			"direction": r.FormValue("direction"),
		}
		_ = s.apiJSON(r, http.MethodPost, "/api/v1/dashboard/tiles/move", payload, nil)
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *webServer) setDashboardTileOrder(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Tiles []string `json:"tiles"`
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if err := s.apiJSON(r, http.MethodPost, "/api/v1/dashboard/tiles/order", payload, nil); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true}`))
}

type searchResult struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	URL      string `json:"url"`
}

func (s *webServer) search(w http.ResponseWriter, r *http.Request) {
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	results := []searchResult{}
	if query == "" {
		writeSearchResults(w, results)
		return
	}

	var dashboard store.Dashboard
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/dashboard", nil, &dashboard); err != nil {
		if errors.Is(err, errUnauthorized) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	for _, project := range dashboard.Projects {
		subtitle := project.Status
		if project.DueDate != nil {
			subtitle = compactMeta(subtitle, "due "+formatDate(project.DueDate))
		}
		results = appendSearchResult(results, query, "Project", project.Title, subtitle, "/projects/"+strconv.FormatInt(project.ID, 10))
	}
	for _, task := range dashboard.Tasks {
		subtitle := task.Status
		if task.DueAt != nil {
			subtitle = compactMeta(subtitle, "due "+formatDate(task.DueAt))
		}
		if task.AssignedName != "" {
			subtitle = compactMeta(subtitle, task.AssignedName)
		}
		results = appendSearchResult(results, query, "Task", task.Title, subtitle, "/tasks/"+strconv.FormatInt(task.ID, 10))
	}
	for _, event := range dashboard.Events {
		subtitle := compactMeta(formatDateTime(event.StartsAt), event.Location)
		results = appendSearchResult(results, query, "Appointment", event.Title, subtitle, "/events/"+strconv.FormatInt(event.ID, 10))
	}
	for _, routine := range dashboard.Routines {
		subtitle := routine.Cadence
		if routine.NextDueAt != nil {
			subtitle = compactMeta(subtitle, "next "+formatDate(routine.NextDueAt))
		}
		results = appendSearchResult(results, query, "Routine", routine.Title, subtitle, "/routines/"+strconv.FormatInt(routine.ID, 10))
	}

	var documents []store.Document
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/documents", nil, &documents); err != nil {
		s.logger.Error("load search documents", "error", err)
	} else {
		for _, document := range documents {
			subtitle := compactMeta(document.Kind, document.FileName)
			results = appendSearchResult(results, query, "Document", document.Title, subtitle, "/documents/"+strconv.FormatInt(document.ID, 10))
		}
	}

	var contacts []store.Contact
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/contacts", nil, &contacts); err != nil {
		s.logger.Error("load search contacts", "error", err)
	} else {
		for _, contact := range contacts {
			subtitle := compactMeta(contact.Kind, contact.Email, contact.Phone)
			results = appendSearchResult(results, query, "Contact", contact.Name, subtitle, "/contacts/"+strconv.FormatInt(contact.ID, 10))
		}
	}

	var assets []store.Asset
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/assets", nil, &assets); err != nil {
		s.logger.Error("load search assets", "error", err)
	} else {
		for _, asset := range assets {
			subtitle := compactMeta(asset.Kind, asset.Model, asset.SerialNumber)
			results = appendSearchResult(results, query, "Asset", asset.Name, subtitle, "/assets/"+strconv.FormatInt(asset.ID, 10))
		}
	}

	var lists []store.HouseholdList
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/lists", nil, &lists); err != nil {
		s.logger.Error("load search lists", "error", err)
	} else {
		for _, list := range lists {
			subtitle := compactMeta(list.Kind, list.Description)
			results = appendSearchResult(results, query, "List", list.Title, subtitle, "/lists/"+strconv.FormatInt(list.ID, 10))
		}
	}

	if len(results) > 12 {
		results = results[:12]
	}
	writeSearchResults(w, results)
}

func appendSearchResult(results []searchResult, query, kind, title, subtitle, href string) []searchResult {
	if !searchMatches(query, kind, title, subtitle) {
		return results
	}
	return append(results, searchResult{
		Type:     kind,
		Title:    title,
		Subtitle: subtitle,
		URL:      href,
	})
}

func searchMatches(query string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}
	return false
}

func compactMeta(parts ...string) string {
	kept := []string{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, " · ")
}

func writeSearchResults(w http.ResponseWriter, results []searchResult) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(results); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *webServer) addMember(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/?error=form", http.StatusSeeOther)
		return
	}
	payload := map[string]any{
		"email": r.FormValue("email"),
		"name":  r.FormValue("name"),
		"role":  defaultValue(r.FormValue("role"), "member"),
	}
	if err := s.apiJSON(r, http.MethodPost, "/api/v1/members", payload, nil); err != nil {
		http.Redirect(w, r, "/?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, safeReturnTo(r.FormValue("return_to"), "/"), http.StatusSeeOther)
}

func (s *webServer) updateMember(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/?error=form", http.StatusSeeOther)
		return
	}
	payload := map[string]any{
		"email": r.FormValue("email"),
		"name":  r.FormValue("name"),
		"role":  defaultValue(r.FormValue("role"), "member"),
	}
	if err := s.apiJSON(r, http.MethodPatch, "/api/v1/members/"+url.PathEscape(id), payload, nil); err != nil {
		http.Redirect(w, r, "/?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, safeReturnTo(r.FormValue("return_to"), "/"), http.StatusSeeOther)
}

func (s *webServer) removeMember(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_ = r.ParseForm()
	if err := s.apiJSON(r, http.MethodDelete, "/api/v1/members/"+url.PathEscape(id), nil, nil); err != nil {
		http.Redirect(w, r, "/?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, safeReturnTo(r.FormValue("return_to"), "/"), http.StatusSeeOther)
}

func (s *webServer) memberIndex(w http.ResponseWriter, r *http.Request) {
	var dashboard store.Dashboard
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/dashboard", nil, &dashboard); err != nil {
		if errors.Is(err, errUnauthorized) {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		s.render(w, http.StatusBadGateway, pageData{Title: "Users", Error: err.Error(), Now: time.Now()})
		return
	}
	s.render(w, http.StatusOK, pageData{
		Title:       "Users",
		Error:       userFacingError(r.URL.Query().Get("error")),
		Dashboard:   dashboard,
		Members:     dashboard.Members,
		MemberIndex: true,
		Now:         time.Now(),
	})
}

func (s *webServer) settings(w http.ResponseWriter, r *http.Request) {
	s.renderSettings(w, r, "", store.APITokenWithSecret{})
}

func (s *webServer) createAPIToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.renderSettings(w, r, "The form could not be read.", store.APITokenWithSecret{})
		return
	}
	payload := map[string]any{
		"name":  r.FormValue("name"),
		"scope": defaultValue(r.FormValue("scope"), "read"),
	}
	var created store.APITokenWithSecret
	if err := s.apiJSON(r, http.MethodPost, "/api/v1/api-tokens", payload, &created); err != nil {
		s.renderSettings(w, r, err.Error(), store.APITokenWithSecret{})
		return
	}
	s.renderSettings(w, r, "", created)
}

func (s *webServer) revokeAPIToken(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.apiJSON(r, http.MethodDelete, "/api/v1/api-tokens/"+url.PathEscape(id), nil, nil); err != nil {
		http.Redirect(w, r, "/settings?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func (s *webServer) renderSettings(w http.ResponseWriter, r *http.Request, pageError string, created store.APITokenWithSecret) {
	var dashboard store.Dashboard
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/dashboard", nil, &dashboard); err != nil {
		if errors.Is(err, errUnauthorized) {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		s.render(w, http.StatusBadGateway, pageData{Title: "Settings", Error: err.Error(), Now: time.Now()})
		return
	}
	var tokens []store.APIToken
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/api-tokens", nil, &tokens); err != nil {
		if pageError == "" {
			pageError = err.Error()
		}
	}
	if pageError == "" {
		pageError = userFacingError(r.URL.Query().Get("error"))
	}
	s.render(w, http.StatusOK, pageData{
		Title:           "Settings",
		Error:           pageError,
		Dashboard:       dashboard,
		APITokens:       tokens,
		CreatedAPIToken: created,
		SettingsPage:    true,
		Now:             time.Now(),
	})
}

func (s *webServer) projectIndex(w http.ResponseWriter, r *http.Request) {
	var dashboard store.Dashboard
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/dashboard", nil, &dashboard); err != nil {
		if errors.Is(err, errUnauthorized) {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		s.render(w, http.StatusBadGateway, pageData{Title: "Projects", Error: err.Error(), Now: time.Now()})
		return
	}
	s.render(w, http.StatusOK, pageData{
		Title:        "Projects",
		Error:        userFacingError(r.URL.Query().Get("error")),
		Dashboard:    dashboard,
		Projects:     dashboard.Projects,
		ProjectIndex: true,
		DueFilter:    dueFilterFromRequest(r),
		Now:          time.Now(),
	})
}

func (s *webServer) createProject(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/?error=form", http.StatusSeeOther)
		return
	}
	payload := map[string]any{
		"title":       r.FormValue("title"),
		"description": r.FormValue("description"),
		"priority":    defaultValue(r.FormValue("priority"), "normal"),
	}
	if due := strings.TrimSpace(r.FormValue("due_date")); due != "" {
		payload["due_date"] = due + "T00:00:00Z"
	}
	_ = s.apiJSON(r, http.MethodPost, "/api/v1/projects", payload, nil)
	http.Redirect(w, r, safeReturnTo(r.FormValue("return_to"), "/"), http.StatusSeeOther)
}

func (s *webServer) projectDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var project store.Project
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/projects/"+url.PathEscape(id), nil, &project); err != nil {
		s.render(w, http.StatusNotFound, pageData{Title: "Project", Error: err.Error(), Now: time.Now()})
		return
	}

	var dashboard store.Dashboard
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/dashboard", nil, &dashboard); err != nil {
		s.render(w, http.StatusBadGateway, pageData{Title: project.Title, Error: err.Error(), Project: project, Now: time.Now()})
		return
	}

	var folders []store.ProjectFolder
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/projects/"+url.PathEscape(id)+"/folders", nil, &folders); err != nil {
		s.render(w, http.StatusBadGateway, pageData{Title: project.Title, Error: err.Error(), Project: project, Dashboard: dashboard, Now: time.Now()})
		return
	}
	var documents []store.Document
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/documents", nil, &documents); err != nil {
		s.render(w, http.StatusBadGateway, pageData{Title: project.Title, Error: err.Error(), Project: project, Dashboard: dashboard, Now: time.Now()})
		return
	}
	var contacts []store.Contact
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/contacts", nil, &contacts); err != nil {
		s.render(w, http.StatusBadGateway, pageData{Title: project.Title, Error: err.Error(), Project: project, Dashboard: dashboard, Now: time.Now()})
		return
	}
	var assets []store.Asset
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/assets", nil, &assets); err != nil {
		s.render(w, http.StatusBadGateway, pageData{Title: project.Title, Error: err.Error(), Project: project, Dashboard: dashboard, Now: time.Now()})
		return
	}
	var relatedDocs []store.RelatedDocument
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/related-documents?entity_type=project&entity_id="+url.QueryEscape(id), nil, &relatedDocs); err != nil {
		s.render(w, http.StatusBadGateway, pageData{Title: project.Title, Error: err.Error(), Project: project, Dashboard: dashboard, Now: time.Now()})
		return
	}
	var relatedContacts []store.RelatedContact
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/related-contacts?entity_type=project&entity_id="+url.QueryEscape(id), nil, &relatedContacts); err != nil {
		s.render(w, http.StatusBadGateway, pageData{Title: project.Title, Error: err.Error(), Project: project, Dashboard: dashboard, Now: time.Now()})
		return
	}
	var relatedAssets []store.RelatedAsset
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/related-assets?entity_type=project&entity_id="+url.QueryEscape(id), nil, &relatedAssets); err != nil {
		s.render(w, http.StatusBadGateway, pageData{Title: project.Title, Error: err.Error(), Project: project, Dashboard: dashboard, Now: time.Now()})
		return
	}
	tasks := projectTasks(dashboard.Tasks, project.ID)
	taskDocuments := map[int64][]store.RelatedDocument{}
	taskContacts := map[int64][]store.RelatedContact{}
	taskAssets := map[int64][]store.RelatedAsset{}
	for _, task := range tasks {
		var docs []store.RelatedDocument
		path := "/api/v1/related-documents?entity_type=task&entity_id=" + url.QueryEscape(strconv.FormatInt(task.ID, 10))
		if err := s.apiJSON(r, http.MethodGet, path, nil, &docs); err != nil {
			s.render(w, http.StatusBadGateway, pageData{Title: project.Title, Error: err.Error(), Project: project, Dashboard: dashboard, Now: time.Now()})
			return
		}
		taskDocuments[task.ID] = docs
		var linkedContacts []store.RelatedContact
		contactsPath := "/api/v1/related-contacts?entity_type=task&entity_id=" + url.QueryEscape(strconv.FormatInt(task.ID, 10))
		if err := s.apiJSON(r, http.MethodGet, contactsPath, nil, &linkedContacts); err != nil {
			s.render(w, http.StatusBadGateway, pageData{Title: project.Title, Error: err.Error(), Project: project, Dashboard: dashboard, Now: time.Now()})
			return
		}
		taskContacts[task.ID] = linkedContacts
		var linkedAssets []store.RelatedAsset
		assetsPath := "/api/v1/related-assets?entity_type=task&entity_id=" + url.QueryEscape(strconv.FormatInt(task.ID, 10))
		if err := s.apiJSON(r, http.MethodGet, assetsPath, nil, &linkedAssets); err != nil {
			s.render(w, http.StatusBadGateway, pageData{Title: project.Title, Error: err.Error(), Project: project, Dashboard: dashboard, Now: time.Now()})
			return
		}
		taskAssets[task.ID] = linkedAssets
	}

	s.render(w, http.StatusOK, pageData{
		Title:           project.Title,
		Error:           userFacingError(r.URL.Query().Get("error")),
		Project:         project,
		Dashboard:       dashboard,
		Tasks:           tasks,
		ProjectFolders:  folders,
		Contacts:        contacts,
		Documents:       documents,
		Assets:          assets,
		RelatedDocs:     relatedDocs,
		RelatedContacts: relatedContacts,
		RelatedAssets:   relatedAssets,
		TaskDocuments:   taskDocuments,
		TaskContacts:    taskContacts,
		TaskAssets:      taskAssets,
		Now:             time.Now(),
	})
}

func (s *webServer) updateProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/projects/"+url.PathEscape(id)+"?error=form", http.StatusSeeOther)
		return
	}

	payload := map[string]any{
		"title":       r.FormValue("title"),
		"description": r.FormValue("description"),
		"status":      defaultValue(r.FormValue("status"), "active"),
		"priority":    defaultValue(r.FormValue("priority"), "normal"),
	}
	if due := strings.TrimSpace(r.FormValue("due_date")); due != "" {
		payload["due_date"] = due + "T00:00:00Z"
	}
	if err := s.apiJSON(r, http.MethodPatch, "/api/v1/projects/"+url.PathEscape(id), payload, nil); err != nil {
		http.Redirect(w, r, "/projects/"+url.PathEscape(id)+"?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, safeReturnTo(r.FormValue("return_to"), "/projects/"+url.PathEscape(id)), http.StatusSeeOther)
}

func (s *webServer) archiveProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_ = s.apiJSON(r, http.MethodPost, "/api/v1/projects/"+url.PathEscape(id)+"/archive", nil, nil)
	http.Redirect(w, r, "/projects", http.StatusSeeOther)
}

func (s *webServer) createProjectFolder(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/projects/"+url.PathEscape(projectID)+"?error=form", http.StatusSeeOther)
		return
	}
	payload := map[string]any{
		"title": r.FormValue("title"),
	}
	if err := s.apiJSON(r, http.MethodPost, "/api/v1/projects/"+url.PathEscape(projectID)+"/folders", payload, nil); err != nil {
		http.Redirect(w, r, "/projects/"+url.PathEscape(projectID)+"?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/projects/"+url.PathEscape(projectID), http.StatusSeeOther)
}

func (s *webServer) updateProjectFolder(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	folderID := r.PathValue("folderID")
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/projects/"+url.PathEscape(projectID)+"?error=form", http.StatusSeeOther)
		return
	}
	payload := map[string]any{
		"title": r.FormValue("title"),
	}
	if err := s.apiJSON(r, http.MethodPatch, "/api/v1/project-folders/"+url.PathEscape(folderID), payload, nil); err != nil {
		http.Redirect(w, r, "/projects/"+url.PathEscape(projectID)+"?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/projects/"+url.PathEscape(projectID), http.StatusSeeOther)
}

func (s *webServer) archiveProjectFolder(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	folderID := r.PathValue("folderID")
	_ = s.apiJSON(r, http.MethodPost, "/api/v1/project-folders/"+url.PathEscape(folderID)+"/archive", nil, nil)
	http.Redirect(w, r, "/projects/"+url.PathEscape(projectID), http.StatusSeeOther)
}

func (s *webServer) createProjectTask(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/projects/"+url.PathEscape(projectID)+"?error=form", http.StatusSeeOther)
		return
	}
	payload := taskPayloadFromForm(r)
	if id, ok := optionalInt64(projectID); ok {
		payload["project_id"] = id
	}
	if err := s.apiJSON(r, http.MethodPost, "/api/v1/tasks", payload, nil); err != nil {
		http.Redirect(w, r, "/projects/"+url.PathEscape(projectID)+"?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/projects/"+url.PathEscape(projectID), http.StatusSeeOther)
}

func (s *webServer) updateProjectTask(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	taskID := r.PathValue("taskID")
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/projects/"+url.PathEscape(projectID)+"?error=form", http.StatusSeeOther)
		return
	}
	payload := taskPayloadFromForm(r)
	if id, ok := optionalInt64(projectID); ok {
		payload["project_id"] = id
	}
	if err := s.apiJSON(r, http.MethodPatch, "/api/v1/tasks/"+url.PathEscape(taskID), payload, nil); err != nil {
		http.Redirect(w, r, "/projects/"+url.PathEscape(projectID)+"?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/projects/"+url.PathEscape(projectID), http.StatusSeeOther)
}

func (s *webServer) completeProjectTask(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	taskID := r.PathValue("taskID")
	_ = s.apiJSON(r, http.MethodPatch, "/api/v1/tasks/"+url.PathEscape(taskID)+"/complete", nil, nil)
	http.Redirect(w, r, "/projects/"+url.PathEscape(projectID), http.StatusSeeOther)
}

func (s *webServer) reopenProjectTask(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	taskID := r.PathValue("taskID")
	_ = s.apiJSON(r, http.MethodPatch, "/api/v1/tasks/"+url.PathEscape(taskID)+"/reopen", nil, nil)
	http.Redirect(w, r, "/projects/"+url.PathEscape(projectID), http.StatusSeeOther)
}

func (s *webServer) archiveProjectTask(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	taskID := r.PathValue("taskID")
	_ = s.apiJSON(r, http.MethodPost, "/api/v1/tasks/"+url.PathEscape(taskID)+"/archive", nil, nil)
	http.Redirect(w, r, "/projects/"+url.PathEscape(projectID), http.StatusSeeOther)
}

func (s *webServer) createTask(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/?error=form", http.StatusSeeOther)
		return
	}
	payload := taskPayloadFromForm(r)
	_ = s.apiJSON(r, http.MethodPost, "/api/v1/tasks", payload, nil)
	http.Redirect(w, r, safeReturnTo(r.FormValue("return_to"), "/"), http.StatusSeeOther)
}

func (s *webServer) taskIndex(w http.ResponseWriter, r *http.Request) {
	var dashboard store.Dashboard
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/dashboard", nil, &dashboard); err != nil {
		if errors.Is(err, errUnauthorized) {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		s.render(w, http.StatusBadGateway, pageData{Title: "Tasks", Error: err.Error(), Now: time.Now()})
		return
	}
	s.render(w, http.StatusOK, pageData{
		Title:     "Tasks",
		Error:     userFacingError(r.URL.Query().Get("error")),
		Dashboard: dashboard,
		Tasks:     dashboard.Tasks,
		Projects:  dashboard.Projects,
		Members:   dashboard.Members,
		TaskIndex: true,
		DueFilter: dueFilterFromRequest(r),
		Now:       time.Now(),
	})
}

func (s *webServer) taskDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var task store.Task
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/tasks/"+url.PathEscape(id), nil, &task); err != nil {
		s.render(w, http.StatusNotFound, pageData{Title: "Task", Error: err.Error(), Now: time.Now()})
		return
	}

	var dashboard store.Dashboard
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/dashboard", nil, &dashboard); err != nil {
		s.render(w, http.StatusBadGateway, pageData{Title: task.Title, Error: err.Error(), Task: task, Now: time.Now()})
		return
	}
	var documents []store.Document
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/documents", nil, &documents); err != nil {
		s.render(w, http.StatusBadGateway, pageData{Title: task.Title, Error: err.Error(), Task: task, Dashboard: dashboard, Now: time.Now()})
		return
	}
	var contacts []store.Contact
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/contacts", nil, &contacts); err != nil {
		s.render(w, http.StatusBadGateway, pageData{Title: task.Title, Error: err.Error(), Task: task, Dashboard: dashboard, Now: time.Now()})
		return
	}
	var assets []store.Asset
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/assets", nil, &assets); err != nil {
		s.render(w, http.StatusBadGateway, pageData{Title: task.Title, Error: err.Error(), Task: task, Dashboard: dashboard, Now: time.Now()})
		return
	}
	var relatedDocs []store.RelatedDocument
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/related-documents?entity_type=task&entity_id="+url.QueryEscape(id), nil, &relatedDocs); err != nil {
		s.render(w, http.StatusBadGateway, pageData{Title: task.Title, Error: err.Error(), Task: task, Dashboard: dashboard, Now: time.Now()})
		return
	}
	var relatedContacts []store.RelatedContact
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/related-contacts?entity_type=task&entity_id="+url.QueryEscape(id), nil, &relatedContacts); err != nil {
		s.render(w, http.StatusBadGateway, pageData{Title: task.Title, Error: err.Error(), Task: task, Dashboard: dashboard, Now: time.Now()})
		return
	}
	var relatedAssets []store.RelatedAsset
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/related-assets?entity_type=task&entity_id="+url.QueryEscape(id), nil, &relatedAssets); err != nil {
		s.render(w, http.StatusBadGateway, pageData{Title: task.Title, Error: err.Error(), Task: task, Dashboard: dashboard, Now: time.Now()})
		return
	}

	s.render(w, http.StatusOK, pageData{
		Title:           task.Title,
		Error:           userFacingError(r.URL.Query().Get("error")),
		Task:            task,
		Dashboard:       dashboard,
		Projects:        dashboard.Projects,
		Members:         dashboard.Members,
		Contacts:        contacts,
		Documents:       documents,
		Assets:          assets,
		RelatedContacts: relatedContacts,
		RelatedDocs:     relatedDocs,
		RelatedAssets:   relatedAssets,
		Now:             time.Now(),
	})
}

func (s *webServer) updateTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/tasks/"+url.PathEscape(id)+"?error=form", http.StatusSeeOther)
		return
	}
	payload := taskPayloadFromForm(r)
	if err := s.apiJSON(r, http.MethodPatch, "/api/v1/tasks/"+url.PathEscape(id), payload, nil); err != nil {
		http.Redirect(w, r, "/tasks/"+url.PathEscape(id)+"?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, safeReturnTo(r.FormValue("return_to"), "/tasks/"+url.PathEscape(id)), http.StatusSeeOther)
}

func (s *webServer) archiveTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_ = s.apiJSON(r, http.MethodPost, "/api/v1/tasks/"+url.PathEscape(id)+"/archive", nil, nil)
	http.Redirect(w, r, "/tasks", http.StatusSeeOther)
}

func (s *webServer) createEvent(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/?error=form", http.StatusSeeOther)
		return
	}
	payload := map[string]any{
		"title":       r.FormValue("title"),
		"description": r.FormValue("description"),
		"location":    r.FormValue("location"),
		"starts_at":   r.FormValue("starts_at") + ":00Z",
		"ends_at":     r.FormValue("ends_at") + ":00Z",
	}
	_ = s.apiJSON(r, http.MethodPost, "/api/v1/events", payload, nil)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *webServer) eventDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var event store.Event
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/events/"+url.PathEscape(id), nil, &event); err != nil {
		s.render(w, http.StatusNotFound, pageData{Title: "Appointment", Error: err.Error(), Now: time.Now()})
		return
	}
	var dashboard store.Dashboard
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/dashboard", nil, &dashboard); err != nil {
		s.render(w, http.StatusBadGateway, pageData{Title: event.Title, Error: err.Error(), Event: event, Now: time.Now()})
		return
	}
	s.render(w, http.StatusOK, pageData{
		Title:     event.Title,
		Error:     userFacingError(r.URL.Query().Get("error")),
		Event:     event,
		Dashboard: dashboard,
		Now:       time.Now(),
	})
}

func (s *webServer) updateEvent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/events/"+url.PathEscape(id)+"?error=form", http.StatusSeeOther)
		return
	}
	payload := map[string]any{
		"title":       r.FormValue("title"),
		"description": r.FormValue("description"),
		"location":    r.FormValue("location"),
		"starts_at":   r.FormValue("starts_at") + ":00Z",
		"ends_at":     r.FormValue("ends_at") + ":00Z",
	}
	if err := s.apiJSON(r, http.MethodPatch, "/api/v1/events/"+url.PathEscape(id), payload, nil); err != nil {
		http.Redirect(w, r, "/events/"+url.PathEscape(id)+"?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/events/"+url.PathEscape(id), http.StatusSeeOther)
}

func (s *webServer) deleteEvent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_ = s.apiJSON(r, http.MethodDelete, "/api/v1/events/"+url.PathEscape(id), nil, nil)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *webServer) createRoutine(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/?error=form", http.StatusSeeOther)
		return
	}
	payload := routinePayloadFromForm(r)
	if err := s.apiJSON(r, http.MethodPost, "/api/v1/routines", payload, nil); err != nil {
		http.Redirect(w, r, "/?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, safeReturnTo(r.FormValue("return_to"), "/"), http.StatusSeeOther)
}

func (s *webServer) routineIndex(w http.ResponseWriter, r *http.Request) {
	var dashboard store.Dashboard
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/dashboard", nil, &dashboard); err != nil {
		if errors.Is(err, errUnauthorized) {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		s.render(w, http.StatusBadGateway, pageData{Title: "Routines", Error: err.Error(), Now: time.Now()})
		return
	}
	s.render(w, http.StatusOK, pageData{
		Title:        "Routines",
		Error:        userFacingError(r.URL.Query().Get("error")),
		Dashboard:    dashboard,
		Routines:     dashboard.Routines,
		Members:      dashboard.Members,
		RoutineIndex: true,
		Now:          time.Now(),
	})
}

func (s *webServer) routineDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var routine store.Routine
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/routines/"+url.PathEscape(id), nil, &routine); err != nil {
		s.render(w, http.StatusNotFound, pageData{Title: "Routine", Error: err.Error(), Now: time.Now()})
		return
	}
	var dashboard store.Dashboard
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/dashboard", nil, &dashboard); err != nil {
		s.render(w, http.StatusBadGateway, pageData{Title: routine.Title, Error: err.Error(), Routine: routine, Now: time.Now()})
		return
	}
	s.render(w, http.StatusOK, pageData{
		Title:     routine.Title,
		Error:     userFacingError(r.URL.Query().Get("error")),
		Routine:   routine,
		Dashboard: dashboard,
		Members:   dashboard.Members,
		Now:       time.Now(),
	})
}

func (s *webServer) updateRoutine(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/routines/"+url.PathEscape(id)+"?error=form", http.StatusSeeOther)
		return
	}
	payload := routinePayloadFromForm(r)
	if err := s.apiJSON(r, http.MethodPatch, "/api/v1/routines/"+url.PathEscape(id), payload, nil); err != nil {
		http.Redirect(w, r, "/routines/"+url.PathEscape(id)+"?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/routines/"+url.PathEscape(id), http.StatusSeeOther)
}

func (s *webServer) archiveRoutine(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_ = s.apiJSON(r, http.MethodPost, "/api/v1/routines/"+url.PathEscape(id)+"/archive", nil, nil)
	http.Redirect(w, r, "/routines", http.StatusSeeOther)
}

func (s *webServer) generateRoutineTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.apiJSON(r, http.MethodPost, "/api/v1/routines/"+url.PathEscape(id)+"/generate-task", nil, nil); err != nil {
		http.Redirect(w, r, "/routines/"+url.PathEscape(id)+"?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/routines/"+url.PathEscape(id), http.StatusSeeOther)
}

func (s *webServer) listIndex(w http.ResponseWriter, r *http.Request) {
	var dashboard store.Dashboard
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/dashboard", nil, &dashboard); err != nil {
		if errors.Is(err, errUnauthorized) {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		s.render(w, http.StatusBadGateway, pageData{Title: "Lists", Error: err.Error(), Now: time.Now()})
		return
	}
	var lists []store.HouseholdList
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/lists", nil, &lists); err != nil {
		s.render(w, http.StatusBadGateway, pageData{Title: "Lists", Error: err.Error(), Dashboard: dashboard, ListIndex: true, Now: time.Now()})
		return
	}
	s.render(w, http.StatusOK, pageData{
		Title:     "Lists",
		Error:     userFacingError(r.URL.Query().Get("error")),
		Dashboard: dashboard,
		Lists:     lists,
		ListIndex: true,
		Now:       time.Now(),
	})
}

func (s *webServer) createList(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/lists?error=form", http.StatusSeeOther)
		return
	}
	payload := listPayloadFromForm(r)
	var created store.HouseholdList
	if err := s.apiJSON(r, http.MethodPost, "/api/v1/lists", payload, &created); err != nil {
		http.Redirect(w, r, "/lists?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/lists/"+strconv.FormatInt(created.ID, 10), http.StatusSeeOther)
}

func (s *webServer) listDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var list store.HouseholdList
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/lists/"+url.PathEscape(id), nil, &list); err != nil {
		s.render(w, http.StatusNotFound, pageData{Title: "List", Error: err.Error(), Now: time.Now()})
		return
	}
	var dashboard store.Dashboard
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/dashboard", nil, &dashboard); err != nil {
		s.render(w, http.StatusBadGateway, pageData{Title: list.Title, Error: err.Error(), List: list, Now: time.Now()})
		return
	}
	var items []store.ListItem
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/lists/"+url.PathEscape(id)+"/items", nil, &items); err != nil {
		s.render(w, http.StatusBadGateway, pageData{Title: list.Title, Error: err.Error(), Dashboard: dashboard, List: list, Now: time.Now()})
		return
	}
	s.render(w, http.StatusOK, pageData{
		Title:     list.Title,
		Error:     userFacingError(r.URL.Query().Get("error")),
		Dashboard: dashboard,
		List:      list,
		ListItems: items,
		Members:   dashboard.Members,
		Now:       time.Now(),
	})
}

func (s *webServer) updateList(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/lists/"+url.PathEscape(id)+"?error=form", http.StatusSeeOther)
		return
	}
	payload := listPayloadFromForm(r)
	if err := s.apiJSON(r, http.MethodPatch, "/api/v1/lists/"+url.PathEscape(id), payload, nil); err != nil {
		http.Redirect(w, r, "/lists/"+url.PathEscape(id)+"?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/lists/"+url.PathEscape(id), http.StatusSeeOther)
}

func (s *webServer) archiveList(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_ = s.apiJSON(r, http.MethodPost, "/api/v1/lists/"+url.PathEscape(id)+"/archive", nil, nil)
	http.Redirect(w, r, "/lists", http.StatusSeeOther)
}

func (s *webServer) createListItem(w http.ResponseWriter, r *http.Request) {
	listID := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/lists/"+url.PathEscape(listID)+"?error=form", http.StatusSeeOther)
		return
	}
	payload := listItemPayloadFromForm(r)
	if err := s.apiJSON(r, http.MethodPost, "/api/v1/lists/"+url.PathEscape(listID)+"/items", payload, nil); err != nil {
		http.Redirect(w, r, "/lists/"+url.PathEscape(listID)+"?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/lists/"+url.PathEscape(listID), http.StatusSeeOther)
}

func (s *webServer) updateListItem(w http.ResponseWriter, r *http.Request) {
	listID := r.PathValue("listID")
	itemID := r.PathValue("itemID")
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/lists/"+url.PathEscape(listID)+"?error=form", http.StatusSeeOther)
		return
	}
	payload := listItemPayloadFromForm(r)
	if err := s.apiJSON(r, http.MethodPatch, "/api/v1/list-items/"+url.PathEscape(itemID), payload, nil); err != nil {
		http.Redirect(w, r, "/lists/"+url.PathEscape(listID)+"?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/lists/"+url.PathEscape(listID), http.StatusSeeOther)
}

func (s *webServer) completeListItem(w http.ResponseWriter, r *http.Request) {
	listID := r.PathValue("listID")
	itemID := r.PathValue("itemID")
	_ = r.ParseForm()
	_ = s.apiJSON(r, http.MethodPatch, "/api/v1/list-items/"+url.PathEscape(itemID)+"/complete", nil, nil)
	http.Redirect(w, r, safeReturnTo(r.FormValue("return_to"), "/lists/"+url.PathEscape(listID)), http.StatusSeeOther)
}

func (s *webServer) reopenListItem(w http.ResponseWriter, r *http.Request) {
	listID := r.PathValue("listID")
	itemID := r.PathValue("itemID")
	_ = r.ParseForm()
	_ = s.apiJSON(r, http.MethodPatch, "/api/v1/list-items/"+url.PathEscape(itemID)+"/reopen", nil, nil)
	http.Redirect(w, r, safeReturnTo(r.FormValue("return_to"), "/lists/"+url.PathEscape(listID)), http.StatusSeeOther)
}

func (s *webServer) archiveListItem(w http.ResponseWriter, r *http.Request) {
	listID := r.PathValue("listID")
	itemID := r.PathValue("itemID")
	_ = s.apiJSON(r, http.MethodPost, "/api/v1/list-items/"+url.PathEscape(itemID)+"/archive", nil, nil)
	http.Redirect(w, r, "/lists/"+url.PathEscape(listID), http.StatusSeeOther)
}

func (s *webServer) contactIndex(w http.ResponseWriter, r *http.Request) {
	var dashboard store.Dashboard
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/dashboard", nil, &dashboard); err != nil {
		if errors.Is(err, errUnauthorized) {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		s.render(w, http.StatusBadGateway, pageData{Title: "Contacts", Error: err.Error(), Now: time.Now()})
		return
	}
	var contacts []store.Contact
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/contacts", nil, &contacts); err != nil {
		s.render(w, http.StatusBadGateway, pageData{Title: "Contacts", Error: err.Error(), Dashboard: dashboard, ContactIndex: true, Now: time.Now()})
		return
	}
	s.render(w, http.StatusOK, pageData{
		Title:        "Contacts",
		Error:        userFacingError(r.URL.Query().Get("error")),
		Dashboard:    dashboard,
		Contacts:     contacts,
		ContactIndex: true,
		Now:          time.Now(),
	})
}

func (s *webServer) createContact(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/contacts?error=form", http.StatusSeeOther)
		return
	}
	var created store.Contact
	if err := s.apiJSON(r, http.MethodPost, "/api/v1/contacts", contactPayloadFromForm(r), &created); err != nil {
		http.Redirect(w, r, "/contacts?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/contacts/"+strconv.FormatInt(created.ID, 10), http.StatusSeeOther)
}

func (s *webServer) contactDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var contact store.Contact
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/contacts/"+url.PathEscape(id), nil, &contact); err != nil {
		s.render(w, http.StatusNotFound, pageData{Title: "Contact", Error: err.Error(), Now: time.Now()})
		return
	}
	var dashboard store.Dashboard
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/dashboard", nil, &dashboard); err != nil {
		s.render(w, http.StatusBadGateway, pageData{Title: contact.Name, Error: err.Error(), Contact: contact, Now: time.Now()})
		return
	}
	s.render(w, http.StatusOK, pageData{
		Title:     contact.Name,
		Error:     userFacingError(r.URL.Query().Get("error")),
		Dashboard: dashboard,
		Contact:   contact,
		Now:       time.Now(),
	})
}

func (s *webServer) updateContact(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/contacts/"+url.PathEscape(id)+"?error=form", http.StatusSeeOther)
		return
	}
	if err := s.apiJSON(r, http.MethodPatch, "/api/v1/contacts/"+url.PathEscape(id), contactPayloadFromForm(r), nil); err != nil {
		http.Redirect(w, r, "/contacts/"+url.PathEscape(id)+"?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/contacts/"+url.PathEscape(id), http.StatusSeeOther)
}

func (s *webServer) archiveContact(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_ = s.apiJSON(r, http.MethodPost, "/api/v1/contacts/"+url.PathEscape(id)+"/archive", nil, nil)
	http.Redirect(w, r, "/contacts", http.StatusSeeOther)
}

func (s *webServer) assetIndex(w http.ResponseWriter, r *http.Request) {
	var dashboard store.Dashboard
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/dashboard", nil, &dashboard); err != nil {
		if errors.Is(err, errUnauthorized) {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		s.render(w, http.StatusBadGateway, pageData{Title: "Assets", Error: err.Error(), Now: time.Now()})
		return
	}
	var assets []store.Asset
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/assets", nil, &assets); err != nil {
		s.render(w, http.StatusBadGateway, pageData{Title: "Assets", Error: err.Error(), Dashboard: dashboard, Now: time.Now()})
		return
	}
	s.render(w, http.StatusOK, pageData{
		Title:      "Assets",
		Error:      userFacingError(r.URL.Query().Get("error")),
		Dashboard:  dashboard,
		Assets:     assets,
		AssetIndex: true,
		Now:        time.Now(),
	})
}

func (s *webServer) createAsset(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/assets?error=form", http.StatusSeeOther)
		return
	}
	var created store.Asset
	if err := s.apiJSON(r, http.MethodPost, "/api/v1/assets", assetPayloadFromForm(r), &created); err != nil {
		http.Redirect(w, r, "/assets?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/assets/"+strconv.FormatInt(created.ID, 10), http.StatusSeeOther)
}

func (s *webServer) assetDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var asset store.Asset
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/assets/"+url.PathEscape(id), nil, &asset); err != nil {
		s.render(w, http.StatusNotFound, pageData{Title: "Asset", Error: err.Error(), Now: time.Now()})
		return
	}
	var dashboard store.Dashboard
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/dashboard", nil, &dashboard); err != nil {
		s.render(w, http.StatusBadGateway, pageData{Title: asset.Name, Error: err.Error(), Asset: asset, Now: time.Now()})
		return
	}
	var documents []store.Document
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/documents", nil, &documents); err != nil {
		s.render(w, http.StatusBadGateway, pageData{Title: asset.Name, Error: err.Error(), Asset: asset, Dashboard: dashboard, Now: time.Now()})
		return
	}
	var contacts []store.Contact
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/contacts", nil, &contacts); err != nil {
		s.render(w, http.StatusBadGateway, pageData{Title: asset.Name, Error: err.Error(), Asset: asset, Dashboard: dashboard, Now: time.Now()})
		return
	}
	var relatedDocs []store.RelatedDocument
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/related-documents?entity_type=asset&entity_id="+url.QueryEscape(id), nil, &relatedDocs); err != nil {
		s.render(w, http.StatusBadGateway, pageData{Title: asset.Name, Error: err.Error(), Asset: asset, Dashboard: dashboard, Now: time.Now()})
		return
	}
	var relatedContacts []store.RelatedContact
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/related-contacts?entity_type=asset&entity_id="+url.QueryEscape(id), nil, &relatedContacts); err != nil {
		s.render(w, http.StatusBadGateway, pageData{Title: asset.Name, Error: err.Error(), Asset: asset, Dashboard: dashboard, Now: time.Now()})
		return
	}
	var maintenanceItems []store.AssetMaintenanceItem
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/assets/"+url.PathEscape(id)+"/maintenance", nil, &maintenanceItems); err != nil {
		s.render(w, http.StatusBadGateway, pageData{Title: asset.Name, Error: err.Error(), Asset: asset, Dashboard: dashboard, Now: time.Now()})
		return
	}
	s.render(w, http.StatusOK, pageData{
		Title:                 asset.Name,
		Error:                 userFacingError(r.URL.Query().Get("error")),
		Dashboard:             dashboard,
		Asset:                 asset,
		Contacts:              contacts,
		Documents:             documents,
		AssetMaintenanceItems: maintenanceItems,
		RelatedDocs:           relatedDocs,
		RelatedContacts:       relatedContacts,
		Tasks:                 dashboard.Tasks,
		Now:                   time.Now(),
	})
}

func (s *webServer) updateAsset(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/assets/"+url.PathEscape(id)+"?error=form", http.StatusSeeOther)
		return
	}
	if err := s.apiJSON(r, http.MethodPatch, "/api/v1/assets/"+url.PathEscape(id), assetPayloadFromForm(r), nil); err != nil {
		http.Redirect(w, r, "/assets/"+url.PathEscape(id)+"?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/assets/"+url.PathEscape(id), http.StatusSeeOther)
}

func (s *webServer) archiveAsset(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_ = s.apiJSON(r, http.MethodPost, "/api/v1/assets/"+url.PathEscape(id)+"/archive", nil, nil)
	http.Redirect(w, r, "/assets", http.StatusSeeOther)
}

func (s *webServer) createAssetMaintenanceItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/assets/"+url.PathEscape(id)+"?error=form", http.StatusSeeOther)
		return
	}
	if err := s.apiJSON(r, http.MethodPost, "/api/v1/assets/"+url.PathEscape(id)+"/maintenance", assetMaintenancePayloadFromForm(r), nil); err != nil {
		http.Redirect(w, r, "/assets/"+url.PathEscape(id)+"?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/assets/"+url.PathEscape(id), http.StatusSeeOther)
}

func (s *webServer) updateAssetMaintenanceItem(w http.ResponseWriter, r *http.Request) {
	assetID := r.PathValue("assetID")
	itemID := r.PathValue("itemID")
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/assets/"+url.PathEscape(assetID)+"?error=form", http.StatusSeeOther)
		return
	}
	if err := s.apiJSON(r, http.MethodPatch, "/api/v1/asset-maintenance/"+url.PathEscape(itemID), assetMaintenancePayloadFromForm(r), nil); err != nil {
		http.Redirect(w, r, "/assets/"+url.PathEscape(assetID)+"?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/assets/"+url.PathEscape(assetID), http.StatusSeeOther)
}

func (s *webServer) archiveAssetMaintenanceItem(w http.ResponseWriter, r *http.Request) {
	assetID := r.PathValue("assetID")
	itemID := r.PathValue("itemID")
	if err := s.apiJSON(r, http.MethodPost, "/api/v1/asset-maintenance/"+url.PathEscape(itemID)+"/archive", nil, nil); err != nil {
		http.Redirect(w, r, "/assets/"+url.PathEscape(assetID)+"?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/assets/"+url.PathEscape(assetID), http.StatusSeeOther)
}

func (s *webServer) generateAssetMaintenanceTask(w http.ResponseWriter, r *http.Request) {
	assetID := r.PathValue("assetID")
	itemID := r.PathValue("itemID")
	if err := s.apiJSON(r, http.MethodPost, "/api/v1/asset-maintenance/"+url.PathEscape(itemID)+"/generate-task", nil, nil); err != nil {
		http.Redirect(w, r, "/assets/"+url.PathEscape(assetID)+"?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/assets/"+url.PathEscape(assetID), http.StatusSeeOther)
}

func (s *webServer) documentIndex(w http.ResponseWriter, r *http.Request) {
	var dashboard store.Dashboard
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/dashboard", nil, &dashboard); err != nil {
		if errors.Is(err, errUnauthorized) {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		s.render(w, http.StatusBadGateway, pageData{Title: "Documents", Error: err.Error(), Now: time.Now()})
		return
	}
	var documents []store.Document
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/documents", nil, &documents); err != nil {
		s.render(w, http.StatusBadGateway, pageData{Title: "Documents", Error: err.Error(), Dashboard: dashboard, DocumentIndex: true, Now: time.Now()})
		return
	}
	s.render(w, http.StatusOK, pageData{
		Title:         "Documents",
		Error:         userFacingError(r.URL.Query().Get("error")),
		Dashboard:     dashboard,
		Documents:     documents,
		DocumentIndex: true,
		Now:           time.Now(),
	})
}

func (s *webServer) createDocument(w http.ResponseWriter, r *http.Request) {
	if err := s.parseDocumentForm(w, r); err != nil {
		http.Redirect(w, r, "/documents?error=form", http.StatusSeeOther)
		return
	}
	var created store.Document
	if err := s.apiDocumentForm(r, http.MethodPost, "/api/v1/documents", &created); err != nil {
		http.Redirect(w, r, "/documents?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/documents/"+strconv.FormatInt(created.ID, 10), http.StatusSeeOther)
}

func (s *webServer) documentDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var document store.Document
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/documents/"+url.PathEscape(id), nil, &document); err != nil {
		s.render(w, http.StatusNotFound, pageData{Title: "Document", Error: err.Error(), Now: time.Now()})
		return
	}
	var dashboard store.Dashboard
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/dashboard", nil, &dashboard); err != nil {
		s.render(w, http.StatusBadGateway, pageData{Title: document.Title, Error: err.Error(), Document: document, Now: time.Now()})
		return
	}
	var relatedItems []store.RelatedItem
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/documents/"+url.PathEscape(id)+"/related-items", nil, &relatedItems); err != nil {
		s.render(w, http.StatusBadGateway, pageData{Title: document.Title, Error: err.Error(), Document: document, Dashboard: dashboard, Now: time.Now()})
		return
	}
	var assets []store.Asset
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/assets", nil, &assets); err != nil {
		s.render(w, http.StatusBadGateway, pageData{Title: document.Title, Error: err.Error(), Document: document, Dashboard: dashboard, Now: time.Now()})
		return
	}
	s.render(w, http.StatusOK, pageData{
		Title:        document.Title,
		Error:        userFacingError(r.URL.Query().Get("error")),
		Dashboard:    dashboard,
		Document:     document,
		RelatedItems: relatedItems,
		Projects:     dashboard.Projects,
		Tasks:        dashboard.Tasks,
		Assets:       assets,
		Now:          time.Now(),
	})
}

func (s *webServer) updateDocument(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.parseDocumentForm(w, r); err != nil {
		http.Redirect(w, r, "/documents/"+url.PathEscape(id)+"?error=form", http.StatusSeeOther)
		return
	}
	if err := s.apiDocumentForm(r, http.MethodPatch, "/api/v1/documents/"+url.PathEscape(id), nil); err != nil {
		http.Redirect(w, r, "/documents/"+url.PathEscape(id)+"?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/documents/"+url.PathEscape(id), http.StatusSeeOther)
}

func (s *webServer) downloadDocument(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, s.cfg.APIInternalURL+"/api/v1/documents/"+url.PathEscape(id)+"/download", nil)
	if err != nil {
		http.Error(w, "failed to build download request", http.StatusInternalServerError)
		return
	}
	if cookie, err := r.Cookie(s.cfg.SessionCookieName); err == nil {
		req.AddCookie(cookie)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		http.Error(w, "failed to download document", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	for _, header := range []string{"Content-Type", "Content-Disposition", "Content-Length", "Last-Modified"} {
		if value := resp.Header.Get(header); value != "" {
			w.Header().Set(header, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func (s *webServer) archiveDocument(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_ = s.apiJSON(r, http.MethodPost, "/api/v1/documents/"+url.PathEscape(id)+"/archive", nil, nil)
	http.Redirect(w, r, "/documents", http.StatusSeeOther)
}

func (s *webServer) linkDocumentRelatedItem(w http.ResponseWriter, r *http.Request) {
	documentID := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/documents/"+url.PathEscape(documentID)+"?error=form", http.StatusSeeOther)
		return
	}
	payload := map[string]any{"entity_type": r.FormValue("entity_type")}
	if entityID, ok := optionalInt64(r.FormValue("entity_id")); ok {
		payload["entity_id"] = entityID
	}
	if err := s.apiJSON(r, http.MethodPost, "/api/v1/documents/"+url.PathEscape(documentID)+"/related-items", payload, nil); err != nil {
		http.Redirect(w, r, "/documents/"+url.PathEscape(documentID)+"?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/documents/"+url.PathEscape(documentID), http.StatusSeeOther)
}

func (s *webServer) unlinkDocumentFromDocumentPage(w http.ResponseWriter, r *http.Request) {
	documentID := r.PathValue("documentID")
	linkID := r.PathValue("linkID")
	_ = s.apiJSON(r, http.MethodDelete, "/api/v1/document-links/"+url.PathEscape(linkID), nil, nil)
	http.Redirect(w, r, "/documents/"+url.PathEscape(documentID), http.StatusSeeOther)
}

func (s *webServer) attachProjectDocument(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if err := s.attachDocument(w, r, "project", projectID); err != nil {
		http.Redirect(w, r, "/projects/"+url.PathEscape(projectID)+"?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/projects/"+url.PathEscape(projectID), http.StatusSeeOther)
}

func (s *webServer) unlinkProjectDocument(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	linkID := r.PathValue("linkID")
	_ = s.apiJSON(r, http.MethodDelete, "/api/v1/document-links/"+url.PathEscape(linkID), nil, nil)
	http.Redirect(w, r, "/projects/"+url.PathEscape(projectID), http.StatusSeeOther)
}

func (s *webServer) attachProjectTaskDocument(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	taskID := r.PathValue("taskID")
	if err := s.attachDocument(w, r, "task", taskID); err != nil {
		http.Redirect(w, r, "/projects/"+url.PathEscape(projectID)+"?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/projects/"+url.PathEscape(projectID), http.StatusSeeOther)
}

func (s *webServer) unlinkProjectTaskDocument(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	linkID := r.PathValue("linkID")
	_ = s.apiJSON(r, http.MethodDelete, "/api/v1/document-links/"+url.PathEscape(linkID), nil, nil)
	http.Redirect(w, r, "/projects/"+url.PathEscape(projectID), http.StatusSeeOther)
}

func (s *webServer) attachProjectTaskContact(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	taskID := r.PathValue("taskID")
	if err := s.attachContact(r, "task", taskID); err != nil {
		http.Redirect(w, r, "/projects/"+url.PathEscape(projectID)+"?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/projects/"+url.PathEscape(projectID), http.StatusSeeOther)
}

func (s *webServer) unlinkProjectTaskContact(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	linkID := r.PathValue("linkID")
	_ = s.apiJSON(r, http.MethodDelete, "/api/v1/contact-links/"+url.PathEscape(linkID), nil, nil)
	http.Redirect(w, r, "/projects/"+url.PathEscape(projectID), http.StatusSeeOther)
}

func (s *webServer) attachProjectTaskAsset(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	taskID := r.PathValue("taskID")
	if err := s.attachAsset(r, "task", taskID); err != nil {
		http.Redirect(w, r, "/projects/"+url.PathEscape(projectID)+"?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/projects/"+url.PathEscape(projectID), http.StatusSeeOther)
}

func (s *webServer) unlinkProjectTaskAsset(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	linkID := r.PathValue("linkID")
	_ = s.apiJSON(r, http.MethodDelete, "/api/v1/asset-links/"+url.PathEscape(linkID), nil, nil)
	http.Redirect(w, r, "/projects/"+url.PathEscape(projectID), http.StatusSeeOther)
}

func (s *webServer) attachTaskDocument(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	if err := s.attachDocument(w, r, "task", taskID); err != nil {
		http.Redirect(w, r, "/tasks/"+url.PathEscape(taskID)+"?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/tasks/"+url.PathEscape(taskID), http.StatusSeeOther)
}

func (s *webServer) unlinkTaskDocument(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("taskID")
	linkID := r.PathValue("linkID")
	_ = s.apiJSON(r, http.MethodDelete, "/api/v1/document-links/"+url.PathEscape(linkID), nil, nil)
	http.Redirect(w, r, "/tasks/"+url.PathEscape(taskID), http.StatusSeeOther)
}

func (s *webServer) attachProjectContact(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if err := s.attachContact(r, "project", projectID); err != nil {
		http.Redirect(w, r, "/projects/"+url.PathEscape(projectID)+"?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/projects/"+url.PathEscape(projectID), http.StatusSeeOther)
}

func (s *webServer) unlinkProjectContact(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	linkID := r.PathValue("linkID")
	_ = s.apiJSON(r, http.MethodDelete, "/api/v1/contact-links/"+url.PathEscape(linkID), nil, nil)
	http.Redirect(w, r, "/projects/"+url.PathEscape(projectID), http.StatusSeeOther)
}

func (s *webServer) attachTaskContact(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	if err := s.attachContact(r, "task", taskID); err != nil {
		http.Redirect(w, r, "/tasks/"+url.PathEscape(taskID)+"?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/tasks/"+url.PathEscape(taskID), http.StatusSeeOther)
}

func (s *webServer) unlinkTaskContact(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("taskID")
	linkID := r.PathValue("linkID")
	_ = s.apiJSON(r, http.MethodDelete, "/api/v1/contact-links/"+url.PathEscape(linkID), nil, nil)
	http.Redirect(w, r, "/tasks/"+url.PathEscape(taskID), http.StatusSeeOther)
}

func (s *webServer) attachProjectAsset(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if err := s.attachAsset(r, "project", projectID); err != nil {
		http.Redirect(w, r, "/projects/"+url.PathEscape(projectID)+"?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/projects/"+url.PathEscape(projectID), http.StatusSeeOther)
}

func (s *webServer) unlinkProjectAsset(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	linkID := r.PathValue("linkID")
	_ = s.apiJSON(r, http.MethodDelete, "/api/v1/asset-links/"+url.PathEscape(linkID), nil, nil)
	http.Redirect(w, r, "/projects/"+url.PathEscape(projectID), http.StatusSeeOther)
}

func (s *webServer) attachTaskAsset(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	if err := s.attachAsset(r, "task", taskID); err != nil {
		http.Redirect(w, r, "/tasks/"+url.PathEscape(taskID)+"?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/tasks/"+url.PathEscape(taskID), http.StatusSeeOther)
}

func (s *webServer) unlinkTaskAsset(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("taskID")
	linkID := r.PathValue("linkID")
	_ = s.apiJSON(r, http.MethodDelete, "/api/v1/asset-links/"+url.PathEscape(linkID), nil, nil)
	http.Redirect(w, r, "/tasks/"+url.PathEscape(taskID), http.StatusSeeOther)
}

func (s *webServer) attachAssetDocument(w http.ResponseWriter, r *http.Request) {
	assetID := r.PathValue("id")
	if err := s.attachDocument(w, r, "asset", assetID); err != nil {
		http.Redirect(w, r, "/assets/"+url.PathEscape(assetID)+"?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/assets/"+url.PathEscape(assetID), http.StatusSeeOther)
}

func (s *webServer) unlinkAssetDocument(w http.ResponseWriter, r *http.Request) {
	assetID := r.PathValue("assetID")
	linkID := r.PathValue("linkID")
	_ = s.apiJSON(r, http.MethodDelete, "/api/v1/document-links/"+url.PathEscape(linkID), nil, nil)
	http.Redirect(w, r, "/assets/"+url.PathEscape(assetID), http.StatusSeeOther)
}

func (s *webServer) attachAssetContact(w http.ResponseWriter, r *http.Request) {
	assetID := r.PathValue("id")
	if err := s.attachContact(r, "asset", assetID); err != nil {
		http.Redirect(w, r, "/assets/"+url.PathEscape(assetID)+"?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/assets/"+url.PathEscape(assetID), http.StatusSeeOther)
}

func (s *webServer) unlinkAssetContact(w http.ResponseWriter, r *http.Request) {
	assetID := r.PathValue("assetID")
	linkID := r.PathValue("linkID")
	_ = s.apiJSON(r, http.MethodDelete, "/api/v1/contact-links/"+url.PathEscape(linkID), nil, nil)
	http.Redirect(w, r, "/assets/"+url.PathEscape(assetID), http.StatusSeeOther)
}

func (s *webServer) attachContact(r *http.Request, entityType, rawEntityID string) error {
	if isMultipart(r) {
		if err := r.ParseMultipartForm(1024 * 1024); err != nil {
			return err
		}
	} else if err := r.ParseForm(); err != nil {
		return err
	}
	entityID, ok := optionalInt64(rawEntityID)
	if !ok {
		return errors.New("invalid related item")
	}
	contactID, ok := optionalInt64(r.FormValue("contact_id"))
	if !ok {
		var created store.Contact
		if err := s.apiJSON(r, http.MethodPost, "/api/v1/contacts", contactPayloadFromForm(r), &created); err != nil {
			return err
		}
		contactID = created.ID
	}
	payload := map[string]any{
		"contact_id":  contactID,
		"entity_type": entityType,
		"entity_id":   entityID,
	}
	return s.apiJSON(r, http.MethodPost, "/api/v1/related-contacts", payload, nil)
}

func (s *webServer) attachAsset(r *http.Request, entityType, rawEntityID string) error {
	if err := r.ParseForm(); err != nil {
		return err
	}
	entityID, ok := optionalInt64(rawEntityID)
	if !ok {
		return errors.New("invalid related item")
	}
	assetID, ok := optionalInt64(r.FormValue("asset_id"))
	if !ok {
		var created store.Asset
		if err := s.apiJSON(r, http.MethodPost, "/api/v1/assets", assetPayloadFromForm(r), &created); err != nil {
			return err
		}
		assetID = created.ID
	}
	payload := map[string]any{
		"asset_id":    assetID,
		"entity_type": entityType,
		"entity_id":   entityID,
	}
	return s.apiJSON(r, http.MethodPost, "/api/v1/related-assets", payload, nil)
}

func (s *webServer) attachDocument(w http.ResponseWriter, r *http.Request, entityType, rawEntityID string) error {
	if err := s.parseDocumentForm(w, r); err != nil {
		return err
	}
	entityID, ok := optionalInt64(rawEntityID)
	if !ok {
		return errors.New("invalid related item")
	}
	documentID, hasDocument := optionalInt64(r.FormValue("document_id"))
	if !hasDocument {
		var created store.Document
		if err := s.apiDocumentForm(r, http.MethodPost, "/api/v1/documents", &created); err != nil {
			return err
		}
		documentID = created.ID
	}
	payload := map[string]any{
		"document_id": documentID,
		"entity_type": entityType,
		"entity_id":   entityID,
	}
	return s.apiJSON(r, http.MethodPost, "/api/v1/related-documents", payload, nil)
}

func (s *webServer) completeTask(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err == nil {
		id := url.PathEscape(r.FormValue("id"))
		_ = s.apiJSON(r, http.MethodPatch, "/api/v1/tasks/"+id+"/complete", nil, nil)
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *webServer) reopenTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_ = s.apiJSON(r, http.MethodPatch, "/api/v1/tasks/"+url.PathEscape(id)+"/reopen", nil, nil)
	http.Redirect(w, r, "/tasks/"+url.PathEscape(id), http.StatusSeeOther)
}

func (s *webServer) logout(w http.ResponseWriter, r *http.Request) {
	_ = s.apiJSON(r, http.MethodPost, "/auth/logout", nil, nil)
	http.SetCookie(w, &http.Cookie{
		Name:     s.cfg.SessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.cfg.Env == "production",
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

var errUnauthorized = errors.New("unauthorized")

func (s *webServer) apiJSON(r *http.Request, method, path string, payload any, target any) error {
	var body io.Reader
	if payload != nil {
		buf, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(r.Context(), method, s.cfg.APIInternalURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie, err := r.Cookie(s.cfg.SessionCookieName); err == nil {
		req.AddCookie(cookie)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return errUnauthorized
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr struct {
			Error string `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err == nil && apiErr.Error != "" {
			return errors.New(apiErr.Error)
		}
		return fmt.Errorf("api returned %s", resp.Status)
	}
	if target == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func (s *webServer) apiDocumentForm(r *http.Request, method, path string, target any) error {
	if isMultipart(r) {
		return s.apiMultipart(r, method, path, documentFieldsFromForm(r), documentFileFromForm(r), target)
	}
	return s.apiJSON(r, method, path, documentPayloadFromForm(r), target)
}

func (s *webServer) apiMultipart(r *http.Request, method, path string, fields map[string]string, fileHeader *multipart.FileHeader, target any) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			return err
		}
	}
	if fileHeader != nil && fileHeader.Filename != "" {
		file, err := fileHeader.Open()
		if err != nil {
			return err
		}
		defer file.Close()

		part, err := writer.CreateFormFile("file", fileHeader.Filename)
		if err != nil {
			return err
		}
		if _, err := io.Copy(part, file); err != nil {
			return err
		}
	}
	if err := writer.Close(); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(r.Context(), method, s.cfg.APIInternalURL+path, &body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if cookie, err := r.Cookie(s.cfg.SessionCookieName); err == nil {
		req.AddCookie(cookie)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return errUnauthorized
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr struct {
			Error string `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err == nil && apiErr.Error != "" {
			return errors.New(apiErr.Error)
		}
		return fmt.Errorf("api returned %s", resp.Status)
	}
	if target == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func (s *webServer) parseDocumentForm(w http.ResponseWriter, r *http.Request) error {
	if !isMultipart(r) {
		return r.ParseForm()
	}
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.DocumentMaxBytes+1024*1024)
	return r.ParseMultipartForm(s.cfg.DocumentMaxBytes)
}

func isMultipart(r *http.Request) bool {
	return strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/form-data")
}

func documentFileFromForm(r *http.Request) *multipart.FileHeader {
	if r.MultipartForm == nil || r.MultipartForm.File == nil {
		return nil
	}
	files := r.MultipartForm.File["file"]
	if len(files) == 0 || files[0].Filename == "" {
		return nil
	}
	return files[0]
}

func (s *webServer) render(w http.ResponseWriter, status int, data pageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := s.templates.ExecuteTemplate(w, "layout", data); err != nil {
		s.logger.Error("render template", "error", err)
	}
}

func defaultValue(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func formatDate(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Format("Jan 2")
}

func formatDateTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("Jan 2, 3:04 PM")
}

func formatDateInput(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

func formatDateTimeInput(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("2006-01-02T15:04")
}

func formatDateTimeInputPtr(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Local().Format("2006-01-02T15:04")
}

func monthTitle(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("January 2006")
}

func dateTitle(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("Mon, Jan 2")
}

func dateShort(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("Jan 2")
}

func dayNumber(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("2")
}

func weekdayShort(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("Mon")
}

func entryTime(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	local := t.Local()
	if local.Hour() == 0 && local.Minute() == 0 && local.Second() == 0 {
		return ""
	}
	return local.Format("3:04 PM")
}

func calendarStateFromRequest(r *http.Request) (string, time.Time) {
	view := strings.TrimSpace(r.URL.Query().Get("calendar_view"))
	if !validDashboardCalendarView(view) {
		if cookie, err := r.Cookie("homebase_dashboard_calendar_view"); err == nil && validDashboardCalendarView(cookie.Value) {
			view = cookie.Value
		} else {
			view = "week"
		}
	}

	focus := time.Now()
	if raw := strings.TrimSpace(r.URL.Query().Get("calendar_date")); raw != "" {
		if parsed, err := time.ParseInLocation("2006-01-02", raw, time.Local); err == nil {
			focus = parsed
		}
	} else if raw := strings.TrimSpace(r.URL.Query().Get("calendar_month")); raw != "" {
		if parsed, err := time.ParseInLocation("2006-01", raw, time.Local); err == nil {
			focus = parsed
		}
	}

	return view, focus
}

func validDashboardCalendarView(view string) bool {
	switch view {
	case "day", "week", "month":
		return true
	default:
		return false
	}
}

func calendarDay(calendar *store.CalendarMonth, focus time.Time) store.CalendarDay {
	if calendar == nil {
		return store.CalendarDay{Date: focus, InMonth: true}
	}
	key := focus.In(time.Local).Format("2006-01-02")
	for _, day := range calendar.Days {
		if day.Date.In(time.Local).Format("2006-01-02") == key {
			return day
		}
	}
	return store.CalendarDay{Date: focus, InMonth: focus.Month() == calendar.Month.Month()}
}

func calendarWeek(calendar *store.CalendarMonth, focus time.Time) []store.CalendarDay {
	start := time.Date(focus.In(time.Local).Year(), focus.In(time.Local).Month(), focus.In(time.Local).Day(), 0, 0, 0, 0, time.Local)
	start = start.AddDate(0, 0, -int(start.Weekday()))
	days := make([]store.CalendarDay, 0, 7)
	for i := 0; i < 7; i++ {
		days = append(days, calendarDay(calendar, start.AddDate(0, 0, i)))
	}
	return days
}

func addDays(t time.Time, days int) time.Time {
	return t.AddDate(0, 0, days)
}

func addMonths(t time.Time, months int) time.Time {
	if t.IsZero() {
		t = time.Now()
	}
	return time.Date(t.In(time.Local).Year(), t.In(time.Local).Month(), 1, 0, 0, 0, 0, time.Local).AddDate(0, months, 0)
}

func calendarStep(view string, focus time.Time, direction int) time.Time {
	if focus.IsZero() {
		focus = time.Now()
	}
	switch view {
	case "day":
		return focus.In(time.Local).AddDate(0, 0, direction)
	case "week":
		return focus.In(time.Local).AddDate(0, 0, direction*7)
	default:
		return addMonths(focus, direction)
	}
}

func dashboardCalendarURL(view string, date time.Time) string {
	if date.IsZero() {
		date = time.Now()
	}
	return "/?calendar_view=" + url.QueryEscape(view) + "&calendar_date=" + url.QueryEscape(date.In(time.Local).Format("2006-01-02"))
}

func calendarPageURL(date time.Time) string {
	if date.IsZero() {
		date = time.Now()
	}
	return "/calendar?month=" + url.QueryEscape(date.In(time.Local).Format("2006-01"))
}

func dictRelated(docs []store.RelatedDocument, entityType string, entityID int64) map[string]any {
	return dictRelatedPrefix(docs, "/"+entityType+"s/"+strconv.FormatInt(entityID, 10)+"/documents")
}

func dict(values ...any) (map[string]any, error) {
	if len(values)%2 != 0 {
		return nil, errors.New("dict requires key-value pairs")
	}
	result := make(map[string]any, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			return nil, errors.New("dict keys must be strings")
		}
		result[key] = values[i+1]
	}
	return result, nil
}

func dictRelatedPrefix(docs []store.RelatedDocument, removePrefix string) map[string]any {
	return map[string]any{
		"Docs":         docs,
		"RemovePrefix": removePrefix,
	}
}

func dictDocumentInfo(id string, document store.Document) map[string]any {
	return map[string]any{
		"ID":       id,
		"Document": document,
	}
}

func dictRelatedContacts(contacts []store.RelatedContact, removePrefix string) map[string]any {
	return map[string]any{
		"Contacts":     contacts,
		"RemovePrefix": removePrefix,
	}
}

func dictContactInfo(id string, contact store.Contact) map[string]any {
	return map[string]any{
		"ID":      id,
		"Contact": contact,
	}
}

func dictRelatedAssets(assets []store.RelatedAsset, removePrefix string) map[string]any {
	return map[string]any{
		"Assets":       assets,
		"RemovePrefix": removePrefix,
	}
}

func dictAssetInfo(id string, asset store.Asset) map[string]any {
	return map[string]any{
		"ID":    id,
		"Asset": asset,
	}
}

func dictAttach(id, action string, documents []store.Document) map[string]any {
	return map[string]any{
		"ID":        id,
		"Action":    action,
		"Documents": documents,
	}
}

func dictAttachContact(id, action string, contacts []store.Contact) map[string]any {
	return map[string]any{
		"ID":       id,
		"Action":   action,
		"Contacts": contacts,
	}
}

func dictAttachAsset(id, action string, assets []store.Asset) map[string]any {
	return map[string]any{
		"ID":     id,
		"Action": action,
		"Assets": assets,
		"Asset":  store.Asset{},
	}
}

func taskRelatedDocs(taskDocuments map[int64][]store.RelatedDocument, taskID int64) []store.RelatedDocument {
	if taskDocuments == nil {
		return nil
	}
	return taskDocuments[taskID]
}

func hasTaskDocuments(taskDocuments map[int64][]store.RelatedDocument) bool {
	for _, docs := range taskDocuments {
		if len(docs) > 0 {
			return true
		}
	}
	return false
}

func taskRelatedContacts(taskContacts map[int64][]store.RelatedContact, taskID int64) []store.RelatedContact {
	if taskContacts == nil {
		return nil
	}
	return taskContacts[taskID]
}

func hasTaskContacts(taskContacts map[int64][]store.RelatedContact) bool {
	for _, contacts := range taskContacts {
		if len(contacts) > 0 {
			return true
		}
	}
	return false
}

func taskRelatedAssets(taskAssets map[int64][]store.RelatedAsset, taskID int64) []store.RelatedAsset {
	if taskAssets == nil {
		return nil
	}
	return taskAssets[taskID]
}

func hasTaskAssets(taskAssets map[int64][]store.RelatedAsset) bool {
	for _, assets := range taskAssets {
		if len(assets) > 0 {
			return true
		}
	}
	return false
}

func documentOpenURL(document store.Document) string {
	if document.FileName != "" {
		return "/documents/" + strconv.FormatInt(document.ID, 10) + "/file/" + url.PathEscape(document.FileName)
	}
	return ""
}

func documentSourceLabel(document store.Document) string {
	if document.FileName != "" {
		return document.FileName
	}
	return "No file"
}

func fileSize(size int64) string {
	if size <= 0 {
		return ""
	}
	const unit = 1024
	if size < unit {
		return strconv.FormatInt(size, 10) + " B"
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit && exp < 3; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(size)/float64(div), "KMGT"[exp])
}

func formatMoney(value *float64) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%.2f", *value)
}

func count(items []store.ModuleItem) int {
	return len(items)
}

func initials(name, email string) string {
	source := strings.TrimSpace(name)
	if source == "" {
		source = strings.TrimSpace(email)
	}
	if source == "" {
		return "?"
	}

	parts := strings.FieldsFunc(source, func(r rune) bool {
		return r == ' ' || r == '.' || r == '_' || r == '-' || r == '@'
	})
	if len(parts) == 0 {
		return strings.ToUpper(source[:1])
	}

	first := strings.ToUpper(parts[0][:1])
	if len(parts) == 1 {
		return first
	}
	return first + strings.ToUpper(parts[1][:1])
}

func selectedID(current *int64, id int64) template.HTMLAttr {
	if current != nil && *current == id {
		return "selected"
	}
	return ""
}

func idValue(id *int64) int64 {
	if id == nil {
		return 0
	}
	return *id
}

func selectedString(current, value string) template.HTMLAttr {
	if current == value {
		return "selected"
	}
	return ""
}

func optionalInt64(value string) (int64, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	id, err := strconv.ParseInt(value, 10, 64)
	return id, err == nil && id > 0
}

func routinePayloadFromForm(r *http.Request) map[string]any {
	payload := map[string]any{
		"title":   r.FormValue("title"),
		"notes":   r.FormValue("notes"),
		"cadence": defaultValue(r.FormValue("cadence"), "monthly"),
		"status":  defaultValue(r.FormValue("status"), "active"),
	}
	if assignedTo, ok := optionalInt64(r.FormValue("assigned_to")); ok {
		payload["assigned_to"] = assignedTo
	}
	if nextDue := strings.TrimSpace(r.FormValue("next_due_at")); nextDue != "" {
		payload["next_due_at"] = nextDue + "T00:00:00Z"
	}
	return payload
}

func listPayloadFromForm(r *http.Request) map[string]any {
	return map[string]any{
		"title":       r.FormValue("title"),
		"description": r.FormValue("description"),
		"kind":        defaultValue(r.FormValue("kind"), "general"),
		"status":      defaultValue(r.FormValue("status"), "active"),
	}
}

func listItemPayloadFromForm(r *http.Request) map[string]any {
	payload := map[string]any{
		"title":  r.FormValue("title"),
		"notes":  r.FormValue("notes"),
		"status": defaultValue(r.FormValue("status"), "open"),
	}
	if assignedTo, ok := optionalInt64(r.FormValue("assigned_to")); ok {
		payload["assigned_to"] = assignedTo
	}
	if due := strings.TrimSpace(r.FormValue("due_at")); due != "" {
		if strings.Contains(due, "T") {
			payload["due_at"] = due + ":00Z"
		} else {
			payload["due_at"] = due + "T00:00:00Z"
		}
	}
	return payload
}

func contactPayloadFromForm(r *http.Request) map[string]any {
	return map[string]any{
		"name":   r.FormValue("name"),
		"kind":   defaultValue(r.FormValue("kind"), "general"),
		"email":  r.FormValue("email"),
		"phone":  r.FormValue("phone"),
		"notes":  r.FormValue("notes"),
		"status": defaultValue(r.FormValue("status"), "active"),
	}
}

func assetPayloadFromForm(r *http.Request) map[string]any {
	payload := map[string]any{
		"name":          r.FormValue("name"),
		"kind":          defaultValue(r.FormValue("kind"), "general"),
		"serial_number": r.FormValue("serial_number"),
		"vendor":        r.FormValue("vendor"),
		"model":         r.FormValue("model"),
		"notes":         r.FormValue("notes"),
		"status":        defaultValue(r.FormValue("status"), "active"),
	}
	if warranty := strings.TrimSpace(r.FormValue("warranty_expires_at")); warranty != "" {
		payload["warranty_expires_at"] = warranty + "T00:00:00Z"
	}
	if purchaseDate := strings.TrimSpace(r.FormValue("purchase_date")); purchaseDate != "" {
		payload["purchase_date"] = purchaseDate + "T00:00:00Z"
	}
	if cost := strings.TrimSpace(r.FormValue("purchase_cost")); cost != "" {
		if parsed, err := strconv.ParseFloat(cost, 64); err == nil {
			payload["purchase_cost"] = parsed
		}
	}
	return payload
}

func assetMaintenancePayloadFromForm(r *http.Request) map[string]any {
	payload := map[string]any{
		"title":   r.FormValue("title"),
		"notes":   r.FormValue("notes"),
		"cadence": defaultValue(r.FormValue("cadence"), "monthly"),
		"status":  defaultValue(r.FormValue("status"), "active"),
	}
	if nextDue := strings.TrimSpace(r.FormValue("next_due_at")); nextDue != "" {
		payload["next_due_at"] = nextDue + "T00:00:00Z"
	}
	return payload
}

func documentPayloadFromForm(r *http.Request) map[string]any {
	fields := documentFieldsFromForm(r)
	return map[string]any{
		"title":       fields["title"],
		"description": fields["description"],
		"kind":        fields["kind"],
		"status":      fields["status"],
	}
}

func documentFieldsFromForm(r *http.Request) map[string]string {
	return map[string]string{
		"title":       r.FormValue("title"),
		"description": r.FormValue("description"),
		"kind":        defaultValue(r.FormValue("kind"), "general"),
		"status":      defaultValue(r.FormValue("status"), "active"),
	}
}

func taskPayloadFromForm(r *http.Request) map[string]any {
	payload := map[string]any{
		"title":    r.FormValue("title"),
		"notes":    r.FormValue("notes"),
		"status":   defaultValue(r.FormValue("status"), "open"),
		"priority": defaultValue(r.FormValue("priority"), "normal"),
	}
	if projectID, ok := optionalInt64(r.FormValue("project_id")); ok {
		payload["project_id"] = projectID
	}
	if folderID, ok := optionalInt64(r.FormValue("project_folder_id")); ok {
		payload["project_folder_id"] = folderID
	}
	if routineID, ok := optionalInt64(r.FormValue("routine_id")); ok {
		payload["routine_id"] = routineID
	}
	if assetID, ok := optionalInt64(r.FormValue("asset_id")); ok {
		payload["asset_id"] = assetID
	}
	if itemID, ok := optionalInt64(r.FormValue("asset_maintenance_item_id")); ok {
		payload["asset_maintenance_item_id"] = itemID
	}
	if assignedTo, ok := optionalInt64(r.FormValue("assigned_to")); ok {
		payload["assigned_to"] = assignedTo
	}
	if due := strings.TrimSpace(r.FormValue("due_at")); due != "" {
		if strings.Contains(due, "T") {
			payload["due_at"] = due + ":00Z"
		} else {
			payload["due_at"] = due + "T00:00:00Z"
		}
	}
	return payload
}

func tileOrder(order []string, key string) int {
	for i, current := range order {
		if current == key {
			return i + 1
		}
	}
	return 99
}

func tileActive(order []string, key string) bool {
	for _, current := range order {
		if current == key {
			return true
		}
	}
	return false
}

func standaloneTasks(tasks []store.Task) []store.Task {
	filtered := []store.Task{}
	for _, task := range tasks {
		if task.ProjectID == nil {
			filtered = append(filtered, task)
		}
	}
	return filtered
}

func activeTasks(tasks []store.Task) []store.Task {
	filtered := []store.Task{}
	for _, task := range tasks {
		if task.Status == "open" {
			filtered = append(filtered, task)
		}
	}
	return filtered
}

func activeStandaloneTasks(tasks []store.Task) []store.Task {
	filtered := []store.Task{}
	for _, task := range tasks {
		if task.ProjectID == nil && task.Status == "open" {
			filtered = append(filtered, task)
		}
	}
	return filtered
}

func projectTasks(tasks []store.Task, projectID int64) []store.Task {
	filtered := []store.Task{}
	for _, task := range tasks {
		if task.ProjectID != nil && *task.ProjectID == projectID {
			filtered = append(filtered, task)
		}
	}
	return filtered
}

func tasksInFolder(tasks []store.Task, folderID int64) []store.Task {
	filtered := []store.Task{}
	for _, task := range tasks {
		if task.ProjectFolderID != nil && *task.ProjectFolderID == folderID {
			filtered = append(filtered, task)
		}
	}
	return filtered
}

func tasksWithoutFolder(tasks []store.Task) []store.Task {
	filtered := []store.Task{}
	for _, task := range tasks {
		if task.ProjectFolderID == nil {
			filtered = append(filtered, task)
		}
	}
	return filtered
}

func activeProjectTasks(tasks []store.Task, projectID int64) []store.Task {
	filtered := []store.Task{}
	for _, task := range tasks {
		if task.ProjectID != nil && *task.ProjectID == projectID && task.Status == "open" {
			filtered = append(filtered, task)
		}
	}
	return filtered
}

func openTaskCount(tasks []store.Task) int {
	count := 0
	for _, task := range tasks {
		if task.Status == "open" {
			count++
		}
	}
	return count
}

func doneTaskCount(tasks []store.Task) int {
	count := 0
	for _, task := range tasks {
		if task.Status == "done" {
			count++
		}
	}
	return count
}

func openListItemCount(items []store.ListItem) int {
	count := 0
	for _, item := range items {
		if item.Status == "open" {
			count++
		}
	}
	return count
}

func doneListItemCount(items []store.ListItem) int {
	count := 0
	for _, item := range items {
		if item.Status == "done" {
			count++
		}
	}
	return count
}

func folderStatus(tasks []store.Task) string {
	if len(tasks) == 0 {
		return "empty"
	}
	for _, task := range tasks {
		if task.Status == "open" {
			return "open"
		}
	}
	return "done"
}

func folderDue(tasks []store.Task) *time.Time {
	var due *time.Time
	for _, task := range tasks {
		if task.DueAt == nil {
			continue
		}
		if due == nil || task.DueAt.Before(*due) {
			due = task.DueAt
		}
	}
	return due
}

func taskContext(task store.Task, projects []store.Project, routines []store.Routine) string {
	if task.ProjectID != nil {
		for _, project := range projects {
			if project.ID == *task.ProjectID {
				return "Project · " + project.Title
			}
		}
		return "Project task"
	}
	if task.RoutineID != nil {
		for _, routine := range routines {
			if routine.ID == *task.RoutineID {
				return "Routine · " + routine.Title
			}
		}
		return "Routine task"
	}
	if task.AssetID != nil {
		return "Asset maintenance"
	}
	return "Standalone task"
}

func routineTasks(tasks []store.Task, routineID int64) []store.Task {
	filtered := []store.Task{}
	for _, task := range tasks {
		if task.RoutineID != nil && *task.RoutineID == routineID {
			filtered = append(filtered, task)
		}
	}
	return filtered
}

func assetTasks(tasks []store.Task, assetID int64) []store.Task {
	filtered := []store.Task{}
	for _, task := range tasks {
		if task.AssetID != nil && *task.AssetID == assetID {
			filtered = append(filtered, task)
		}
	}
	return filtered
}

func taskStatCount(tasks []store.Task, now time.Time, bucket string) int {
	start := dayStart(now)
	tomorrow := start.AddDate(0, 0, 1)
	week := start.AddDate(0, 0, 8)
	count := 0
	for _, task := range tasks {
		if task.Status != "open" || task.DueAt == nil {
			continue
		}
		switch bucket {
		case "past":
			if task.DueAt.Before(start) {
				count++
			}
		case "today":
			if !task.DueAt.Before(start) && task.DueAt.Before(tomorrow) {
				count++
			}
		case "upcoming":
			if !task.DueAt.Before(tomorrow) && task.DueAt.Before(week) {
				count++
			}
		}
	}
	return count
}

func projectStatCount(projects []store.Project, now time.Time, bucket string) int {
	start := dayStart(now)
	tomorrow := start.AddDate(0, 0, 1)
	week := start.AddDate(0, 0, 8)
	count := 0
	for _, project := range projects {
		if project.Status == "done" || project.Status == "archived" || project.DueDate == nil {
			continue
		}
		switch bucket {
		case "past":
			if project.DueDate.Before(start) {
				count++
			}
		case "today":
			if !project.DueDate.Before(start) && project.DueDate.Before(tomorrow) {
				count++
			}
		case "upcoming":
			if !project.DueDate.Before(tomorrow) && project.DueDate.Before(week) {
				count++
			}
		}
	}
	return count
}

func taskDueBucket(task store.Task, now time.Time) string {
	return dueBucket(task.DueAt, now)
}

func projectDueBucket(project store.Project, now time.Time) string {
	return dueBucket(project.DueDate, now)
}

func dueBucket(due *time.Time, now time.Time) string {
	if due == nil || due.IsZero() {
		return "none"
	}
	start := dayStart(now)
	tomorrow := start.AddDate(0, 0, 1)
	week := start.AddDate(0, 0, 8)
	switch {
	case due.Before(start):
		return "past"
	case !due.Before(start) && due.Before(tomorrow):
		return "today"
	case !due.Before(tomorrow) && due.Before(week):
		return "upcoming"
	default:
		return "later"
	}
}

func eventsForDayOffset(events []store.Event, now time.Time, offset int) []store.Event {
	start := dayStart(now).AddDate(0, 0, offset)
	end := start.AddDate(0, 0, 1)
	filtered := []store.Event{}
	for _, event := range events {
		if !event.StartsAt.Before(start) && event.StartsAt.Before(end) {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

func dayStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func selectedDashboardListID(r *http.Request, lists []store.HouseholdList) int64 {
	if raw := strings.TrimSpace(r.URL.Query().Get("list_id")); raw != "" {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil && listIDExists(lists, id) {
			return id
		}
	}
	if cookie, err := r.Cookie("homebase_dashboard_list_id"); err == nil {
		if id, err := strconv.ParseInt(cookie.Value, 10, 64); err == nil && listIDExists(lists, id) {
			return id
		}
	}
	if len(lists) > 0 {
		return lists[0].ID
	}
	return 0
}

func listIDExists(lists []store.HouseholdList, id int64) bool {
	for _, list := range lists {
		if list.ID == id {
			return true
		}
	}
	return false
}

func dueFilterFromRequest(r *http.Request) string {
	value := strings.TrimSpace(r.URL.Query().Get("due"))
	switch value {
	case "past", "today", "upcoming":
		return value
	default:
		return ""
	}
}

func safeReturnTo(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
		return fallback
	}
	return value
}

func userFacingError(value string) string {
	switch value {
	case "":
		return ""
	case "not_added":
		return "That account has not been added to a household yet."
	case "form":
		return "The form could not be read."
	default:
		return value
	}
}

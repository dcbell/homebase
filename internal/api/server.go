package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"homebase/internal/config"
	"homebase/internal/store"
)

type contextKey string

const (
	userKey      contextKey = "user"
	householdKey contextKey = "household"
)

type Server struct {
	cfg    config.Config
	store  *store.Store
	logger *slog.Logger
	mux    *http.ServeMux
}

func New(cfg config.Config, st *store.Store, logger *slog.Logger) *Server {
	s := &Server{
		cfg:    cfg,
		store:  st,
		logger: logger,
		mux:    http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.health)
	s.mux.HandleFunc("GET /auth/google/start", s.googleStart)
	s.mux.HandleFunc("GET /auth/google/callback", s.googleCallback)
	s.mux.HandleFunc("GET /auth/dev-login", s.devLogin)
	s.mux.HandleFunc("POST /auth/logout", s.logout)

	s.mux.Handle("/api/v1/", s.requireSession(http.HandlerFunc(s.apiRoute)))
}

func (s *Server) apiRoute(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1")

	switch {
	case r.Method == http.MethodGet && path == "/me":
		s.me(w, r)
	case r.Method == http.MethodGet && path == "/households/current":
		s.currentHousehold(w, r)
	case r.Method == http.MethodGet && path == "/dashboard":
		s.dashboard(w, r)
	case r.Method == http.MethodGet && path == "/calendar":
		s.calendar(w, r)
	case r.Method == http.MethodPost && path == "/dashboard/tiles/move":
		s.moveDashboardTile(w, r)
	case r.Method == http.MethodPost && path == "/dashboard/tiles/order":
		s.setDashboardTileOrder(w, r)
	case r.Method == http.MethodGet && path == "/members":
		s.listMembers(w, r)
	case r.Method == http.MethodPost && path == "/members":
		s.addMember(w, r)
	case r.Method == http.MethodPatch && strings.HasPrefix(path, "/members/"):
		s.updateMember(w, r, strings.TrimPrefix(path, "/members/"))
	case r.Method == http.MethodDelete && strings.HasPrefix(path, "/members/"):
		s.removeMember(w, r, strings.TrimPrefix(path, "/members/"))
	case r.Method == http.MethodGet && path == "/projects":
		s.listProjects(w, r)
	case r.Method == http.MethodPost && path == "/projects":
		s.createProject(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/projects/") && strings.HasSuffix(path, "/folders"):
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/projects/"), "/folders")
		s.listProjectFolders(w, r, id)
	case r.Method == http.MethodPost && strings.HasPrefix(path, "/projects/") && strings.HasSuffix(path, "/folders"):
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/projects/"), "/folders")
		s.createProjectFolder(w, r, id)
	case r.Method == http.MethodPatch && strings.HasPrefix(path, "/project-folders/"):
		s.updateProjectFolder(w, r, strings.TrimPrefix(path, "/project-folders/"))
	case r.Method == http.MethodPost && strings.HasPrefix(path, "/project-folders/") && strings.HasSuffix(path, "/archive"):
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/project-folders/"), "/archive")
		s.archiveProjectFolder(w, r, id)
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/projects/"):
		s.getProject(w, r, strings.TrimPrefix(path, "/projects/"))
	case r.Method == http.MethodPatch && strings.HasPrefix(path, "/projects/"):
		s.updateProject(w, r, strings.TrimPrefix(path, "/projects/"))
	case r.Method == http.MethodPost && strings.HasPrefix(path, "/projects/") && strings.HasSuffix(path, "/archive"):
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/projects/"), "/archive")
		s.archiveProject(w, r, id)
	case r.Method == http.MethodGet && path == "/tasks":
		s.listTasks(w, r)
	case r.Method == http.MethodPost && path == "/tasks":
		s.createTask(w, r)
	case r.Method == http.MethodPatch && strings.HasPrefix(path, "/tasks/") && strings.HasSuffix(path, "/complete"):
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/tasks/"), "/complete")
		s.completeTask(w, r, id)
	case r.Method == http.MethodPatch && strings.HasPrefix(path, "/tasks/") && strings.HasSuffix(path, "/reopen"):
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/tasks/"), "/reopen")
		s.reopenTask(w, r, id)
	case r.Method == http.MethodPost && strings.HasPrefix(path, "/tasks/") && strings.HasSuffix(path, "/archive"):
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/tasks/"), "/archive")
		s.archiveTask(w, r, id)
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/tasks/"):
		s.getTask(w, r, strings.TrimPrefix(path, "/tasks/"))
	case r.Method == http.MethodPatch && strings.HasPrefix(path, "/tasks/"):
		s.updateTask(w, r, strings.TrimPrefix(path, "/tasks/"))
	case r.Method == http.MethodGet && path == "/events":
		s.listEvents(w, r)
	case r.Method == http.MethodPost && path == "/events":
		s.createEvent(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/events/"):
		s.getEvent(w, r, strings.TrimPrefix(path, "/events/"))
	case r.Method == http.MethodPatch && strings.HasPrefix(path, "/events/"):
		s.updateEvent(w, r, strings.TrimPrefix(path, "/events/"))
	case r.Method == http.MethodDelete && strings.HasPrefix(path, "/events/"):
		s.deleteEvent(w, r, strings.TrimPrefix(path, "/events/"))
	case r.Method == http.MethodGet && path == "/routines":
		s.listRoutines(w, r)
	case r.Method == http.MethodPost && path == "/routines":
		s.createRoutine(w, r)
	case r.Method == http.MethodPost && strings.HasPrefix(path, "/routines/") && strings.HasSuffix(path, "/generate-task"):
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/routines/"), "/generate-task")
		s.generateRoutineTask(w, r, id)
	case r.Method == http.MethodPost && strings.HasPrefix(path, "/routines/") && strings.HasSuffix(path, "/archive"):
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/routines/"), "/archive")
		s.archiveRoutine(w, r, id)
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/routines/"):
		s.getRoutine(w, r, strings.TrimPrefix(path, "/routines/"))
	case r.Method == http.MethodPatch && strings.HasPrefix(path, "/routines/"):
		s.updateRoutine(w, r, strings.TrimPrefix(path, "/routines/"))
	case r.Method == http.MethodGet && path == "/lists":
		s.listHouseholdLists(w, r)
	case r.Method == http.MethodPost && path == "/lists":
		s.createHouseholdList(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/lists/") && strings.HasSuffix(path, "/items"):
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/lists/"), "/items")
		s.listListItems(w, r, id)
	case r.Method == http.MethodPost && strings.HasPrefix(path, "/lists/") && strings.HasSuffix(path, "/items"):
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/lists/"), "/items")
		s.createListItem(w, r, id)
	case r.Method == http.MethodPost && strings.HasPrefix(path, "/lists/") && strings.HasSuffix(path, "/archive"):
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/lists/"), "/archive")
		s.archiveHouseholdList(w, r, id)
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/lists/"):
		s.getHouseholdList(w, r, strings.TrimPrefix(path, "/lists/"))
	case r.Method == http.MethodPatch && strings.HasPrefix(path, "/lists/"):
		s.updateHouseholdList(w, r, strings.TrimPrefix(path, "/lists/"))
	case r.Method == http.MethodPatch && strings.HasPrefix(path, "/list-items/") && strings.HasSuffix(path, "/complete"):
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/list-items/"), "/complete")
		s.completeListItem(w, r, id)
	case r.Method == http.MethodPatch && strings.HasPrefix(path, "/list-items/") && strings.HasSuffix(path, "/reopen"):
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/list-items/"), "/reopen")
		s.reopenListItem(w, r, id)
	case r.Method == http.MethodPost && strings.HasPrefix(path, "/list-items/") && strings.HasSuffix(path, "/archive"):
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/list-items/"), "/archive")
		s.archiveListItem(w, r, id)
	case r.Method == http.MethodPatch && strings.HasPrefix(path, "/list-items/"):
		s.updateListItem(w, r, strings.TrimPrefix(path, "/list-items/"))
	case r.Method == http.MethodGet && path == "/contacts":
		s.listContacts(w, r)
	case r.Method == http.MethodPost && path == "/contacts":
		s.createContact(w, r)
	case r.Method == http.MethodPost && strings.HasPrefix(path, "/contacts/") && strings.HasSuffix(path, "/archive"):
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/contacts/"), "/archive")
		s.archiveContact(w, r, id)
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/contacts/"):
		s.getContact(w, r, strings.TrimPrefix(path, "/contacts/"))
	case r.Method == http.MethodPatch && strings.HasPrefix(path, "/contacts/"):
		s.updateContact(w, r, strings.TrimPrefix(path, "/contacts/"))
	case r.Method == http.MethodGet && path == "/related-contacts":
		s.listRelatedContacts(w, r)
	case r.Method == http.MethodPost && path == "/related-contacts":
		s.linkRelatedContact(w, r)
	case r.Method == http.MethodDelete && strings.HasPrefix(path, "/contact-links/"):
		s.unlinkContact(w, r, strings.TrimPrefix(path, "/contact-links/"))
	case r.Method == http.MethodGet && path == "/assets":
		s.listAssets(w, r)
	case r.Method == http.MethodPost && path == "/assets":
		s.createAsset(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/assets/") && strings.HasSuffix(path, "/maintenance"):
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/assets/"), "/maintenance")
		s.listAssetMaintenanceItems(w, r, id)
	case r.Method == http.MethodPost && strings.HasPrefix(path, "/assets/") && strings.HasSuffix(path, "/maintenance"):
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/assets/"), "/maintenance")
		s.createAssetMaintenanceItem(w, r, id)
	case r.Method == http.MethodPost && strings.HasPrefix(path, "/assets/") && strings.HasSuffix(path, "/archive"):
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/assets/"), "/archive")
		s.archiveAsset(w, r, id)
	case r.Method == http.MethodPatch && strings.HasPrefix(path, "/asset-maintenance/"):
		s.updateAssetMaintenanceItem(w, r, strings.TrimPrefix(path, "/asset-maintenance/"))
	case r.Method == http.MethodPost && strings.HasPrefix(path, "/asset-maintenance/") && strings.HasSuffix(path, "/archive"):
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/asset-maintenance/"), "/archive")
		s.archiveAssetMaintenanceItem(w, r, id)
	case r.Method == http.MethodPost && strings.HasPrefix(path, "/asset-maintenance/") && strings.HasSuffix(path, "/generate-task"):
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/asset-maintenance/"), "/generate-task")
		s.generateAssetMaintenanceTask(w, r, id)
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/assets/"):
		s.getAsset(w, r, strings.TrimPrefix(path, "/assets/"))
	case r.Method == http.MethodPatch && strings.HasPrefix(path, "/assets/"):
		s.updateAsset(w, r, strings.TrimPrefix(path, "/assets/"))
	case r.Method == http.MethodGet && path == "/related-assets":
		s.listRelatedAssets(w, r)
	case r.Method == http.MethodPost && path == "/related-assets":
		s.linkRelatedAsset(w, r)
	case r.Method == http.MethodDelete && strings.HasPrefix(path, "/asset-links/"):
		s.unlinkAsset(w, r, strings.TrimPrefix(path, "/asset-links/"))
	case r.Method == http.MethodGet && path == "/documents":
		s.listDocuments(w, r)
	case r.Method == http.MethodPost && path == "/documents":
		s.createDocument(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/documents/") && strings.HasSuffix(path, "/related-items"):
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/documents/"), "/related-items")
		s.listRelatedItemsForDocument(w, r, id)
	case r.Method == http.MethodPost && strings.HasPrefix(path, "/documents/") && strings.HasSuffix(path, "/related-items"):
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/documents/"), "/related-items")
		s.linkRelatedItemToDocument(w, r, id)
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/documents/") && strings.HasSuffix(path, "/download"):
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/documents/"), "/download")
		s.downloadDocument(w, r, id)
	case r.Method == http.MethodPost && strings.HasPrefix(path, "/documents/") && strings.HasSuffix(path, "/archive"):
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/documents/"), "/archive")
		s.archiveDocument(w, r, id)
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/documents/"):
		s.getDocument(w, r, strings.TrimPrefix(path, "/documents/"))
	case r.Method == http.MethodPatch && strings.HasPrefix(path, "/documents/"):
		s.updateDocument(w, r, strings.TrimPrefix(path, "/documents/"))
	case r.Method == http.MethodGet && path == "/related-documents":
		s.listRelatedDocuments(w, r)
	case r.Method == http.MethodPost && path == "/related-documents":
		s.linkRelatedDocument(w, r)
	case r.Method == http.MethodDelete && strings.HasPrefix(path, "/document-links/"):
		s.unlinkDocument(w, r, strings.TrimPrefix(path, "/document-links/"))
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/modules/"):
		s.listModule(w, r, strings.TrimPrefix(path, "/modules/"))
	case r.Method == http.MethodPost && path == "/integrations/google/calendar/connect":
		s.googleCalendarConnect(w, r)
	case r.Method == http.MethodPost && path == "/integrations/google/calendar/sync":
		s.googleCalendarSync(w, r)
	default:
		writeError(w, http.StatusNotFound, "route not found")
	}
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(s.cfg.SessionCookieName)
		if err != nil || cookie.Value == "" {
			writeError(w, http.StatusUnauthorized, "login required")
			return
		}

		user, household, err := s.store.SessionContext(r.Context(), cookie.Value)
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "session expired")
			return
		}
		if err != nil {
			s.logger.Error("load session", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to load session")
			return
		}

		ctx := context.WithValue(r.Context(), userKey, user)
		ctx = context.WithValue(ctx, householdKey, household)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func userFrom(r *http.Request) store.User {
	return r.Context().Value(userKey).(store.User)
}

func householdFrom(r *http.Request) store.Household {
	return r.Context().Value(householdKey).(store.Household)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"user":      userFrom(r),
		"household": householdFrom(r),
	})
}

func (s *Server) currentHousehold(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, householdFrom(r))
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	dashboard, err := s.store.Dashboard(r.Context(), userFrom(r), householdFrom(r), strings.TrimSpace(getenv("BUDGET_APP_URL")), s.cfg.GoogleConfigured())
	if err != nil {
		s.logger.Error("dashboard", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to load dashboard")
		return
	}
	writeJSON(w, http.StatusOK, dashboard)
}

func (s *Server) calendar(w http.ResponseWriter, r *http.Request) {
	month := time.Now()
	if raw := strings.TrimSpace(r.URL.Query().Get("month")); raw != "" {
		parsed, err := time.Parse("2006-01", raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "month must use YYYY-MM")
			return
		}
		month = parsed
	}

	calendar, err := s.store.CalendarMonth(r.Context(), householdFrom(r).ID, month)
	if err != nil {
		s.logger.Error("calendar", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to load calendar")
		return
	}
	writeJSON(w, http.StatusOK, calendar)
}

func (s *Server) moveDashboardTile(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Tile      string `json:"tile"`
		Direction string `json:"direction"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}

	if err := s.store.MoveDashboardTile(r.Context(), householdFrom(r).ID, input.Tile, input.Direction); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusBadRequest, "invalid tile move")
		return
	} else if err != nil {
		s.logger.Error("move dashboard tile", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to move tile")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) setDashboardTileOrder(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Tiles []string `json:"tiles"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := s.store.SetDashboardTileOrder(r.Context(), householdFrom(r).ID, input.Tiles); err != nil {
		s.logger.Error("set dashboard tile order", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to save tile order")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) listMembers(w http.ResponseWriter, r *http.Request) {
	members, err := s.store.ListMembers(r.Context(), householdFrom(r).ID)
	if err != nil {
		s.logger.Error("list members", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list members")
		return
	}
	writeJSON(w, http.StatusOK, members)
}

func (s *Server) addMember(w http.ResponseWriter, r *http.Request) {
	if householdFrom(r).Role != "owner" {
		writeError(w, http.StatusForbidden, "only owners can add household members")
		return
	}

	var input store.MemberInput
	if !decodeJSON(w, r, &input) {
		return
	}

	member, err := s.store.AddMember(r.Context(), householdFrom(r).ID, input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, member)
}

func (s *Server) updateMember(w http.ResponseWriter, r *http.Request, id string) {
	if householdFrom(r).Role != "owner" {
		writeError(w, http.StatusForbidden, "only owners can update household members")
		return
	}
	memberID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid member id")
		return
	}

	var input store.MemberInput
	if !decodeJSON(w, r, &input) {
		return
	}

	member, err := s.store.UpdateMember(r.Context(), householdFrom(r).ID, memberID, input)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "member not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, member)
}

func (s *Server) removeMember(w http.ResponseWriter, r *http.Request, id string) {
	if householdFrom(r).Role != "owner" {
		writeError(w, http.StatusForbidden, "only owners can remove household members")
		return
	}
	memberID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid member id")
		return
	}

	err = s.store.RemoveMember(r.Context(), householdFrom(r).ID, memberID, userFrom(r).ID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "member not found")
		return
	}
	if errors.Is(err, store.ErrForbidden) {
		writeError(w, http.StatusForbidden, "owners cannot remove themselves")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) listProjects(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListProjects(r.Context(), householdFrom(r).ID)
	if err != nil {
		s.logger.Error("list projects", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list projects")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	var input store.ProjectInput
	if !decodeJSON(w, r, &input) {
		return
	}
	project, err := s.store.CreateProject(r.Context(), householdFrom(r).ID, userFrom(r).ID, input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, project)
}

func (s *Server) getProject(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	project, err := s.store.GetProject(r.Context(), householdFrom(r).ID, id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load project")
		return
	}
	writeJSON(w, http.StatusOK, project)
}

func (s *Server) updateProject(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	var input store.ProjectInput
	if !decodeJSON(w, r, &input) {
		return
	}
	project, err := s.store.UpdateProject(r.Context(), householdFrom(r).ID, id, input)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, project)
}

func (s *Server) archiveProject(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	if err := s.store.ArchiveProject(r.Context(), householdFrom(r).ID, id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "project not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to archive project")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) listProjectFolders(w http.ResponseWriter, r *http.Request, rawProjectID string) {
	projectID, ok := parseID(w, rawProjectID)
	if !ok {
		return
	}
	folders, err := s.store.ListProjectFolders(r.Context(), householdFrom(r).ID, projectID)
	if err != nil {
		s.logger.Error("list project folders", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list project folders")
		return
	}
	writeJSON(w, http.StatusOK, folders)
}

func (s *Server) createProjectFolder(w http.ResponseWriter, r *http.Request, rawProjectID string) {
	projectID, ok := parseID(w, rawProjectID)
	if !ok {
		return
	}
	var input store.ProjectFolderInput
	if !decodeJSON(w, r, &input) {
		return
	}
	folder, err := s.store.CreateProjectFolder(r.Context(), householdFrom(r).ID, userFrom(r).ID, projectID, input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, folder)
}

func (s *Server) updateProjectFolder(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	var input store.ProjectFolderInput
	if !decodeJSON(w, r, &input) {
		return
	}
	folder, err := s.store.UpdateProjectFolder(r.Context(), householdFrom(r).ID, id, input)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "project folder not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, folder)
}

func (s *Server) archiveProjectFolder(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	if err := s.store.ArchiveProjectFolder(r.Context(), householdFrom(r).ID, id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "project folder not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to archive project folder")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) listTasks(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListTasks(r.Context(), householdFrom(r).ID)
	if err != nil {
		s.logger.Error("list tasks", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list tasks")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) createTask(w http.ResponseWriter, r *http.Request) {
	var input store.TaskInput
	if !decodeJSON(w, r, &input) {
		return
	}
	task, err := s.store.CreateTask(r.Context(), householdFrom(r).ID, userFrom(r).ID, input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, task)
}

func (s *Server) getTask(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	task, err := s.store.GetTask(r.Context(), householdFrom(r).ID, id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load task")
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (s *Server) updateTask(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	var input store.TaskInput
	if !decodeJSON(w, r, &input) {
		return
	}
	task, err := s.store.UpdateTask(r.Context(), householdFrom(r).ID, id, input)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (s *Server) completeTask(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	task, err := s.store.CompleteTask(r.Context(), householdFrom(r).ID, id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to complete task")
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (s *Server) reopenTask(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	task, err := s.store.ReopenTask(r.Context(), householdFrom(r).ID, id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to reopen task")
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (s *Server) archiveTask(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	if err := s.store.ArchiveTask(r.Context(), householdFrom(r).ID, id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "task not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to archive task")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) listEvents(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListEvents(r.Context(), householdFrom(r).ID)
	if err != nil {
		s.logger.Error("list events", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list events")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) createEvent(w http.ResponseWriter, r *http.Request) {
	var input store.EventInput
	if !decodeJSON(w, r, &input) {
		return
	}
	event, err := s.store.CreateEvent(r.Context(), householdFrom(r).ID, userFrom(r).ID, input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, event)
}

func (s *Server) getEvent(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	event, err := s.store.GetEvent(r.Context(), householdFrom(r).ID, id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "event not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load event")
		return
	}
	writeJSON(w, http.StatusOK, event)
}

func (s *Server) updateEvent(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	var input store.EventInput
	if !decodeJSON(w, r, &input) {
		return
	}
	event, err := s.store.UpdateEvent(r.Context(), householdFrom(r).ID, id, input)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "event not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, event)
}

func (s *Server) deleteEvent(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	if err := s.store.DeleteEvent(r.Context(), householdFrom(r).ID, id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "event not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete event")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) listRoutines(w http.ResponseWriter, r *http.Request) {
	routines, err := s.store.ListRoutines(r.Context(), householdFrom(r).ID)
	if err != nil {
		s.logger.Error("list routines", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list routines")
		return
	}
	writeJSON(w, http.StatusOK, routines)
}

func (s *Server) createRoutine(w http.ResponseWriter, r *http.Request) {
	var input store.RoutineInput
	if !decodeJSON(w, r, &input) {
		return
	}
	routine, err := s.store.CreateRoutine(r.Context(), householdFrom(r).ID, userFrom(r).ID, input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, routine)
}

func (s *Server) getRoutine(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	routine, err := s.store.GetRoutine(r.Context(), householdFrom(r).ID, id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "routine not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load routine")
		return
	}
	writeJSON(w, http.StatusOK, routine)
}

func (s *Server) updateRoutine(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	var input store.RoutineInput
	if !decodeJSON(w, r, &input) {
		return
	}
	routine, err := s.store.UpdateRoutine(r.Context(), householdFrom(r).ID, id, input)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "routine not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, routine)
}

func (s *Server) archiveRoutine(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	if err := s.store.ArchiveRoutine(r.Context(), householdFrom(r).ID, id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "routine not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to archive routine")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) generateRoutineTask(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	task, err := s.store.GenerateRoutineTask(r.Context(), householdFrom(r).ID, userFrom(r).ID, id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "routine not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, task)
}

func (s *Server) listHouseholdLists(w http.ResponseWriter, r *http.Request) {
	lists, err := s.store.ListHouseholdLists(r.Context(), householdFrom(r).ID)
	if err != nil {
		s.logger.Error("list household lists", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list lists")
		return
	}
	writeJSON(w, http.StatusOK, lists)
}

func (s *Server) createHouseholdList(w http.ResponseWriter, r *http.Request) {
	var input store.HouseholdListInput
	if !decodeJSON(w, r, &input) {
		return
	}
	list, err := s.store.CreateHouseholdList(r.Context(), householdFrom(r).ID, userFrom(r).ID, input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, list)
}

func (s *Server) getHouseholdList(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	list, err := s.store.GetHouseholdList(r.Context(), householdFrom(r).ID, id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "list not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load list")
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) updateHouseholdList(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	var input store.HouseholdListInput
	if !decodeJSON(w, r, &input) {
		return
	}
	list, err := s.store.UpdateHouseholdList(r.Context(), householdFrom(r).ID, id, input)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "list not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) archiveHouseholdList(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	if err := s.store.ArchiveHouseholdList(r.Context(), householdFrom(r).ID, id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "list not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to archive list")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) listListItems(w http.ResponseWriter, r *http.Request, rawListID string) {
	listID, ok := parseID(w, rawListID)
	if !ok {
		return
	}
	items, err := s.store.ListListItems(r.Context(), householdFrom(r).ID, listID)
	if err != nil {
		s.logger.Error("list list items", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list items")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) createListItem(w http.ResponseWriter, r *http.Request, rawListID string) {
	listID, ok := parseID(w, rawListID)
	if !ok {
		return
	}
	var input store.ListItemInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.store.CreateListItem(r.Context(), householdFrom(r).ID, userFrom(r).ID, listID, input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) updateListItem(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	var input store.ListItemInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.store.UpdateListItem(r.Context(), householdFrom(r).ID, id, input)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "list item not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) completeListItem(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	item, err := s.store.CompleteListItem(r.Context(), householdFrom(r).ID, id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "list item not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to complete list item")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) reopenListItem(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	item, err := s.store.ReopenListItem(r.Context(), householdFrom(r).ID, id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "list item not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to reopen list item")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) archiveListItem(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	if err := s.store.ArchiveListItem(r.Context(), householdFrom(r).ID, id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "list item not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to archive list item")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) listContacts(w http.ResponseWriter, r *http.Request) {
	contacts, err := s.store.ListContacts(r.Context(), householdFrom(r).ID)
	if err != nil {
		s.logger.Error("list contacts", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list contacts")
		return
	}
	writeJSON(w, http.StatusOK, contacts)
}

func (s *Server) createContact(w http.ResponseWriter, r *http.Request) {
	var input store.ContactInput
	if !decodeJSON(w, r, &input) {
		return
	}
	contact, err := s.store.CreateContact(r.Context(), householdFrom(r).ID, userFrom(r).ID, input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, contact)
}

func (s *Server) getContact(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	contact, err := s.store.GetContact(r.Context(), householdFrom(r).ID, id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "contact not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load contact")
		return
	}
	writeJSON(w, http.StatusOK, contact)
}

func (s *Server) updateContact(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	var input store.ContactInput
	if !decodeJSON(w, r, &input) {
		return
	}
	contact, err := s.store.UpdateContact(r.Context(), householdFrom(r).ID, id, input)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "contact not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, contact)
}

func (s *Server) archiveContact(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	if err := s.store.ArchiveContact(r.Context(), householdFrom(r).ID, id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "contact not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to archive contact")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) listRelatedContacts(w http.ResponseWriter, r *http.Request) {
	entityID, err := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("entity_id")), 10, 64)
	if err != nil || entityID <= 0 {
		writeError(w, http.StatusBadRequest, "entity_id is required")
		return
	}
	input := store.RelatedItemInput{
		EntityType: strings.TrimSpace(r.URL.Query().Get("entity_type")),
		EntityID:   entityID,
	}
	contacts, err := s.store.ListRelatedContacts(r.Context(), householdFrom(r).ID, input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, contacts)
}

func (s *Server) linkRelatedContact(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ContactID  int64  `json:"contact_id"`
		EntityType string `json:"entity_type"`
		EntityID   int64  `json:"entity_id"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	err := s.store.LinkContact(r.Context(), householdFrom(r).ID, userFrom(r).ID, input.ContactID, store.RelatedItemInput{
		EntityType: input.EntityType,
		EntityID:   input.EntityID,
	})
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "contact not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func (s *Server) unlinkContact(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	if err := s.store.UnlinkContact(r.Context(), householdFrom(r).ID, id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "contact link not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to unlink contact")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) listAssets(w http.ResponseWriter, r *http.Request) {
	assets, err := s.store.ListAssets(r.Context(), householdFrom(r).ID)
	if err != nil {
		s.logger.Error("list assets", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list assets")
		return
	}
	writeJSON(w, http.StatusOK, assets)
}

func (s *Server) createAsset(w http.ResponseWriter, r *http.Request) {
	var input store.AssetInput
	if !decodeJSON(w, r, &input) {
		return
	}
	asset, err := s.store.CreateAsset(r.Context(), householdFrom(r).ID, userFrom(r).ID, input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, asset)
}

func (s *Server) getAsset(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	asset, err := s.store.GetAsset(r.Context(), householdFrom(r).ID, id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "asset not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load asset")
		return
	}
	writeJSON(w, http.StatusOK, asset)
}

func (s *Server) updateAsset(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	var input store.AssetInput
	if !decodeJSON(w, r, &input) {
		return
	}
	asset, err := s.store.UpdateAsset(r.Context(), householdFrom(r).ID, id, input)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "asset not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, asset)
}

func (s *Server) archiveAsset(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	if err := s.store.ArchiveAsset(r.Context(), householdFrom(r).ID, id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "asset not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to archive asset")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) listAssetMaintenanceItems(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	items, err := s.store.ListAssetMaintenanceItems(r.Context(), householdFrom(r).ID, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list maintenance")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) createAssetMaintenanceItem(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	var input store.AssetMaintenanceInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.store.CreateAssetMaintenanceItem(r.Context(), householdFrom(r).ID, userFrom(r).ID, id, input)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "asset not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) updateAssetMaintenanceItem(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	var input store.AssetMaintenanceInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.store.UpdateAssetMaintenanceItem(r.Context(), householdFrom(r).ID, id, input)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "maintenance item not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) archiveAssetMaintenanceItem(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	if err := s.store.ArchiveAssetMaintenanceItem(r.Context(), householdFrom(r).ID, id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "maintenance item not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to archive maintenance item")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) generateAssetMaintenanceTask(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	task, err := s.store.GenerateAssetMaintenanceItemTask(r.Context(), householdFrom(r).ID, userFrom(r).ID, id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "maintenance item not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, task)
}

func (s *Server) listRelatedAssets(w http.ResponseWriter, r *http.Request) {
	entityID, err := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("entity_id")), 10, 64)
	if err != nil || entityID <= 0 {
		writeError(w, http.StatusBadRequest, "entity_id is required")
		return
	}
	input := store.RelatedItemInput{
		EntityType: strings.TrimSpace(r.URL.Query().Get("entity_type")),
		EntityID:   entityID,
	}
	assets, err := s.store.ListRelatedAssets(r.Context(), householdFrom(r).ID, input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, assets)
}

func (s *Server) linkRelatedAsset(w http.ResponseWriter, r *http.Request) {
	var input struct {
		AssetID    int64  `json:"asset_id"`
		EntityType string `json:"entity_type"`
		EntityID   int64  `json:"entity_id"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	err := s.store.LinkAsset(r.Context(), householdFrom(r).ID, userFrom(r).ID, input.AssetID, store.RelatedItemInput{
		EntityType: input.EntityType,
		EntityID:   input.EntityID,
	})
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "asset not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func (s *Server) unlinkAsset(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	if err := s.store.UnlinkAsset(r.Context(), householdFrom(r).ID, id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "asset link not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to unlink asset")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) listDocuments(w http.ResponseWriter, r *http.Request) {
	documents, err := s.store.ListDocuments(r.Context(), householdFrom(r).ID)
	if err != nil {
		s.logger.Error("list documents", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list documents")
		return
	}
	writeJSON(w, http.StatusOK, documents)
}

func (s *Server) createDocument(w http.ResponseWriter, r *http.Request) {
	if isMultipart(r) {
		s.createUploadedDocument(w, r)
		return
	}

	var input store.DocumentInput
	if !decodeJSON(w, r, &input) {
		return
	}
	document, err := s.store.CreateDocument(r.Context(), householdFrom(r).ID, userFrom(r).ID, input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, document)
}

func (s *Server) createUploadedDocument(w http.ResponseWriter, r *http.Request) {
	input, cleanup, err := s.documentInputFromMultipart(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer func() {
		if cleanup != nil {
			cleanup()
		}
	}()

	document, err := s.store.CreateDocument(r.Context(), householdFrom(r).ID, userFrom(r).ID, input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	cleanup = nil
	writeJSON(w, http.StatusCreated, document)
}

func (s *Server) getDocument(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	document, err := s.store.GetDocument(r.Context(), householdFrom(r).ID, id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load document")
		return
	}
	writeJSON(w, http.StatusOK, document)
}

func (s *Server) updateDocument(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	if isMultipart(r) {
		s.updateUploadedDocument(w, r, id)
		return
	}

	var input store.DocumentInput
	if !decodeJSON(w, r, &input) {
		return
	}
	document, err := s.store.UpdateDocument(r.Context(), householdFrom(r).ID, id, input)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, document)
}

func (s *Server) updateUploadedDocument(w http.ResponseWriter, r *http.Request, id int64) {
	existing, err := s.store.GetDocument(r.Context(), householdFrom(r).ID, id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load document")
		return
	}

	input, cleanup, err := s.documentInputFromMultipart(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer func() {
		if cleanup != nil {
			cleanup()
		}
	}()

	document, err := s.store.UpdateDocument(r.Context(), householdFrom(r).ID, id, input)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if cleanup != nil {
		cleanup = nil
		if existing.FilePath != "" && existing.FilePath != document.FilePath {
			s.removeUploadedDocument(existing.FilePath)
		}
	}
	writeJSON(w, http.StatusOK, document)
}

func (s *Server) downloadDocument(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	document, err := s.store.GetDocument(r.Context(), householdFrom(r).ID, id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load document")
		return
	}
	if document.Status == "archived" {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}
	if document.FilePath == "" {
		writeError(w, http.StatusNotFound, "document has no uploaded file")
		return
	}

	path, err := s.uploadedDocumentPath(document.FilePath)
	if err != nil {
		s.logger.Warn("invalid document path", "document_id", document.ID, "path", document.FilePath, "error", err)
		writeError(w, http.StatusNotFound, "document file not found")
		return
	}
	w.Header().Set("Content-Type", documentContentType(document))
	w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": document.FileName}))
	http.ServeFile(w, r, path)
}

func (s *Server) archiveDocument(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	if err := s.store.ArchiveDocument(r.Context(), householdFrom(r).ID, id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "document not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to archive document")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) listRelatedItemsForDocument(w http.ResponseWriter, r *http.Request, rawDocumentID string) {
	documentID, ok := parseID(w, rawDocumentID)
	if !ok {
		return
	}
	items, err := s.store.ListRelatedItemsForDocument(r.Context(), householdFrom(r).ID, documentID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}
	if err != nil {
		s.logger.Error("list document related items", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list related items")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) linkRelatedItemToDocument(w http.ResponseWriter, r *http.Request, rawDocumentID string) {
	documentID, ok := parseID(w, rawDocumentID)
	if !ok {
		return
	}
	var input store.RelatedItemInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.store.LinkDocument(r.Context(), householdFrom(r).ID, userFrom(r).ID, documentID, input)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) listRelatedDocuments(w http.ResponseWriter, r *http.Request) {
	entityID, err := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("entity_id")), 10, 64)
	if err != nil || entityID <= 0 {
		writeError(w, http.StatusBadRequest, "entity_id is required")
		return
	}
	input := store.RelatedItemInput{
		EntityType: strings.TrimSpace(r.URL.Query().Get("entity_type")),
		EntityID:   entityID,
	}
	documents, err := s.store.ListRelatedDocuments(r.Context(), householdFrom(r).ID, input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, documents)
}

func (s *Server) linkRelatedDocument(w http.ResponseWriter, r *http.Request) {
	var input struct {
		DocumentID int64  `json:"document_id"`
		EntityType string `json:"entity_type"`
		EntityID   int64  `json:"entity_id"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.store.LinkDocument(r.Context(), householdFrom(r).ID, userFrom(r).ID, input.DocumentID, store.RelatedItemInput{
		EntityType: input.EntityType,
		EntityID:   input.EntityID,
	})
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) unlinkDocument(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	if err := s.store.UnlinkDocument(r.Context(), householdFrom(r).ID, id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "document link not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to unlink document")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) listModule(w http.ResponseWriter, r *http.Request, name string) {
	items, err := s.store.ListModuleItems(r.Context(), householdFrom(r).ID, name)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "module not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list module")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) documentInputFromMultipart(w http.ResponseWriter, r *http.Request) (store.DocumentInput, func(), error) {
	maxBody := s.cfg.DocumentMaxBytes + 1024*1024
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	if err := r.ParseMultipartForm(s.cfg.DocumentMaxBytes); err != nil {
		return store.DocumentInput{}, nil, fmt.Errorf("upload is too large or invalid")
	}

	input := store.DocumentInput{
		Title:       r.FormValue("title"),
		Description: r.FormValue("description"),
		URL:         r.FormValue("url"),
		Kind:        r.FormValue("kind"),
		Status:      r.FormValue("status"),
	}

	file, header, err := r.FormFile("file")
	if errors.Is(err, http.ErrMissingFile) {
		return input, nil, nil
	}
	if err != nil {
		return store.DocumentInput{}, nil, err
	}
	defer file.Close()
	if header.Filename == "" {
		return input, nil, nil
	}

	metadata, cleanup, err := s.saveUploadedDocument(file, header.Filename, header.Header.Get("Content-Type"), householdFrom(r).ID)
	if err != nil {
		return store.DocumentInput{}, nil, err
	}
	input.FileName = metadata.FileName
	input.FilePath = metadata.FilePath
	input.ContentType = metadata.ContentType
	input.FileSize = metadata.FileSize
	if strings.TrimSpace(input.Title) == "" {
		input.Title = metadata.FileName
	}
	return input, cleanup, nil
}

type uploadedDocumentMetadata struct {
	FileName    string
	FilePath    string
	ContentType string
	FileSize    int64
}

func (s *Server) saveUploadedDocument(file io.Reader, originalName, contentType string, householdID int64) (uploadedDocumentMetadata, func(), error) {
	fileName := cleanUploadName(originalName)
	if fileName == "" {
		return uploadedDocumentMetadata{}, nil, errors.New("file name is required")
	}

	token, err := randomUploadToken()
	if err != nil {
		return uploadedDocumentMetadata{}, nil, err
	}
	relativePath := filepath.Join(strconv.FormatInt(householdID, 10), token+strings.ToLower(filepath.Ext(fileName)))
	fullPath, err := s.uploadedDocumentPath(relativePath)
	if err != nil {
		return uploadedDocumentMetadata{}, nil, err
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o770); err != nil {
		return uploadedDocumentMetadata{}, nil, err
	}

	out, err := os.OpenFile(fullPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o660)
	if err != nil {
		return uploadedDocumentMetadata{}, nil, err
	}
	defer out.Close()

	written, err := io.Copy(out, io.LimitReader(file, s.cfg.DocumentMaxBytes+1))
	if err != nil {
		_ = os.Remove(fullPath)
		return uploadedDocumentMetadata{}, nil, err
	}
	if written > s.cfg.DocumentMaxBytes {
		_ = os.Remove(fullPath)
		return uploadedDocumentMetadata{}, nil, fmt.Errorf("file must be %d MB or smaller", s.cfg.DocumentMaxBytes/(1024*1024))
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}

	cleanup := func() {
		_ = os.Remove(fullPath)
	}
	return uploadedDocumentMetadata{
		FileName:    fileName,
		FilePath:    filepath.ToSlash(relativePath),
		ContentType: contentType,
		FileSize:    written,
	}, cleanup, nil
}

func (s *Server) uploadedDocumentPath(relativePath string) (string, error) {
	cleanRelative := filepath.Clean(relativePath)
	if cleanRelative == "." || filepath.IsAbs(cleanRelative) || strings.HasPrefix(cleanRelative, ".."+string(filepath.Separator)) || cleanRelative == ".." {
		return "", errors.New("invalid upload path")
	}

	root, err := filepath.Abs(s.cfg.DocumentUploadDir)
	if err != nil {
		return "", err
	}
	fullPath := filepath.Join(root, cleanRelative)
	rel, err := filepath.Rel(root, fullPath)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("invalid upload path")
	}
	return fullPath, nil
}

func (s *Server) removeUploadedDocument(relativePath string) {
	path, err := s.uploadedDocumentPath(relativePath)
	if err != nil {
		return
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		s.logger.Warn("remove uploaded document", "path", relativePath, "error", err)
	}
}

func isMultipart(r *http.Request) bool {
	return strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/form-data")
}

func cleanUploadName(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == '/' || r == '\\' {
			return -1
		}
		return r
	}, name)
	return strings.TrimSpace(name)
}

func documentContentType(document store.Document) string {
	contentType := strings.TrimSpace(document.ContentType)
	if contentType != "" && contentType != "application/octet-stream" {
		return contentType
	}
	if extType := mime.TypeByExtension(strings.ToLower(filepath.Ext(document.FileName))); extType != "" {
		return extType
	}
	if contentType != "" {
		return contentType
	}
	return "application/octet-stream"
}

func randomUploadToken() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func parseID(w http.ResponseWriter, raw string) (int64, bool) {
	id, err := strconv.ParseInt(strings.Trim(raw, "/"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return id, true
}

func setSessionCookie(w http.ResponseWriter, cfg config.Config, session store.Session) {
	http.SetCookie(w, &http.Cookie{
		Name:     cfg.SessionCookieName,
		Value:    session.ID,
		Path:     "/",
		Expires:  session.ExpiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   cfg.Env == "production",
	})
}

func clearSessionCookie(w http.ResponseWriter, cfg config.Config) {
	http.SetCookie(w, &http.Cookie{
		Name:     cfg.SessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   cfg.Env == "production",
	})
}

func getenv(key string) string {
	return os.Getenv(key)
}

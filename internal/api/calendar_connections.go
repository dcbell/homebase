package api

import (
	"errors"
	"net/http"

	"homebase/internal/caldav"
	"homebase/internal/store"
)

// createdCalendarConnection is the only response that ever carries a Calendar
// Connection's secret.
type createdCalendarConnection struct {
	Connection store.CalendarConnection `json:"connection"`
	ServerURL  string                   `json:"server_url"`
	Username   string                   `json:"username"`
	Secret     string                   `json:"secret"`
}

func (s *Server) requireBrowserSession(w http.ResponseWriter, r *http.Request) bool {
	if authKindFrom(r) != authKindSession {
		writeError(w, http.StatusForbidden, "Calendar Connection management requires a browser session")
		return false
	}
	return true
}

func (s *Server) listCalendarConnections(w http.ResponseWriter, r *http.Request) {
	if !s.requireBrowserSession(w, r) {
		return
	}
	connections, err := s.store.ListCalendarConnections(r.Context(), userFrom(r).ID, householdFrom(r).ID)
	if err != nil {
		s.logger.Error("list calendar connections", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list Calendar Connections")
		return
	}
	writeJSON(w, http.StatusOK, connections)
}

func (s *Server) createCalendarConnection(w http.ResponseWriter, r *http.Request) {
	if !s.requireBrowserSession(w, r) {
		return
	}
	var input store.CalendarConnectionInput
	if !decodeJSON(w, r, &input) {
		return
	}
	// iPhone setup profiles arrive with #37; until then only manual setup.
	if input.Kind != store.CalendarConnectionOther {
		writeError(w, http.StatusBadRequest, "kind must be other")
		return
	}
	created, err := s.store.CreateCalendarConnection(r.Context(), userFrom(r).ID, householdFrom(r).ID, input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, createdCalendarConnection{
		Connection: created.CalendarConnection,
		ServerURL:  s.cfg.WebBaseURL + caldav.Root,
		Username:   userFrom(r).Email,
		Secret:     created.Secret,
	})
}

func (s *Server) revokeCalendarConnection(w http.ResponseWriter, r *http.Request, rawID string) {
	if !s.requireBrowserSession(w, r) {
		return
	}
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	if err := s.store.RevokeCalendarConnection(r.Context(), userFrom(r).ID, householdFrom(r).ID, id, s.clock.Now()); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "Calendar Connection not found")
			return
		}
		s.logger.Error("revoke calendar connection", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to disconnect Calendar Connection")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

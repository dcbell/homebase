package api

import (
	"errors"
	"net/http"
	"time"

	"homebase/internal/caldav"
	"homebase/internal/store"
)

// calendarProfilePath is where setup profile links are served. It needs no
// session: Safari fetches it, and the token is the credential.
const calendarProfilePath = "/api/v1/calendar-profiles/"

// calendarProfileLinkTTL is how long a setup profile link works. The web form
// sends the browser straight to it, so it only has to outlast a slow tap.
const calendarProfileLinkTTL = 10 * time.Minute

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
	if input.Kind == store.CalendarConnectionIPhone {
		s.createIPhoneConnection(w, r, input)
		return
	}
	created, err := s.store.CreateCalendarConnection(r.Context(), userFrom(r).ID, householdFrom(r).ID, input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, store.CreatedCalendarConnection{
		Connection: created.CalendarConnection,
		ServerURL:  s.cfg.WebBaseURL + caldav.Root,
		Username:   userFrom(r).Email,
		Secret:     created.Secret,
	})
}

func (s *Server) createIPhoneConnection(w http.ResponseWriter, r *http.Request, input store.CalendarConnectionInput) {
	connection, token, err := s.store.CreateCalendarConnectionWithProfileLink(r.Context(), userFrom(r).ID, householdFrom(r).ID, input,
		s.clock.Now().Add(calendarProfileLinkTTL))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, store.CreatedIPhoneConnection{
		Connection: connection,
		ProfileURL: s.cfg.WebBaseURL + calendarProfilePath + token,
	})
}

// downloadCalendarProfile serves an iPhone setup profile once, issuing the
// connection's secret inside it.
func (s *Server) downloadCalendarProfile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	profile, err := s.store.RedeemCalendarProfileLink(r.Context(), r.PathValue("token"), s.clock.Now())
	if errors.Is(err, store.ErrNotFound) {
		http.Error(w, "This setup link has expired or was already used. Set up your iPhone again from Settings.", http.StatusNotFound)
		return
	}
	if err != nil {
		s.logger.Error("redeem calendar profile link", "error", err)
		http.Error(w, "The setup profile could not be made.", http.StatusInternalServerError)
		return
	}
	body, err := caldav.Profile{
		WebBaseURL:   s.cfg.WebBaseURL,
		ConnectionID: profile.Connection.ID,
		DeviceName:   profile.Connection.Name,
		UserID:       profile.User.ID,
		Username:     profile.User.Email,
		Secret:       profile.Secret,
	}.Render()
	if err != nil {
		s.logger.Error("render calendar profile", "error", err)
		http.Error(w, "The setup profile could not be made.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", caldav.ProfileContentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+caldav.ProfileFilename+`"`)
	_, _ = w.Write(body)
}

func (s *Server) disconnectCalendarConnection(w http.ResponseWriter, r *http.Request, rawID string) {
	if !s.requireBrowserSession(w, r) {
		return
	}
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	if err := s.store.DisconnectCalendarConnection(r.Context(), userFrom(r).ID, householdFrom(r).ID, id, s.clock.Now()); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "Calendar Connection not found")
			return
		}
		s.logger.Error("disconnect calendar connection", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to disconnect Calendar Connection")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

package main

import (
	"net/http"
	"net/url"

	"homebase/internal/store"
)

// settingsCreated is what a Settings form just created, shown once on the
// page it renders.
type settingsCreated struct {
	APIToken store.APITokenWithSecret
	Calendar createdCalendarConnection
}

// createdCalendarConnection is the API's answer to creating a Calendar
// Connection: the only time its secret is available.
type createdCalendarConnection struct {
	Connection store.CalendarConnection `json:"connection"`
	ServerURL  string                   `json:"server_url"`
	Username   string                   `json:"username"`
	Secret     string                   `json:"secret"`
}

func (s *webServer) calendarConnectionRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /settings/calendar-connections", s.createCalendarConnection)
	mux.HandleFunc("POST /settings/calendar-connections/{id}/disconnect", s.disconnectCalendarConnection)
}

// createCalendarConnection connects an Other device and renders Settings with
// its connection details, which are never shown again.
func (s *webServer) createCalendarConnection(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.renderSettings(w, r, "The form could not be read.", settingsCreated{})
		return
	}
	payload := map[string]any{
		"name": r.FormValue("name"),
		"kind": store.CalendarConnectionOther,
	}
	var created createdCalendarConnection
	if err := s.apiJSON(r, http.MethodPost, "/api/v1/calendar-connections", payload, &created); err != nil {
		s.renderSettings(w, r, err.Error(), settingsCreated{})
		return
	}
	s.renderSettings(w, r, "", settingsCreated{Calendar: created})
}

func (s *webServer) disconnectCalendarConnection(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.apiJSON(r, http.MethodDelete, "/api/v1/calendar-connections/"+url.PathEscape(id), nil, nil); err != nil {
		http.Redirect(w, r, "/settings?error="+url.QueryEscape(err.Error())+"#calendar", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/settings#calendar", http.StatusSeeOther)
}

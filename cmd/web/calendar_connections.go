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
	Calendar store.CreatedCalendarConnection
}

func (s *webServer) calendarConnectionRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /settings/calendar-connections", s.createCalendarConnection)
	mux.HandleFunc("POST /settings/calendar-connections/{id}/disconnect", s.disconnectCalendarConnection)
}

// createCalendarConnection connects a device. For an Other device it renders
// Settings with the connection details, which are never shown again. For an
// iPhone it sends the browser to the setup profile download.
func (s *webServer) createCalendarConnection(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.renderSettings(w, r, "The form could not be read.", settingsCreated{})
		return
	}
	if r.FormValue("kind") == store.CalendarConnectionIPhone {
		s.setUpIPhone(w, r)
		return
	}
	payload := map[string]any{
		"name": r.FormValue("name"),
		"kind": store.CalendarConnectionOther,
	}
	var created store.CreatedCalendarConnection
	if err := s.apiJSON(r, http.MethodPost, "/api/v1/calendar-connections", payload, &created); err != nil {
		s.renderSettings(w, r, err.Error(), settingsCreated{})
		return
	}
	s.renderSettings(w, r, "", settingsCreated{Calendar: created})
}

func (s *webServer) setUpIPhone(w http.ResponseWriter, r *http.Request) {
	payload := map[string]any{
		"name": r.FormValue("name"),
		"kind": store.CalendarConnectionIPhone,
	}
	var created store.CreatedIPhoneConnection
	if err := s.apiJSON(r, http.MethodPost, "/api/v1/calendar-connections", payload, &created); err != nil {
		s.renderSettings(w, r, err.Error(), settingsCreated{})
		return
	}
	// The link is a download, not a page that could show the Saved toast,
	// and the API serves it as given.
	skipSavedSignal(w)
	http.Redirect(w, r, created.ProfileURL, http.StatusSeeOther)
}

func (s *webServer) disconnectCalendarConnection(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.apiJSON(r, http.MethodDelete, "/api/v1/calendar-connections/"+url.PathEscape(id), nil, nil); err != nil {
		http.Redirect(w, r, "/settings?error="+url.QueryEscape(err.Error())+"#calendar", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/settings#calendar", http.StatusSeeOther)
}

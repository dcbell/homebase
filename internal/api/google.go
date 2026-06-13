package api

import "net/http"

func (s *Server) googleCalendarConnect(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.GoogleConfigured() {
		writeError(w, http.StatusPreconditionFailed, "Google OAuth is not configured")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "connected_by_login",
		"message": "Calendar scope is requested during Google login; background sync is the next integration step.",
	})
}

func (s *Server) googleCalendarSync(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.GoogleConfigured() {
		writeError(w, http.StatusPreconditionFailed, "Google OAuth is not configured")
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "queued",
		"message": "Calendar sync endpoint is reserved for the worker-backed two-way sync job.",
	})
}

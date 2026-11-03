package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"homebase/internal/store"
)

// The page and service worker call these same-origin routes instead of
// /api/v1 directly, so Push works whether or not a reverse proxy puts the API
// on the web origin (in docker compose dev it doesn't). Each one forwards to
// the API, which owns every rule (ADR 0001).

func (s *webServer) pushRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /push/vapid-public-key", func(w http.ResponseWriter, r *http.Request) {
		s.forwardJSON(w, r, http.MethodGet, "/api/v1/push/vapid-public-key")
	})
	mux.HandleFunc("POST /push-subscriptions", func(w http.ResponseWriter, r *http.Request) {
		s.forwardJSON(w, r, http.MethodPost, "/api/v1/push-subscriptions")
	})
	mux.HandleFunc("DELETE /push-subscriptions/{id}", func(w http.ResponseWriter, r *http.Request) {
		s.forwardJSON(w, r, http.MethodDelete, "/api/v1/push-subscriptions/"+url.PathEscape(r.PathValue("id")))
	})
	mux.HandleFunc("POST /settings/push-subscriptions/{id}/remove", s.removePushSubscription)
	mux.HandleFunc("POST /notifications/{id}/read", s.readNotification)
}

// forwardJSON sends the request body to the API with the session cookie and
// the browser's User-Agent (the API names devices from it), and relays the
// API's status and JSON body unchanged.
func (s *webServer) forwardJSON(w http.ResponseWriter, r *http.Request, method, path string) {
	req, err := http.NewRequestWithContext(r.Context(), method, s.cfg.APIInternalURL+path, r.Body)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	req.Header.Set("Accept", "application/json")
	if r.Body != nil && r.ContentLength != 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", r.UserAgent())
	if cookie, err := r.Cookie(s.cfg.SessionCookieName); err == nil {
		req.AddCookie(cookie)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		s.logger.Error("forward to api", "path", path, "error", err)
		writeJSONError(w, http.StatusBadGateway, "the API is unavailable")
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// removePushSubscription is the Remove button next to a device in Settings.
func (s *webServer) removePushSubscription(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.apiJSON(r, http.MethodDelete, "/api/v1/push-subscriptions/"+url.PathEscape(id), nil, nil); err != nil {
		http.Redirect(w, r, "/settings?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

// readNotification marks a Notification read for the service worker when a
// Push is tapped, and answers with the URL to open and the new Unread count
// for the app-icon badge.
func (s *webServer) readNotification(w http.ResponseWriter, r *http.Request) {
	var notification store.Notification
	if err := s.apiJSON(r, http.MethodPost, "/api/v1/notifications/"+url.PathEscape(r.PathValue("id"))+"/read", nil, &notification); err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, errUnauthorized) {
			status = http.StatusUnauthorized
		}
		writeJSONError(w, status, err.Error())
		return
	}
	var list store.NotificationList
	if err := s.apiJSON(r, http.MethodGet, "/api/v1/notifications?limit=1", nil, &list); err != nil {
		writeJSONError(w, http.StatusBadGateway, err.Error())
		return
	}
	target := notification.URL
	if !strings.HasPrefix(target, "/") || strings.HasPrefix(target, "//") {
		target = "/"
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"url":          target,
		"unread_count": list.UnreadCount,
	})
}

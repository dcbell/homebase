package main

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"homebase/internal/store"
)

// openNotification marks a Notification read and opens its subject. The bell
// links here instead of straight to the subject so that tapping marks it read.
func (s *webServer) openNotification(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var notification store.Notification
	if err := s.apiJSON(r, http.MethodPost, "/api/v1/notifications/"+url.PathEscape(id)+"/read", nil, &notification); err != nil {
		if errors.Is(err, errUnauthorized) {
			redirectToSignIn(w, r)
			return
		}
		http.Redirect(w, r, returnPath(r)+"?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	target := notification.URL
	if !strings.HasPrefix(target, "/") || strings.HasPrefix(target, "//") {
		target = "/"
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// markAllNotificationsRead clears the bell and returns to the page it was on.
func (s *webServer) markAllNotificationsRead(w http.ResponseWriter, r *http.Request) {
	back := returnPath(r)
	if err := s.apiJSON(r, http.MethodPost, "/api/v1/notifications/read-all", nil, nil); err != nil {
		http.Redirect(w, r, back+"?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}

// returnPath is the same-site page the request came from, or "/".
func returnPath(r *http.Request) string {
	referer, err := url.Parse(r.Referer())
	if err != nil || referer.Path == "" || !strings.HasPrefix(referer.Path, "/") || strings.HasPrefix(referer.Path, "//") {
		return "/"
	}
	if referer.Host != "" && referer.Host != r.Host {
		return "/"
	}
	return referer.Path
}

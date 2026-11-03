package api

import (
	"errors"
	"net/http"
	"strings"

	"homebase/internal/push"
	"homebase/internal/store"
)

func (s *Server) vapidPublicKey(w http.ResponseWriter, r *http.Request) {
	keys, err := push.EnsureVAPIDKeys(r.Context(), s.store)
	if err != nil {
		s.logger.Error("load VAPID keys", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to load VAPID public key")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"public_key": keys.PublicKey})
}

func (s *Server) listPushSubscriptions(w http.ResponseWriter, r *http.Request) {
	subs, err := s.store.ListPushSubscriptions(r.Context(), householdFrom(r).ID, userFrom(r).ID)
	if err != nil {
		s.logger.Error("list push subscriptions", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list push subscriptions")
		return
	}
	writeJSON(w, http.StatusOK, subs)
}

// registerPushSubscription stores the PushSubscription.toJSON() of the
// current browser. It needs a browser session: a Push Subscription belongs to
// a device someone signed in on, not to an API client.
func (s *Server) registerPushSubscription(w http.ResponseWriter, r *http.Request) {
	if authKindFrom(r) != authKindSession {
		writeError(w, http.StatusForbidden, "registering a push subscription requires a browser session")
		return
	}
	var input struct {
		Endpoint string `json:"endpoint"`
		Keys     struct {
			P256dh string `json:"p256dh"`
			Auth   string `json:"auth"`
		} `json:"keys"`
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Endpoint = strings.TrimSpace(input.Endpoint)
	if input.Endpoint == "" || input.Keys.P256dh == "" || input.Keys.Auth == "" {
		writeError(w, http.StatusBadRequest, "endpoint, keys.p256dh and keys.auth are required")
		return
	}
	if !strings.HasPrefix(input.Endpoint, "https://") {
		writeError(w, http.StatusBadRequest, "endpoint must be an https URL")
		return
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = deviceName(r.UserAgent())
	}
	sub, created, err := s.store.SavePushSubscription(r.Context(), householdFrom(r).ID, userFrom(r).ID, store.PushSubscriptionInput{
		Endpoint: input.Endpoint,
		P256dh:   input.Keys.P256dh,
		Auth:     input.Keys.Auth,
		Name:     name,
	}, s.clock.Now())
	if err != nil {
		s.logger.Error("register push subscription", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to register push subscription")
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, sub)
}

func (s *Server) deletePushSubscription(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseID(w, rawID)
	if !ok {
		return
	}
	err := s.store.DeletePushSubscription(r.Context(), householdFrom(r).ID, userFrom(r).ID, id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "push subscription not found")
		return
	}
	if err != nil {
		s.logger.Error("delete push subscription", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete push subscription")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) getPushSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.store.PushSettings(r.Context(), householdFrom(r).ID, userFrom(r).ID)
	if err != nil {
		s.logger.Error("load push settings", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to load push settings")
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) updatePushSettings(w http.ResponseWriter, r *http.Request) {
	var input store.PushSettingsInput
	if !decodeJSON(w, r, &input) {
		return
	}
	settings, err := s.store.UpdatePushSettings(r.Context(), householdFrom(r).ID, userFrom(r).ID, input)
	if err != nil {
		s.logger.Error("update push settings", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update push settings")
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

// deviceName turns a User-Agent into a name a Member recognizes, such as
// "Safari on iPhone".
func deviceName(userAgent string) string {
	ua := strings.ToLower(userAgent)
	device := ""
	switch {
	case strings.Contains(ua, "iphone"):
		device = "iPhone"
	case strings.Contains(ua, "ipad"):
		device = "iPad"
	case strings.Contains(ua, "android"):
		device = "Android"
	case strings.Contains(ua, "cros"):
		device = "ChromeOS"
	case strings.Contains(ua, "macintosh") || strings.Contains(ua, "mac os x"):
		device = "Mac"
	case strings.Contains(ua, "windows"):
		device = "Windows"
	case strings.Contains(ua, "linux"):
		device = "Linux"
	}
	browser := ""
	switch {
	case strings.Contains(ua, "edg/") || strings.Contains(ua, "edga/") || strings.Contains(ua, "edgios/"):
		browser = "Edge"
	case strings.Contains(ua, "firefox/") || strings.Contains(ua, "fxios/"):
		browser = "Firefox"
	case strings.Contains(ua, "samsungbrowser/"):
		browser = "Samsung Internet"
	case strings.Contains(ua, "chrome/") || strings.Contains(ua, "crios/"):
		browser = "Chrome"
	case strings.Contains(ua, "safari/"):
		browser = "Safari"
	}
	switch {
	case browser != "" && device != "":
		return browser + " on " + device
	case device != "":
		return device
	case browser != "":
		return browser
	default:
		return "Unknown device"
	}
}

package api

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"homebase/internal/store"
)

type googleUserInfo struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

func (s *Server) googleStart(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.GoogleConfigured() {
		http.Redirect(w, r, "/auth/dev-login", http.StatusFound)
		return
	}

	state, err := randomState()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create auth state")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "homebase_oauth_state",
		Value:    state,
		Path:     "/",
		Expires:  time.Now().Add(10 * time.Minute),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.cfg.Env == "production",
	})

	http.Redirect(w, r, s.oauthConfig().AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce), http.StatusFound)
}

func (s *Server) googleCallback(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.GoogleConfigured() {
		s.completeLogin(w, r, "dev-google-sub", "demo@example.com", "Demo User", "")
		return
	}

	stateCookie, err := r.Cookie("homebase_oauth_state")
	if err != nil || stateCookie.Value == "" || r.URL.Query().Get("state") != stateCookie.Value {
		writeError(w, http.StatusBadRequest, "invalid auth state")
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "missing auth code")
		return
	}

	token, err := s.oauthConfig().Exchange(r.Context(), code)
	if err != nil {
		s.logger.Error("exchange google code", "error", err)
		writeError(w, http.StatusBadGateway, "failed to exchange Google auth code")
		return
	}

	info, err := fetchGoogleUserInfo(r.Context(), s.oauthConfig().Client(r.Context(), token))
	if err != nil {
		s.logger.Error("fetch google userinfo", "error", err)
		writeError(w, http.StatusBadGateway, "failed to load Google profile")
		return
	}

	user, household, err := s.store.ActivatePreauthorizedGoogleUser(r.Context(), info.ID, info.Email, info.Name, info.Picture)
	if errors.Is(err, store.ErrNotFound) {
		http.Redirect(w, r, s.cfg.WebBaseURL+"/?error=not_added", http.StatusFound)
		return
	}
	if err != nil {
		s.logger.Error("activate google user", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to login")
		return
	}

	if token.AccessToken != "" {
		if err := s.store.UpsertGoogleToken(r.Context(), user.ID, token.AccessToken, token.RefreshToken, token.Expiry); err != nil {
			s.logger.Warn("store google token", "error", err)
		}
	}

	s.finishLogin(w, r, user, household)
}

func (s *Server) devLogin(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Env == "production" {
		writeError(w, http.StatusNotFound, "route not found")
		return
	}
	s.completeLogin(w, r, "dev-google-sub", "demo@example.com", "Demo User", "")
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(s.cfg.SessionCookieName); err == nil && cookie.Value != "" {
		if err := s.store.DeleteSession(r.Context(), cookie.Value); err != nil {
			s.logger.Warn("delete session", "error", err)
		}
	}
	clearSessionCookie(w, s.cfg)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) completeLogin(w http.ResponseWriter, r *http.Request, googleSub, email, name, avatarURL string) {
	user, household, err := s.store.UpsertUserWithHousehold(r.Context(), googleSub, email, name, avatarURL)
	if err != nil {
		s.logger.Error("upsert user", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to login")
		return
	}

	s.finishLogin(w, r, user, household)
}

func (s *Server) finishLogin(w http.ResponseWriter, r *http.Request, user store.User, household store.Household) {
	session, err := s.store.CreateSession(r.Context(), user.ID, household.ID, 30*24*time.Hour)
	if err != nil {
		s.logger.Error("create session", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	setSessionCookie(w, s.cfg, session)
	http.Redirect(w, r, s.cfg.WebBaseURL, http.StatusFound)
}

func (s *Server) oauthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     s.cfg.GoogleClientID,
		ClientSecret: s.cfg.GoogleClientSecret,
		RedirectURL:  s.cfg.GoogleRedirectURL,
		Scopes:       s.cfg.GoogleCalendarScopes,
		Endpoint:     google.Endpoint,
	}
}

func fetchGoogleUserInfo(ctx context.Context, client *http.Client) (googleUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return googleUserInfo{}, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return googleUserInfo{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return googleUserInfo{}, fmt.Errorf("google userinfo status %d", resp.StatusCode)
	}

	var info googleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return googleUserInfo{}, err
	}
	if strings.TrimSpace(info.Email) == "" {
		return googleUserInfo{}, errors.New("google profile missing email")
	}
	return info, nil
}

func randomState() (string, error) {
	const bytes = 24
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", buf), nil
}

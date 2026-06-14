package api

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"homebase/internal/store"
)

type oauthDiscovery struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserInfoEndpoint      string `json:"userinfo_endpoint"`
}

type oauthProfile struct {
	Subject   string `json:"sub"`
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Picture   string `json:"picture"`
	AvatarURL string `json:"avatar_url"`
	Username  string `json:"preferred_username"`
}

func (s *Server) oauthStart(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.OAuthConfigured() {
		http.Redirect(w, r, "/auth/dev-login", http.StatusFound)
		return
	}

	oauthConfig, _, err := s.oauthConfig(r.Context())
	if err != nil {
		s.logger.Error("load oauth config", "error", err)
		writeError(w, http.StatusBadGateway, "failed to load OAuth provider configuration")
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

	http.Redirect(w, r, oauthConfig.AuthCodeURL(state), http.StatusFound)
}

func (s *Server) oauthCallback(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.OAuthConfigured() {
		s.completeLogin(w, r, "dev-oauth-sub", "demo@example.com", "Demo User", "")
		return
	}

	stateCookie, err := r.Cookie("homebase_oauth_state")
	if err != nil || stateCookie.Value == "" || r.URL.Query().Get("state") != stateCookie.Value {
		writeError(w, http.StatusBadRequest, "invalid auth state")
		return
	}

	oauthConfig, userInfoURL, err := s.oauthConfig(r.Context())
	if err != nil {
		s.logger.Error("load oauth config", "error", err)
		writeError(w, http.StatusBadGateway, "failed to load OAuth provider configuration")
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "missing auth code")
		return
	}

	token, err := oauthConfig.Exchange(r.Context(), code)
	if err != nil {
		s.logger.Error("exchange oauth code", "error", err)
		writeError(w, http.StatusBadGateway, "failed to exchange OAuth code")
		return
	}

	info, err := fetchOAuthUserInfo(r.Context(), oauthConfig.Client(r.Context(), token), userInfoURL)
	if err != nil {
		s.logger.Error("fetch oauth userinfo", "error", err)
		writeError(w, http.StatusBadGateway, "failed to load OAuth profile")
		return
	}

	user, household, err := s.store.ActivatePreauthorizedOAuthUser(r.Context(), info.providerSubject(), info.Email, info.displayName(), info.avatar())
	if errors.Is(err, store.ErrNotFound) {
		http.Redirect(w, r, s.cfg.WebBaseURL+"/?error=not_added", http.StatusFound)
		return
	}
	if err != nil {
		s.logger.Error("activate oauth user", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to login")
		return
	}

	s.finishLogin(w, r, user, household)
}

func (s *Server) devLogin(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Env == "production" {
		writeError(w, http.StatusNotFound, "route not found")
		return
	}
	s.completeLogin(w, r, "dev-oauth-sub", "demo@example.com", "Demo User", "")
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

func (s *Server) completeLogin(w http.ResponseWriter, r *http.Request, providerSubject, email, name, avatarURL string) {
	user, household, err := s.store.UpsertUserWithHousehold(r.Context(), providerSubject, email, name, avatarURL)
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

func (s *Server) oauthConfig(ctx context.Context) (*oauth2.Config, string, error) {
	authURL := s.cfg.OAuthAuthURL
	tokenURL := s.cfg.OAuthTokenURL
	userInfoURL := s.cfg.OAuthUserInfoURL
	if s.cfg.OAuthIssuerURL != "" {
		discovery, err := fetchOAuthDiscovery(ctx, s.cfg.OAuthIssuerURL)
		if err != nil {
			return nil, "", err
		}
		if authURL == "" {
			authURL = discovery.AuthorizationEndpoint
		}
		if tokenURL == "" {
			tokenURL = discovery.TokenEndpoint
		}
		if userInfoURL == "" {
			userInfoURL = discovery.UserInfoEndpoint
		}
	}
	if authURL == "" || tokenURL == "" || userInfoURL == "" {
		return nil, "", errors.New("OAuth auth, token, and userinfo endpoints are required")
	}
	return &oauth2.Config{
		ClientID:     s.cfg.OAuthClientID,
		ClientSecret: s.cfg.OAuthClientSecret,
		RedirectURL:  s.cfg.OAuthRedirectURL,
		Scopes:       s.cfg.OAuthScopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:  authURL,
			TokenURL: tokenURL,
		},
	}, userInfoURL, nil
}

func fetchOAuthDiscovery(ctx context.Context, issuerURL string) (oauthDiscovery, error) {
	discoveryURL, err := url.JoinPath(strings.TrimRight(issuerURL, "/"), ".well-known/openid-configuration")
	if err != nil {
		return oauthDiscovery{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return oauthDiscovery{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return oauthDiscovery{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return oauthDiscovery{}, fmt.Errorf("OAuth discovery status %d", resp.StatusCode)
	}
	var discovery oauthDiscovery
	if err := json.NewDecoder(resp.Body).Decode(&discovery); err != nil {
		return oauthDiscovery{}, err
	}
	if discovery.AuthorizationEndpoint == "" || discovery.TokenEndpoint == "" || discovery.UserInfoEndpoint == "" {
		return oauthDiscovery{}, errors.New("OAuth discovery document missing required endpoints")
	}
	return discovery, nil
}

func fetchOAuthUserInfo(ctx context.Context, client *http.Client, userInfoURL string) (oauthProfile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userInfoURL, nil)
	if err != nil {
		return oauthProfile{}, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return oauthProfile{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return oauthProfile{}, fmt.Errorf("OAuth userinfo status %d", resp.StatusCode)
	}

	var info oauthProfile
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return oauthProfile{}, err
	}
	if strings.TrimSpace(info.Email) == "" {
		return oauthProfile{}, errors.New("OAuth profile missing email")
	}
	return info, nil
}

func (p oauthProfile) providerSubject() string {
	subject := strings.TrimSpace(p.Subject)
	if subject == "" {
		subject = strings.TrimSpace(p.ID)
	}
	if subject == "" {
		subject = strings.TrimSpace(p.Email)
	}
	return subject
}

func (p oauthProfile) displayName() string {
	name := strings.TrimSpace(p.Name)
	if name == "" {
		name = strings.TrimSpace(p.Username)
	}
	if name == "" {
		name = strings.TrimSpace(p.Email)
	}
	return name
}

func (p oauthProfile) avatar() string {
	if strings.TrimSpace(p.Picture) != "" {
		return strings.TrimSpace(p.Picture)
	}
	return strings.TrimSpace(p.AvatarURL)
}

func randomState() (string, error) {
	const bytes = 24
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", buf), nil
}

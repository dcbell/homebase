package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env                  string
	APIAddr              string
	WebAddr              string
	APIBaseURL           string
	APIInternalURL       string
	WebBaseURL           string
	DatabaseURL          string
	SessionCookieName    string
	SessionSecret        string
	OAuthProviderName    string
	OAuthIssuerURL       string
	OAuthAuthURL         string
	OAuthTokenURL        string
	OAuthUserInfoURL     string
	OAuthClientID        string
	OAuthClientSecret    string
	OAuthRedirectURL     string
	OAuthScopes          []string
	BootstrapOwnerEmail  string
	BootstrapOwnerName   string
	BootstrapHousehold   string
	DocumentUploadDir    string
	DocumentMaxBytes     int64
	HTTPClientTimeout    time.Duration
	RoutineCheckInterval time.Duration
}

func Load() Config {
	webBaseURL := env("WEB_BASE_URL", "http://localhost:8080")
	apiBaseURL := env("API_BASE_URL", "http://localhost:8081")
	oauthProvider := strings.ToLower(env("OAUTH_PROVIDER_NAME", ""))
	oauthClientID := env("OAUTH_CLIENT_ID", "")
	oauthClientSecret := env("OAUTH_CLIENT_SECRET", "")
	if oauthProvider == "" && (oauthClientID != "" || oauthClientSecret != "") {
		oauthProvider = "oauth"
	}
	oauthRedirectURL := env("OAUTH_REDIRECT_URL", "")
	if oauthRedirectURL == "" {
		oauthRedirectURL = strings.TrimRight(apiBaseURL, "/") + "/auth/oauth/callback"
	}
	oauthScopes := splitList(env("OAUTH_SCOPES", "openid profile email"))
	oauthIssuerURL := env("OAUTH_ISSUER_URL", "")
	oauthAuthURL := env("OAUTH_AUTH_URL", "")
	oauthTokenURL := env("OAUTH_TOKEN_URL", "")
	oauthUserInfoURL := env("OAUTH_USERINFO_URL", "")

	cfg := Config{
		Env:                  env("APP_ENV", "development"),
		APIAddr:              env("API_ADDR", ":8081"),
		WebAddr:              env("WEB_ADDR", ":8080"),
		APIBaseURL:           strings.TrimRight(apiBaseURL, "/"),
		APIInternalURL:       strings.TrimRight(env("API_INTERNAL_URL", apiBaseURL), "/"),
		WebBaseURL:           strings.TrimRight(webBaseURL, "/"),
		DatabaseURL:          env("DATABASE_URL", "postgres://homebase:homebase@db:5432/homebase?sslmode=disable"),
		SessionCookieName:    env("SESSION_COOKIE_NAME", "homebase_session"),
		SessionSecret:        env("SESSION_SECRET", "dev-change-me"),
		OAuthProviderName:    oauthProvider,
		OAuthIssuerURL:       strings.TrimRight(oauthIssuerURL, "/"),
		OAuthAuthURL:         oauthAuthURL,
		OAuthTokenURL:        oauthTokenURL,
		OAuthUserInfoURL:     oauthUserInfoURL,
		OAuthClientID:        oauthClientID,
		OAuthClientSecret:    oauthClientSecret,
		OAuthRedirectURL:     oauthRedirectURL,
		OAuthScopes:          oauthScopes,
		BootstrapOwnerEmail:  env("BOOTSTRAP_OWNER_EMAIL", ""),
		BootstrapOwnerName:   env("BOOTSTRAP_OWNER_NAME", ""),
		BootstrapHousehold:   env("BOOTSTRAP_HOUSEHOLD_NAME", "My Household"),
		DocumentUploadDir:    env("DOCUMENT_UPLOAD_DIR", "/var/lib/homebase/uploads"),
		DocumentMaxBytes:     int64Env("DOCUMENT_MAX_UPLOAD_MB", 25) * 1024 * 1024,
		HTTPClientTimeout:    durationEnv("HTTP_CLIENT_TIMEOUT_SECONDS", 10*time.Second),
		RoutineCheckInterval: durationEnv("ROUTINE_CHECK_INTERVAL_SECONDS", 15*time.Minute),
	}

	return cfg
}

func (c Config) OAuthConfigured() bool {
	hasEndpoints := c.OAuthIssuerURL != "" || (c.OAuthAuthURL != "" && c.OAuthTokenURL != "" && c.OAuthUserInfoURL != "")
	return c.OAuthClientID != "" && c.OAuthClientSecret != "" && hasEndpoints
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	seconds, err := strconv.Atoi(value)
	if err != nil || seconds <= 0 {
		return fallback
	}

	return time.Duration(seconds) * time.Second
}

func int64Env(key string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}

	return parsed
}

func splitList(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\n' || r == '\t'
	})
	result := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" {
			result = append(result, field)
		}
	}
	return result
}

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
	GoogleClientID       string
	GoogleClientSecret   string
	GoogleRedirectURL    string
	GoogleCalendarScopes []string
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

	cfg := Config{
		Env:                env("APP_ENV", "development"),
		APIAddr:            env("API_ADDR", ":8081"),
		WebAddr:            env("WEB_ADDR", ":8080"),
		APIBaseURL:         strings.TrimRight(apiBaseURL, "/"),
		APIInternalURL:     strings.TrimRight(env("API_INTERNAL_URL", apiBaseURL), "/"),
		WebBaseURL:         strings.TrimRight(webBaseURL, "/"),
		DatabaseURL:        env("DATABASE_URL", "postgres://homebase:homebase@db:5432/homebase?sslmode=disable"),
		SessionCookieName:  env("SESSION_COOKIE_NAME", "homebase_session"),
		SessionSecret:      env("SESSION_SECRET", "dev-change-me"),
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL:  env("GOOGLE_REDIRECT_URL", strings.TrimRight(apiBaseURL, "/")+"/auth/google/callback"),
		GoogleCalendarScopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
			"https://www.googleapis.com/auth/calendar",
		},
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

func (c Config) GoogleConfigured() bool {
	return c.GoogleClientID != "" && c.GoogleClientSecret != ""
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

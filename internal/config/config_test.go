package config

import (
	"testing"
	"time"
)

func TestApplyTimezone(t *testing.T) {
	original := time.Local
	t.Cleanup(func() { time.Local = original })

	cfg := Config{Timezone: "America/Chicago"}
	if err := cfg.ApplyTimezone(); err != nil {
		t.Fatalf("ApplyTimezone() error = %v", err)
	}
	if got := time.Local.String(); got != "America/Chicago" {
		t.Fatalf("time.Local = %q, want America/Chicago", got)
	}
}

func TestApplyTimezoneRejectsUnknownLocation(t *testing.T) {
	cfg := Config{Timezone: "Not/A_Timezone"}
	if err := cfg.ApplyTimezone(); err == nil {
		t.Fatal("ApplyTimezone() error = nil, want invalid timezone error")
	}
}

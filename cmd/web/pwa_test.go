package main

import (
	"bytes"
	"encoding/json"
	"image/color"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"homebase/internal/config"
)

func newTestWebServer(t *testing.T) *webServer {
	t.Helper()
	return newWebServer(config.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestManifestDescribesAStandaloneApp(t *testing.T) {
	rec := get(t, newTestWebServer(t).routes(), "/manifest.webmanifest")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/manifest+json" {
		t.Fatalf("Content-Type = %q, want application/manifest+json", got)
	}
	var manifest struct {
		Name       string `json:"name"`
		StartURL   string `json:"start_url"`
		Scope      string `json:"scope"`
		Display    string `json:"display"`
		ThemeColor string `json:"theme_color"`
		Icons      []struct {
			Src     string `json:"src"`
			Sizes   string `json:"sizes"`
			Type    string `json:"type"`
			Purpose string `json:"purpose"`
		} `json:"icons"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &manifest); err != nil {
		t.Fatalf("manifest is not JSON: %v", err)
	}
	if manifest.Name != "Homebase" || manifest.Display != "standalone" || manifest.StartURL != "/" || manifest.Scope != "/" || manifest.ThemeColor != "#5865f2" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	want := map[string]string{
		"192x192 any":      "/icons/icon-192.png",
		"512x512 any":      "/icons/icon-512.png",
		"512x512 maskable": "/icons/icon-maskable-512.png",
	}
	for _, icon := range manifest.Icons {
		key := icon.Sizes + " " + icon.Purpose
		if want[key] == icon.Src && icon.Type == "image/png" {
			delete(want, key)
		}
	}
	if len(want) > 0 {
		t.Fatalf("manifest is missing icons %v", want)
	}
}

func TestAppIconsArePNGs(t *testing.T) {
	routes := newTestWebServer(t).routes()
	for _, path := range []string{"/icons/icon-192.png", "/icons/icon-512.png", "/icons/icon-maskable-512.png", "/apple-touch-icon.png"} {
		rec := get(t, routes, path)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", path, rec.Code)
			continue
		}
		if got := rec.Header().Get("Content-Type"); got != "image/png" {
			t.Errorf("%s: Content-Type = %q, want image/png", path, got)
		}
		if !strings.HasPrefix(rec.Body.String(), "\x89PNG\r\n\x1a\n") {
			t.Errorf("%s: body is not a PNG", path)
		}
	}
}

// Android draws a notification badge from its alpha channel alone, so the
// badge must be a single colour on a transparent background.
func TestNotificationBadgeIsAMonochromeMaskOfTheMark(t *testing.T) {
	routes := newTestWebServer(t).routes()
	rec := get(t, routes, "/icons/badge-96.png")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("Content-Type = %q, want image/png", got)
	}
	img, err := png.Decode(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatalf("badge is not a PNG: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 96 || b.Dy() != 96 {
		t.Fatalf("badge is %dx%d, want 96x96", b.Dx(), b.Dy())
	}
	if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
		t.Errorf("badge corner alpha = %d, want a transparent background", a)
	}
	if _, _, _, a := img.At(48, 48).RGBA(); a == 0 {
		t.Errorf("badge centre is transparent, want the mark drawn there")
	}
	for y := 0; y < 96; y++ {
		for x := 0; x < 96; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if c.A > 0 && (c.R != 255 || c.G != 255 || c.B != 255) {
				t.Fatalf("badge pixel (%d,%d) = %v, want white wherever it is drawn", x, y, c)
			}
		}
	}

	sw := get(t, routes, "/sw.js").Body.String()
	if !strings.Contains(sw, `badge: "/icons/badge-96.png"`) {
		t.Error("service worker does not use the monochrome badge")
	}
}

func TestServiceWorkerIsServedFromTheRoot(t *testing.T) {
	rec := get(t, newTestWebServer(t).routes(), "/sw.js")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/javascript; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want text/javascript; charset=utf-8", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("Cache-Control = %q, want no-cache so a new worker is picked up", got)
	}
	if !strings.Contains(rec.Body.String(), `"/offline.html"`) {
		t.Fatal("service worker does not fall back to the offline page")
	}
}

func TestOfflinePageIsServed(t *testing.T) {
	rec := get(t, newTestWebServer(t).routes(), "/offline.html")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want text/html; charset=utf-8", got)
	}
	if !strings.Contains(rec.Body.String(), "You're offline") {
		t.Fatal("offline page does not tell the Member they are offline")
	}
}

func TestPagesLinkTheManifestAndRegisterTheServiceWorker(t *testing.T) {
	var page strings.Builder
	if err := newTestWebServer(t).templates.ExecuteTemplate(&page, "layout", pageData{Title: "Homebase"}); err != nil {
		t.Fatalf("render layout: %v", err)
	}
	for _, want := range []string{
		`<link rel="manifest" href="/manifest.webmanifest">`,
		`<meta name="theme-color" content="#5865f2">`,
		`<link rel="apple-touch-icon" href="/apple-touch-icon.png">`,
		`<meta name="apple-mobile-web-app-capable" content="yes">`,
		`navigator.serviceWorker.register("/sw.js")`,
	} {
		if !strings.Contains(page.String(), want) {
			t.Errorf("page is missing %s", want)
		}
	}
}

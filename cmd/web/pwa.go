package main

import (
	"embed"
	"net/http"
)

// staticFiles holds the PWA shell: manifest, app icons, service worker and
// offline page. The icons are rasterised from the favicon mark in brand.go.
//
//go:embed static/*
var staticFiles embed.FS

// staticFile serves one embedded file with an explicit content type, since
// browsers are strict about the types of manifests and service workers.
func staticFile(name, contentType, cacheControl string) http.HandlerFunc {
	body, err := staticFiles.ReadFile("static/" + name)
	if err != nil {
		panic(err)
	}
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", cacheControl)
		_, _ = w.Write(body)
	}
}

func (s *webServer) pwaRoutes(mux *http.ServeMux) {
	const day = "public, max-age=86400"
	mux.HandleFunc("GET /manifest.webmanifest", staticFile("manifest.webmanifest", "application/manifest+json", day))
	mux.HandleFunc("GET /icons/icon-192.png", staticFile("icon-192.png", "image/png", day))
	mux.HandleFunc("GET /icons/icon-512.png", staticFile("icon-512.png", "image/png", day))
	mux.HandleFunc("GET /icons/icon-maskable-512.png", staticFile("icon-maskable-512.png", "image/png", day))
	mux.HandleFunc("GET /apple-touch-icon.png", staticFile("apple-touch-icon.png", "image/png", day))
	// The service worker must stay at the root so its scope covers every page,
	// and must not be cached so browsers pick up a new version promptly.
	mux.HandleFunc("GET /sw.js", staticFile("sw.js", "text/javascript; charset=utf-8", "no-cache"))
	mux.HandleFunc("GET /offline.html", staticFile("offline.html", "text/html; charset=utf-8", "no-cache"))
}

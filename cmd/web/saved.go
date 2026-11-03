package main

import (
	"net/http"
	"net/url"
	"strings"
)

// notSaves are the POST routes that redirect without saving anything, so the
// "Saved ✓" toast would mislead.
var notSaves = map[string]bool{
	"POST /logout":                  true,
	"POST /notifications/read-all":  true,
	"POST /notifications/{id}/open": true,
	"POST /notifications/{id}/read": true,
	"POST /dashboard/tiles/move":    true,
	"POST /dashboard/tiles/order":   true,
	"POST /push-subscriptions":      true,
}

// withSavedSignal adds saved=1 to the redirect after every successful save, so
// the page it lands on shows the "Saved ✓" toast. A save is any POST answered
// with 303 See Other whose Location carries no error, apart from notSaves.
func withSavedSignal(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if _, pattern := mux.Handler(r); !notSaves[pattern] {
				w = &savedSignalWriter{ResponseWriter: w}
			}
		}
		mux.ServeHTTP(w, r)
	})
}

type savedSignalWriter struct {
	http.ResponseWriter
	wroteHeader bool
}

func (w *savedSignalWriter) WriteHeader(status int) {
	if !w.wroteHeader && status == http.StatusSeeOther {
		location := w.Header().Get("Location")
		if location != "" && !hasQueryValue(location, "error") && !hasQueryValue(location, "saved") {
			w.Header().Set("Location", withQueryValue(location, "saved", "1"))
		}
	}
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *savedSignalWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

func (w *savedSignalWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func hasQueryValue(location, key string) bool {
	path, _, _ := strings.Cut(location, "#")
	_, rawQuery, _ := strings.Cut(path, "?")
	query, err := url.ParseQuery(rawQuery)
	return err == nil && query.Has(key)
}

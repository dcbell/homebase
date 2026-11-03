package main

import (
	"net/http"
	"net/url"
	"strings"
)

// savedQueryKey is the query key withSavedSignal adds (as saved=1) to tell the
// page it lands on to show the "Saved ✓" toast.
const savedQueryKey = "saved"

// nonSavingRoutes are the POST routes that redirect without saving anything, so the
// "Saved ✓" toast would mislead.
var nonSavingRoutes = map[string]bool{
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
// with 303 See Other whose Location carries no error, apart from nonSavingRoutes.
func withSavedSignal(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if _, pattern := mux.Handler(r); !nonSavingRoutes[pattern] {
				w = &savedSignalWriter{ResponseWriter: w}
			}
		}
		mux.ServeHTTP(w, r)
	})
}

type savedSignalWriter struct {
	http.ResponseWriter
	wroteHeader bool
	skip        bool
}

// redirectToSignIn sends a Member whose session the API rejected back to the
// sign-in page. Nothing was saved, so the redirect never gets saved=1.
func redirectToSignIn(w http.ResponseWriter, r *http.Request) {
	skipSavedSignal(w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// skipSavedSignal stops withSavedSignal adding saved=1 to this response.
func skipSavedSignal(w http.ResponseWriter) {
	for {
		switch rw := w.(type) {
		case *savedSignalWriter:
			rw.skip = true
			return
		case interface{ Unwrap() http.ResponseWriter }:
			w = rw.Unwrap()
		default:
			return
		}
	}
}

func (w *savedSignalWriter) WriteHeader(status int) {
	if !w.wroteHeader && !w.skip && status == http.StatusSeeOther {
		location := w.Header().Get("Location")
		if location != "" && !hasQueryValue(location, "error") && !hasQueryValue(location, savedQueryKey) {
			w.Header().Set("Location", withQueryValue(location, savedQueryKey, "1"))
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
	if err == nil {
		return query.Has(key)
	}
	// ParseQuery drops the pairs it cannot decode, so look at the raw keys too:
	// a malformed error is still an error.
	for _, pair := range strings.Split(rawQuery, "&") {
		if name, _, _ := strings.Cut(pair, "="); name == key {
			return true
		}
	}
	return query.Has(key)
}

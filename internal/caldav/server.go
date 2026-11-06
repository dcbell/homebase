package caldav

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/caldav"

	"homebase/internal/clock"
)

// ErrUnauthorized is what an Authenticator returns for a login it refuses.
var ErrUnauthorized = errors.New("caldav: unauthorized")

// Authenticator checks a Basic auth login: the username is the Member's email
// and the password a Calendar Connection secret. It records the connection's
// last use. Refused logins return ErrUnauthorized.
type Authenticator interface {
	AuthenticateCalendar(ctx context.Context, username, password string) (CalendarLogin, error)
}

// Server serves CalDAV under Root and the well-known redirect.
type Server struct {
	auth        Authenticator
	clock       clock.Clock
	logger      *slog.Logger
	collections []Collection
	dav         *caldav.Handler
}

// New builds a Server with the given collections, in the order calendar apps
// should list them.
func New(auth Authenticator, clk clock.Clock, logger *slog.Logger, collections ...Collection) *Server {
	s := &Server{auth: auth, clock: clk, logger: logger, collections: collections}
	s.dav = &caldav.Handler{Backend: &backend{s: s}, Prefix: strings.TrimSuffix(Root, "/")}
	return s
}

// WellKnown redirects discovery to Root. It needs no login. GET keeps the
// classic 301; other methods (clients PROPFIND it) get 308 so the method and
// body survive the redirect.
func (s *Server) WellKnown(w http.ResponseWriter, r *http.Request) {
	status := http.StatusPermanentRedirect
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		status = http.StatusMovedPermanently
	}
	w.Header().Set("Location", Root)
	w.WriteHeader(status)
}

// ServeHTTP serves everything under Root. Only Basic auth with a Calendar
// Connection is accepted: no session cookies, no API tokens.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == strings.TrimSuffix(Root, "/") {
		r.URL.Path = Root
	}

	username, password, ok := r.BasicAuth()
	if !ok {
		challenge(w)
		return
	}
	member, err := s.auth.AuthenticateCalendar(r.Context(), username, password)
	if errors.Is(err, ErrUnauthorized) {
		challenge(w)
		return
	}
	if err != nil {
		s.logger.Error("authenticate calendar connection", "error", err)
		http.Error(w, "failed to check login", http.StatusInternalServerError)
		return
	}

	p, ok := parsePath(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if p.kind != kindRoot && p.userID != member.UserID {
		http.Error(w, "this is another Member's calendar", http.StatusForbidden)
		return
	}

	ctx := context.WithValue(r.Context(), requestKey{}, &request{
		member:  member,
		now:     s.clock.Now(),
		objects: map[string][]object{},
		ifMatch: webdav.ConditionalMatch(r.Header.Get("If-Match")),
	})
	r = r.WithContext(ctx)
	if r.Method == "PROPFIND" {
		s.propfind(w, r, p)
		return
	}
	s.dav.ServeHTTP(w, r)
}

func challenge(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="Homebase", charset="UTF-8"`)
	http.Error(w, "login required", http.StatusUnauthorized)
}

// request is the per-request state: who is logged in, the Clock's now, and
// each collection's objects once loaded (a write forgets its collection's),
// and the If-Match header, which go-webdav doesn't pass on DELETE.
type request struct {
	member  CalendarLogin
	now     time.Time
	objects map[string][]object
	ifMatch webdav.ConditionalMatch
}

type requestKey struct{}

func requestFrom(ctx context.Context) *request {
	return ctx.Value(requestKey{}).(*request)
}

func (s *Server) collection(name string) Collection {
	for _, c := range s.collections {
		if c.Name() == name {
			return c
		}
	}
	return nil
}

// objects returns a collection's rendered objects for this request, loading
// them the first time.
func (s *Server) objects(ctx context.Context, c Collection) ([]object, error) {
	req := requestFrom(ctx)
	if loaded, ok := req.objects[c.Name()]; ok {
		return loaded, nil
	}
	items, err := c.Objects(ctx, req.member, req.now)
	if err != nil {
		return nil, err
	}
	base := collectionPath(req.member.UserID, c.Name())
	rendered := make([]object, 0, len(items))
	for _, item := range items {
		o, err := render(base, item)
		if err != nil {
			return nil, fmt.Errorf("render %s in %s: %w", item.UID, c.Name(), err)
		}
		rendered = append(rendered, o)
	}
	req.objects[c.Name()] = rendered
	return rendered, nil
}

// Paths.

type pathKind int

const (
	kindRoot pathKind = iota
	kindPrincipal
	kindHome
	kindCollection
	kindObject
)

type davPath struct {
	kind       pathKind
	userID     int64
	collection string
	uid        string
}

func principalPath(userID int64) string { return fmt.Sprintf("%s%d/", Root, userID) }
func homePath(userID int64) string      { return principalPath(userID) + "calendars/" }
func collectionPath(userID int64, name string) string {
	return homePath(userID) + name + "/"
}

// parsePath splits a path under Root. Trailing slashes are optional.
func parsePath(raw string) (davPath, bool) {
	rest, ok := strings.CutPrefix(raw, Root)
	if !ok {
		return davPath{}, false
	}
	rest = strings.TrimSuffix(rest, "/")
	if rest == "" {
		return davPath{kind: kindRoot}, true
	}
	parts := strings.Split(rest, "/")
	userID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || userID <= 0 {
		return davPath{}, false
	}
	p := davPath{kind: kindPrincipal, userID: userID}
	switch {
	case len(parts) == 1:
		return p, true
	case parts[1] != "calendars":
		return davPath{}, false
	case len(parts) == 2:
		p.kind = kindHome
	case len(parts) == 3:
		p.kind, p.collection = kindCollection, parts[2]
	case len(parts) == 4 && strings.HasSuffix(parts[3], ".ics") && len(parts[3]) > len(".ics"):
		p.kind, p.collection, p.uid = kindObject, parts[2], strings.TrimSuffix(parts[3], ".ics")
	default:
		return davPath{}, false
	}
	return p, true
}

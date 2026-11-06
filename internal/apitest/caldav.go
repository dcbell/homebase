package apitest

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/caldav"

	"homebase/internal/store"
)

// CalDAVBaseURL is where CalDAV clients built by the Harness think the server
// is. Requests never leave the process.
const CalDAVBaseURL = "http://homebase.test"

// CalDAV is a Calendar Connection for one Member, with a real CalDAV client
// authenticated as it and a raw helper for what that client can't do
// (getctag, privileges, conditional requests, status codes).
//
//	cal := household.Member.ConnectCalendar()
//	objects, err := cal.Client.QueryCalendar(ctx, cal.CollectionPath("homebase"), query)
//	ms := cal.Propfind(cal.CollectionPath("homebase"), "0", apitest.GetCTag)
type CalDAV struct {
	h          *Harness
	Connection store.CalendarConnection
	Username   string
	Secret     string
	ServerURL  string
	// Client is a go-webdav CalDAV client pointed at /caldav/.
	Client *caldav.Client
}

// ConnectCalendar creates an "other" Calendar Connection for the Client's
// Member through the JSON API and returns a CalDAV client logged in with it.
func (c *Client) ConnectCalendar() *CalDAV {
	c.h.t.Helper()
	created := Decode[struct {
		Connection store.CalendarConnection `json:"connection"`
		ServerURL  string                   `json:"server_url"`
		Username   string                   `json:"username"`
		Secret     string                   `json:"secret"`
	}](c.h.t, c.Post("/api/v1/calendar-connections", map[string]any{
		"name": fmt.Sprintf("Test device %d", c.User.ID),
		"kind": "other",
	}).Expect(http.StatusCreated))
	return &CalDAV{
		h:          c.h,
		Connection: created.Connection,
		Username:   created.Username,
		Secret:     created.Secret,
		ServerURL:  created.ServerURL,
		Client:     c.h.CalDAVClient("/caldav/", created.Username, created.Secret),
	}
}

// PrincipalPath is the connected Member's CalDAV principal.
func (d *CalDAV) PrincipalPath() string {
	return fmt.Sprintf("/caldav/%d/", d.Connection.UserID)
}

// HomePath is the connected Member's calendar home.
func (d *CalDAV) HomePath() string {
	return d.PrincipalPath() + "calendars/"
}

// CollectionPath is one of the connected Member's collections, such as
// "homebase".
func (d *CalDAV) CollectionPath(name string) string {
	return d.HomePath() + name + "/"
}

// CalDAVClient returns a go-webdav CalDAV client for the given endpoint path
// that logs in with Basic auth and talks to the Harness's server in-process.
func (h *Harness) CalDAVClient(endpoint, username, password string) *caldav.Client {
	h.t.Helper()
	httpClient := webdav.HTTPClientWithBasicAuth(&http.Client{Transport: inProcess{h}}, username, password)
	client, err := caldav.NewClient(httpClient, CalDAVBaseURL+endpoint)
	if err != nil {
		h.t.Fatalf("new CalDAV client: %v", err)
	}
	return client
}

// inProcess is an http.RoundTripper that serves requests with the Harness's
// handler.
type inProcess struct{ h *Harness }

func (t inProcess) RoundTrip(req *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	t.h.Server.ServeHTTP(rec, req)
	t.h.Server.WaitForPush()
	resp := rec.Result()
	// go-webdav reads resp.Request (for example in GetCalendarObject), which
	// ResponseRecorder leaves unset.
	resp.Request = req
	return resp, nil
}

// Do sends a raw request authenticated as the connection. header may be nil;
// an Authorization in it replaces the connection's Basic auth.
func (d *CalDAV) Do(method, path string, header http.Header, body string) *Response {
	d.h.t.Helper()
	all := http.Header{}
	all.Set("Authorization", basicAuth(d.Username, d.Secret))
	for key, values := range header {
		all[key] = values
	}
	return d.h.CalDAVRequest(method, path, all, body)
}

// CalDAVRequest sends a raw request with exactly the given headers (no
// credentials unless header carries them).
func (h *Harness) CalDAVRequest(method, path string, header http.Header, body string) *Response {
	h.t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	for key, values := range header {
		req.Header[key] = values
	}
	if body != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/xml; charset=utf-8")
	}
	rec := httptest.NewRecorder()
	h.Server.ServeHTTP(rec, req)
	h.Server.WaitForPush()
	return &Response{t: h.t, Method: method, Path: path, Status: rec.Code, Header: rec.Header(), Body: rec.Body.Bytes()}
}

// BasicAuth returns an Authorization header value for Basic auth.
func BasicAuth(username, password string) string {
	return basicAuth(username, password)
}

func basicAuth(username, password string) string {
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth(username, password)
	return req.Header.Get("Authorization")
}

// Property names for Propfind, in Clark notation ({namespace}local).
const (
	DisplayName             = "{DAV:}displayname"
	ResourceType            = "{DAV:}resourcetype"
	CurrentUserPrincipal    = "{DAV:}current-user-principal"
	CurrentUserPrivilegeSet = "{DAV:}current-user-privilege-set"
	GetETag                 = "{DAV:}getetag"
	SupportedReportSet      = "{DAV:}supported-report-set"
	CalendarHomeSet         = "{urn:ietf:params:xml:ns:caldav}calendar-home-set"
	SupportedComponentSet   = "{urn:ietf:params:xml:ns:caldav}supported-calendar-component-set"
	GetCTag                 = "{http://calendarserver.org/ns/}getctag"
	CalendarColor           = "{http://apple.com/ns/ical/}calendar-color"
)

// MultiStatus is a parsed 207 Multi-Status body: each response's properties by
// href.
type MultiStatus map[string]map[string]Prop

// Prop is one property in a Multi-Status response.
type Prop struct {
	Status int
	// Text is the property's character data, trimmed.
	Text string
	// Children are the names (Clark notation) of every element inside the
	// property, at any depth, such as "{DAV:}read" inside a privilege set.
	Children []string
}

// Has reports whether the property contains an element with the given name.
func (p Prop) Has(name string) bool {
	for _, child := range p.Children {
		if child == name {
			return true
		}
	}
	return false
}

// Propfind sends a PROPFIND for the named properties (Clark notation) with the
// given Depth and returns the parsed Multi-Status. It fails the test unless
// the server answers 207.
func (d *CalDAV) Propfind(path, depth string, props ...string) MultiStatus {
	d.h.t.Helper()
	var body strings.Builder
	body.WriteString(`<?xml version="1.0" encoding="utf-8"?><D:propfind xmlns:D="DAV:"><D:prop>`)
	for _, prop := range props {
		space, local := splitClark(prop)
		fmt.Fprintf(&body, `<x:%s xmlns:x="%s"/>`, local, space)
	}
	body.WriteString(`</D:prop></D:propfind>`)
	resp := d.Do("PROPFIND", path, http.Header{"Depth": {depth}}, body.String()).Expect(http.StatusMultiStatus)
	return ParseMultiStatus(d.h.t, resp.Body)
}

func splitClark(name string) (space, local string) {
	if strings.HasPrefix(name, "{") {
		if end := strings.Index(name, "}"); end > 0 {
			return name[1:end], name[end+1:]
		}
	}
	return "DAV:", name
}

// ParseMultiStatus parses a 207 Multi-Status body.
func ParseMultiStatus(t testing.TB, body []byte) MultiStatus {
	t.Helper()
	var doc struct {
		Responses []struct {
			Href      string `xml:"DAV: href"`
			Propstats []struct {
				Prop struct {
					Inner []rawElement `xml:",any"`
				} `xml:"DAV: prop"`
				Status string `xml:"DAV: status"`
			} `xml:"DAV: propstat"`
		} `xml:"DAV: response"`
	}
	if err := xml.Unmarshal(body, &doc); err != nil {
		t.Fatalf("parse multistatus: %v; body: %s", err, body)
	}
	ms := MultiStatus{}
	for _, r := range doc.Responses {
		props := map[string]Prop{}
		for _, ps := range r.Propstats {
			status := parseStatusLine(ps.Status)
			for _, el := range ps.Prop.Inner {
				props["{"+el.XMLName.Space+"}"+el.XMLName.Local] = Prop{Status: status, Text: strings.TrimSpace(el.text()), Children: el.descendants()}
			}
		}
		ms[r.Href] = props
	}
	return ms
}

func parseStatusLine(line string) int {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0
	}
	code, _ := strconv.Atoi(fields[1])
	return code
}

type rawElement struct {
	XMLName  xml.Name
	CharData string       `xml:",chardata"`
	Children []rawElement `xml:",any"`
}

func (e rawElement) text() string {
	text := e.CharData
	for _, child := range e.Children {
		text += child.text()
	}
	return text
}

func (e rawElement) descendants() []string {
	var names []string
	for _, child := range e.Children {
		names = append(names, "{"+child.XMLName.Space+"}"+child.XMLName.Local)
		names = append(names, child.descendants()...)
	}
	return names
}

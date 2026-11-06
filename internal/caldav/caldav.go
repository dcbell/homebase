// Package caldav serves a Member's Homebase items to phone and desktop
// calendar apps over CalDAV (ADR-0004).
//
// Layout, one principal and calendar home per Member:
//
//	/caldav/                                   root (current-user-principal)
//	/caldav/<userID>/                          principal
//	/caldav/<userID>/calendars/                calendar home
//	/caldav/<userID>/calendars/<collection>/   one Collection, such as homebase
//	/caldav/<userID>/calendars/<collection>/<uid>.ics
//
// Server owns login, discovery, the well-known redirect and PROPFIND (which
// carries getctag, colour and privileges that go-webdav lacks). go-webdav's
// caldav.Handler serves everything else (GET, REPORT, OPTIONS, PUT, DELETE)
// through backend. Each calendar is a Collection: adding one is a matter of
// implementing Collection and passing it to New.
package caldav

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"time"

	"github.com/emersion/go-ical"
)

// Root is the path every CalDAV resource lives under.
const Root = "/caldav/"

// WellKnownPath is the standard discovery address (RFC 6764). It redirects to
// Root.
const WellKnownPath = "/.well-known/caldav"

// CalendarLogin is who a CalDAV request is logged in as: the Member and
// Household of one Calendar Connection.
type CalendarLogin struct {
	UserID       int64
	HouseholdID  int64
	Email        string
	Name         string
	ConnectionID int64
}

// Collection is one calendar in every Member's calendar home.
type Collection interface {
	// Name is the collection's path segment, such as "homebase".
	Name() string
	// DisplayName is what calendar apps show, such as "Homebase".
	DisplayName() string
	// Component is the one iCalendar component the collection holds: VEVENT
	// or VTODO. Mixing them makes lists show up in both Calendar and Reminders.
	Component() string
	// Color is the suggested calendar colour as #RRGGBBAA.
	Color() string
	// Objects returns everything in the Member's view of the collection as of
	// now. It is called at most once per request.
	Objects(ctx context.Context, m CalendarLogin, now time.Time) ([]Object, error)
}

// Object is one calendar object resource in a Collection.
type Object struct {
	// UID is the object's iCalendar UID; its resource name is UID + ".ics".
	UID      string
	Calendar *ical.Calendar
	ModTime  time.Time
}

// Window returns the span of time the calendars cover around now: 90 days
// back to 12 months ahead.
func Window(now time.Time) (from, to time.Time) {
	return now.AddDate(0, 0, -90), now.AddDate(0, 12, 0)
}

// object is an Object rendered to iCalendar text, with its ETag.
type object struct {
	Object
	href string
	data []byte
	etag string
}

func render(collectionPath string, o Object) (object, error) {
	var buf bytes.Buffer
	// The encoder sorts properties, so the same item always renders to the
	// same bytes and so the same ETag.
	if err := ical.NewEncoder(&buf).Encode(o.Calendar); err != nil {
		return object{}, err
	}
	sum := sha256.Sum256(buf.Bytes())
	return object{
		Object: o,
		href:   collectionPath + o.UID + ".ics",
		data:   buf.Bytes(),
		etag:   hex.EncodeToString(sum[:16]),
	}, nil
}

// ctag is a collection's change tag: a hash over every member's href and
// ETag, so it changes whenever an item changes, arrives or leaves.
func ctag(objects []object) string {
	lines := make([]string, len(objects))
	for i, o := range objects {
		lines[i] = o.href + " " + o.etag
	}
	sort.Strings(lines)
	h := sha256.New()
	for _, line := range lines {
		h.Write([]byte(line + "\n"))
	}
	return hex.EncodeToString(h.Sum(nil)[:16])
}

// newCalendar returns an empty VCALENDAR with the properties every object
// needs.
func newCalendar() *ical.Calendar {
	cal := ical.NewCalendar()
	cal.Props.SetText(ical.PropVersion, "2.0")
	cal.Props.SetText(ical.PropProductID, "-//Homebase//CalDAV//EN")
	return cal
}

package caldav

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	// Devices name timezones by Olson ID (TZID=America/Chicago); the API
	// image may have no zoneinfo of its own.
	_ "time/tzdata"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/caldav"

	"homebase/internal/store"
)

// Writable is a Collection devices can change. Before calling Put or Delete
// the backend has checked the request: the calendar data holds exactly one
// component of the collection's type (VTIMEZONEs aside) whose UID matches its
// address, it doesn't repeat, alarms are stripped, and If-Match and
// If-None-Match hold against what Find returns.
//
// Put and Delete go through the shared save operations so the app's rules
// apply. They may return errors from refuse or badRequest, which reach the
// device as they are; store.ErrNotFound becomes 404.
type Writable interface {
	Collection
	// Find returns the object with this UID in the Member's view, whether
	// or not it falls inside the Window. ok is false when there is none.
	Find(ctx context.Context, m Member, uid string) (o Object, ok bool, err error)
	// Put creates the object with this UID (exists false) or replaces it
	// with comp.
	Put(ctx context.Context, m Member, uid string, comp *ical.Component, exists bool) error
	// Delete removes the object with this UID, which Find found.
	Delete(ctx context.Context, m Member, uid string) error
}

// refuse is a 403 carrying a CalDAV precondition (RFC 4791 section 5.3.2.1)
// as its XML error body. go-webdav's own NewPreconditionError says 409; the
// outer status wins.
func refuse(precondition caldav.PreconditionType) error {
	return webdav.NewHTTPError(http.StatusForbidden, caldav.NewPreconditionError(precondition))
}

// badRequest is a 400 for calendar data Homebase can't make an item of.
func badRequest(format string, args ...any) error {
	return webdav.NewHTTPError(http.StatusBadRequest, fmt.Errorf(format, args...))
}

func preconditionFailed() error {
	return webdav.NewHTTPError(http.StatusPreconditionFailed, errors.New("the item has changed; fetch it again"))
}

// writable resolves an object path to a Writable collection and the UID.
// Collections that aren't Writable refuse with 403.
func (b *backend) writable(ctx context.Context, path string) (Writable, string, error) {
	p, ok := parsePath(path)
	if !ok || p.kind != kindObject || p.userID != requestFrom(ctx).member.UserID {
		return nil, "", notFound()
	}
	c := b.s.collection(p.collection)
	if c == nil {
		return nil, "", notFound()
	}
	w, ok := c.(Writable)
	if !ok {
		return nil, "", readOnly()
	}
	return w, p.uid, nil
}

// current returns the object at uid with its ETag, if there is one.
func (b *backend) current(ctx context.Context, w Writable, uid string) (object, bool, error) {
	req := requestFrom(ctx)
	o, ok, err := w.Find(ctx, req.member, uid)
	if err != nil || !ok {
		return object{}, false, err
	}
	rendered, err := render(collectionPath(req.member.UserID, w.Name()), o)
	return rendered, err == nil, err
}

func (b *backend) PutCalendarObject(ctx context.Context, path string, cal *ical.Calendar, opts *caldav.PutCalendarObjectOptions) (*caldav.CalendarObject, error) {
	w, uid, err := b.writable(ctx, path)
	if err != nil {
		return nil, err
	}
	comp, err := incoming(cal, w.Component(), uid)
	if err != nil {
		return nil, err
	}
	existing, exists, err := b.current(ctx, w, uid)
	if err != nil {
		return nil, err
	}
	if err := checkConditions(existing, exists, opts.IfMatch, opts.IfNoneMatch); err != nil {
		return nil, err
	}
	req := requestFrom(ctx)
	if err := w.Put(ctx, req.member, uid, comp, exists); err != nil {
		return nil, writeError(err)
	}
	delete(req.objects, w.Name())
	// No ETag: Homebase stores its own version (alarms dropped, fields
	// normalised), so the device must fetch it again (RFC 4791 5.3.4).
	return &caldav.CalendarObject{Path: path}, nil
}

func (b *backend) DeleteCalendarObject(ctx context.Context, path string) error {
	w, uid, err := b.writable(ctx, path)
	if err != nil {
		return err
	}
	existing, exists, err := b.current(ctx, w, uid)
	if err != nil {
		return err
	}
	if !exists {
		return notFound()
	}
	req := requestFrom(ctx)
	// go-webdav doesn't pass If-Match on DELETE; ServeHTTP keeps it.
	if err := checkConditions(existing, exists, req.ifMatch, ""); err != nil {
		return err
	}
	if err := w.Delete(ctx, req.member, uid); err != nil {
		return writeError(err)
	}
	delete(req.objects, w.Name())
	return nil
}

func writeError(err error) error {
	if errors.Is(err, store.ErrNotFound) {
		return notFound()
	}
	return err
}

// checkConditions applies If-Match and If-None-Match. A write naming no
// version simply overwrites.
func checkConditions(o object, exists bool, ifMatch, ifNoneMatch webdav.ConditionalMatch) error {
	if ifNoneMatch.IsSet() && exists {
		if match, _ := ifNoneMatch.MatchETag(o.etag); match {
			return preconditionFailed()
		}
	}
	if ifMatch.IsSet() {
		if !exists {
			return preconditionFailed()
		}
		if match, err := ifMatch.MatchETag(o.etag); err != nil || !match {
			return preconditionFailed()
		}
	}
	return nil
}

// incoming checks calendar data a device sent and returns its one component
// with any alarms removed. Homebase items don't repeat, so repeating events
// and changes to single occurrences are refused.
func incoming(cal *ical.Calendar, component, uid string) (*ical.Component, error) {
	var found []*ical.Component
	for _, child := range cal.Children {
		switch child.Name {
		case ical.CompTimezone:
		case component:
			found = append(found, child)
		default:
			// Each calendar holds one kind of item.
			return nil, refuse(caldav.PreconditionSupportedCalendarComponent)
		}
	}
	if _, _, err := caldav.ValidateCalendarObject(cal); err != nil {
		return nil, refuse(caldav.PreconditionValidCalendarObjectResource)
	}
	if len(found) == 0 {
		return nil, refuse(caldav.PreconditionSupportedCalendarComponent)
	}
	if len(found) > 1 {
		// A series with changed occurrences.
		return nil, refuse(caldav.PreconditionValidCalendarObjectResource)
	}
	comp := found[0]
	for _, name := range []string{ical.PropRecurrenceRule, ical.PropRecurrenceDates, ical.PropExceptionDates, ical.PropRecurrenceID} {
		if comp.Props.Get(name) != nil {
			return nil, refuse(caldav.PreconditionValidCalendarObjectResource)
		}
	}
	if got, _ := comp.Props.Text(ical.PropUID); got != uid {
		// Objects live at UID.ics, so the UID must match the address.
		return nil, refuse(caldav.PreconditionValidCalendarObjectResource)
	}
	children := comp.Children[:0]
	for _, child := range comp.Children {
		if child.Name != ical.CompAlarm {
			children = append(children, child)
		}
	}
	comp.Children = children
	return comp, nil
}

// dateTime reads a DATE or DATE-TIME property. DATEs and floating times are
// in APP_TIMEZONE (time.Local). A TZID Go doesn't know, which only non-Olson
// clients send, is read as APP_TIMEZONE too rather than refused.
func dateTime(p *ical.Prop) (time.Time, error) {
	t, err := p.DateTime(time.Local)
	if err == nil || p.Params.Get(ical.PropTimezoneID) == "" {
		return t, err
	}
	floating := *p
	floating.Params = ical.Params{}
	for name, values := range p.Params {
		if !strings.EqualFold(name, ical.PropTimezoneID) {
			floating.Params[name] = values
		}
	}
	return floating.DateTime(time.Local)
}

// isDate reports whether a property holds a DATE (an all-day value).
func isDate(p *ical.Prop) bool {
	return p.ValueType() == ical.ValueDate || (p.ValueType() == ical.ValueDefault && len(p.Value) == len("20060102"))
}

// text reads a text property, keeping commas (go-ical's Text splits lists).
func text(comp *ical.Component, name string) (string, error) {
	p := comp.Props.Get(name)
	if p == nil {
		return "", nil
	}
	list, err := p.TextList()
	return strings.Join(list, ","), err
}

package caldav

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/caldav"
)

// backend adapts Server's collections to go-webdav's caldav.Backend, which
// serves GET, HEAD, OPTIONS, REPORT, PUT and DELETE. Paths in and out are
// full paths, Root included.
type backend struct {
	s *Server
}

var _ caldav.Backend = (*backend)(nil)

func notFound() error {
	return webdav.NewHTTPError(http.StatusNotFound, errors.New("not found"))
}

func (b *backend) CurrentUserPrincipal(ctx context.Context) (string, error) {
	return principalPath(requestFrom(ctx).member.UserID), nil
}

func (b *backend) CalendarHomeSetPath(ctx context.Context) (string, error) {
	return homePath(requestFrom(ctx).member.UserID), nil
}

func (b *backend) CreateCalendar(context.Context, *caldav.Calendar) error {
	return webdav.NewHTTPError(http.StatusForbidden, errors.New("calendars can't be created"))
}

func (b *backend) ListCalendars(ctx context.Context) ([]caldav.Calendar, error) {
	userID := requestFrom(ctx).member.UserID
	calendars := make([]caldav.Calendar, 0, len(b.s.collections))
	for _, c := range b.s.collections {
		calendars = append(calendars, calendarOf(userID, c))
	}
	return calendars, nil
}

func (b *backend) GetCalendar(ctx context.Context, path string) (*caldav.Calendar, error) {
	c, err := b.collectionAt(path)
	if err != nil {
		return nil, err
	}
	cal := calendarOf(requestFrom(ctx).member.UserID, c)
	return &cal, nil
}

func calendarOf(userID int64, c Collection) caldav.Calendar {
	return caldav.Calendar{
		Path:                  collectionPath(userID, c.Name()),
		Name:                  c.DisplayName(),
		SupportedComponentSet: []string{c.Component()},
	}
}

func (b *backend) GetCalendarObject(ctx context.Context, path string, _ *caldav.CalendarCompRequest) (*caldav.CalendarObject, error) {
	p, ok := parsePath(path)
	if !ok || p.kind != kindObject || p.userID != requestFrom(ctx).member.UserID {
		return nil, notFound()
	}
	c := b.s.collection(p.collection)
	if c == nil {
		return nil, notFound()
	}
	objects, err := b.s.objects(ctx, c)
	if err != nil {
		return nil, err
	}
	for _, o := range objects {
		if o.UID == p.uid {
			co := calendarObject(o)
			return &co, nil
		}
	}
	return nil, notFound()
}

func (b *backend) ListCalendarObjects(ctx context.Context, path string, _ *caldav.CalendarCompRequest) ([]caldav.CalendarObject, error) {
	c, err := b.collectionAt(path)
	if err != nil {
		return nil, err
	}
	objects, err := b.s.objects(ctx, c)
	if err != nil {
		return nil, err
	}
	list := make([]caldav.CalendarObject, len(objects))
	for i, o := range objects {
		list[i] = calendarObject(o)
	}
	return list, nil
}

// QueryCalendarObjects answers calendar-query. go-webdav's own matcher
// ignores VTODO time ranges, so the filter is applied here: the top-level
// VCALENDAR filter, one component filter naming the collection's component,
// and an optional time range on it.
func (b *backend) QueryCalendarObjects(ctx context.Context, path string, query *caldav.CalendarQuery) ([]caldav.CalendarObject, error) {
	c, err := b.collectionAt(path)
	if err != nil {
		return nil, err
	}
	objects, err := b.s.objects(ctx, c)
	if err != nil {
		return nil, err
	}
	var list []caldav.CalendarObject
	for _, o := range objects {
		if matchesQuery(o, query.CompFilter) {
			list = append(list, calendarObject(o))
		}
	}
	return list, nil
}

func matchesQuery(o object, filter caldav.CompFilter) bool {
	if filter.Name != "" && filter.Name != ical.CompCalendar {
		return false
	}
	if len(filter.Comps) == 0 {
		return true
	}
	for _, compFilter := range filter.Comps {
		if !matchesComponent(o.Calendar, compFilter) {
			return false
		}
	}
	return true
}

func matchesComponent(cal *ical.Calendar, filter caldav.CompFilter) bool {
	for _, child := range cal.Children {
		if child.Name != filter.Name {
			continue
		}
		if filter.IsNotDefined {
			return false
		}
		if filter.Start.IsZero() && filter.End.IsZero() {
			return true
		}
		start, end, ok := span(child)
		if !ok {
			return true
		}
		return overlaps(start, end, filter.Start, filter.End)
	}
	return filter.IsNotDefined
}

// span is the time a component occupies, per RFC 4791 section 9.9.
func span(comp *ical.Component) (start, end time.Time, ok bool) {
	switch comp.Name {
	case ical.CompEvent:
		event := ical.Event{Component: comp}
		start, err := event.DateTimeStart(time.Local)
		if err != nil {
			return time.Time{}, time.Time{}, false
		}
		end, err := event.DateTimeEnd(time.Local)
		if err != nil || end.Before(start) {
			end = start
		}
		return start, end, true
	case ical.CompToDo:
		// A to-do with a DUE is the instant it falls due; one with neither
		// DUE nor DTSTART matches every range (ok false).
		for _, name := range []string{ical.PropDue, ical.PropDateTimeStart} {
			if p := comp.Props.Get(name); p != nil {
				at, err := p.DateTime(time.Local)
				if err != nil {
					return time.Time{}, time.Time{}, false
				}
				return at, at, true
			}
		}
	}
	return time.Time{}, time.Time{}, false
}

// overlaps reports whether [start, end) meets the query range; an instant
// (start == end) counts when it falls inside it.
func overlaps(start, end, from, to time.Time) bool {
	if !to.IsZero() && !start.Before(to) {
		return false
	}
	if from.IsZero() {
		return true
	}
	if start.Equal(end) {
		return !start.Before(from)
	}
	return end.After(from)
}

func (b *backend) PutCalendarObject(context.Context, string, *ical.Calendar, *caldav.PutCalendarObjectOptions) (*caldav.CalendarObject, error) {
	return nil, readOnly()
}

func (b *backend) DeleteCalendarObject(context.Context, string) error {
	return readOnly()
}

// readOnly refuses writes: no collection accepts them yet (#35, #36).
func readOnly() error {
	return webdav.NewHTTPError(http.StatusForbidden, errors.New("this calendar is read-only"))
}

func (b *backend) collectionAt(path string) (Collection, error) {
	p, ok := parsePath(path)
	if !ok || p.kind != kindCollection {
		return nil, notFound()
	}
	c := b.s.collection(p.collection)
	if c == nil {
		return nil, notFound()
	}
	return c, nil
}

func calendarObject(o object) caldav.CalendarObject {
	return caldav.CalendarObject{
		Path:          o.href,
		ModTime:       o.ModTime,
		ContentLength: int64(len(o.data)),
		ETag:          o.etag,
		Data:          o.Calendar,
	}
}

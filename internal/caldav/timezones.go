package caldav

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav/caldav"
	"github.com/teambition/rrule-go"
)

// resolveTimezones rewrites comp's TZID-qualified times that Go can't place
// as UTC, using the VTIMEZONE in cal that the TZID names. A TZID Go knows (an
// Olson name) is left for dateTime to look up. A time whose zone neither Go
// nor the object defines is refused rather than guessed.
func resolveTimezones(cal *ical.Calendar, comp *ical.Component) error {
	for name, props := range comp.Props {
		for i := range props {
			p := &props[i]
			tzid := p.Params.Get(ical.PropTimezoneID)
			if tzid == "" || isDate(p) {
				continue
			}
			if _, err := time.LoadLocation(tzid); err == nil {
				continue
			}
			tz := vtimezone(cal, tzid)
			if tz == nil {
				return refuse(caldav.PreconditionValidCalendarObjectResource)
			}
			wall, err := floating(p).DateTime(time.UTC)
			if err != nil {
				return badRequest("%s: %v", name, err)
			}
			offset, err := offsetAt(tz, wall)
			if err != nil {
				return refuse(caldav.PreconditionValidCalendarObjectResource)
			}
			p.Params.Del(ical.PropTimezoneID)
			p.Value = wall.Add(-offset).Format("20060102T150405Z")
		}
	}
	return nil
}

// vtimezone returns cal's VTIMEZONE with this TZID, if any.
func vtimezone(cal *ical.Calendar, tzid string) *ical.Component {
	for _, child := range cal.Children {
		if child.Name != ical.CompTimezone {
			continue
		}
		if got, _ := child.Props.Text(ical.PropTimezoneID); got == tzid {
			return child
		}
	}
	return nil
}

// floating is p without its TZID, so its value reads as a wall-clock time.
func floating(p *ical.Prop) *ical.Prop {
	f := *p
	f.Params = ical.Params{}
	for name, values := range p.Params {
		if !strings.EqualFold(name, ical.PropTimezoneID) {
			f.Params[name] = values
		}
	}
	return &f
}

// offsetAt is the UTC offset a VTIMEZONE gives the wall-clock time wall
// (expressed as if in UTC): that of the observance (STANDARD or DAYLIGHT)
// with the latest onset at or before it. Before every onset, the earliest
// observance's TZOFFSETFROM applies.
func offsetAt(tz *ical.Component, wall time.Time) (time.Duration, error) {
	var (
		latest, earliest     time.Time
		latestTo, earliestFr time.Duration
		found, seen          bool
	)
	for _, o := range tz.Children {
		if o.Name != ical.CompTimezoneStandard && o.Name != ical.CompTimezoneDaylight {
			continue
		}
		to, err := utcOffset(o, ical.PropTimezoneOffsetTo)
		if err != nil {
			return 0, err
		}
		from, err := utcOffset(o, ical.PropTimezoneOffsetFrom)
		if err != nil {
			from = to
		}
		start := o.Props.Get(ical.PropDateTimeStart)
		if start == nil {
			return 0, errors.New("observance without DTSTART")
		}
		first, err := start.DateTime(time.UTC)
		if err != nil {
			return 0, err
		}
		if !seen || first.Before(earliest) {
			earliest, earliestFr, seen = first, from, true
		}
		onset := first
		if set, err := nearby(o, first, wall).RecurrenceSet(time.UTC); err != nil {
			return 0, err
		} else if set != nil {
			onset = set.Before(wall, true)
		}
		if onset.IsZero() || onset.After(wall) {
			continue
		}
		if !found || onset.After(latest) {
			latest, latestTo, found = onset, to, true
		}
	}
	switch {
	case found:
		return latestTo, nil
	case seen:
		return earliestFr, nil
	}
	return 0, errors.New("VTIMEZONE without observances")
}

// nearby returns observance o with a yearly rule's DTSTART moved to the year
// before wall. Windows clients start their rules in 1601, and the rrule
// library stops counting about 292 years after DTSTART.
func nearby(o *ical.Component, first, wall time.Time) *ical.Component {
	rule, err := o.Props.RecurrenceRule()
	if err != nil || rule == nil || rule.Freq != rrule.YEARLY || first.Year() >= wall.Year()-1 {
		return o
	}
	moved := *o
	moved.Props = ical.Props{}
	for name, props := range o.Props {
		moved.Props[name] = props
	}
	start := time.Date(wall.Year()-1, first.Month(), first.Day(), first.Hour(), first.Minute(), first.Second(), 0, time.UTC)
	moved.Props.Set(&ical.Prop{Name: ical.PropDateTimeStart, Params: ical.Params{}, Value: start.Format("20060102T150405")})
	return &moved
}

// utcOffset reads a UTC-OFFSET property such as -0500 or +053000.
func utcOffset(o *ical.Component, name string) (time.Duration, error) {
	p := o.Props.Get(name)
	if p == nil {
		return 0, fmt.Errorf("missing %s", name)
	}
	v := strings.TrimSpace(p.Value)
	if len(v) != 5 && len(v) != 7 || (v[0] != '+' && v[0] != '-') {
		return 0, fmt.Errorf("%s: bad offset %q", name, v)
	}
	var parts [3]int
	for i := 0; 1+2*i < len(v); i++ {
		n, err := strconv.Atoi(v[1+2*i : 3+2*i])
		if err != nil {
			return 0, fmt.Errorf("%s: bad offset %q", name, v)
		}
		parts[i] = n
	}
	d := time.Duration(parts[0])*time.Hour + time.Duration(parts[1])*time.Minute + time.Duration(parts[2])*time.Second
	if v[0] == '-' {
		d = -d
	}
	return d, nil
}

package api_test

import (
	"context"
	"net/http"
	"testing"

	"homebase/internal/apitest"
)

func TestCalDAVDiscoveryFromTheWellKnownAddressReachesTheHomebaseCalendar(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	cal := household.Member.ConnectCalendar()
	ctx := context.Background()

	client := h.CalDAVClient("/.well-known/caldav", cal.Username, cal.Secret)
	principal, err := client.FindCurrentUserPrincipal(ctx)
	if err != nil {
		t.Fatalf("find principal: %v", err)
	}
	if principal != cal.PrincipalPath() {
		t.Errorf("principal = %q, want %q", principal, cal.PrincipalPath())
	}
	home, err := client.FindCalendarHomeSet(ctx, principal)
	if err != nil {
		t.Fatalf("find calendar home: %v", err)
	}
	if home != cal.HomePath() {
		t.Errorf("calendar home = %q, want %q", home, cal.HomePath())
	}
	calendars, err := client.FindCalendars(ctx, home)
	if err != nil {
		t.Fatalf("find calendars: %v", err)
	}
	if len(calendars) != 1 {
		t.Fatalf("calendars = %+v, want just homebase", calendars)
	}
	got := calendars[0]
	if got.Path != cal.CollectionPath("homebase") || got.Name != "Homebase" {
		t.Errorf("calendar = %+v, want %s named Homebase", got, cal.CollectionPath("homebase"))
	}
	if len(got.SupportedComponentSet) != 1 || got.SupportedComponentSet[0] != "VEVENT" {
		t.Errorf("components = %v, want only VEVENT", got.SupportedComponentSet)
	}
}

func TestTheWellKnownAddressRedirectsWithoutLogin(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)

	for _, method := range []string{http.MethodGet, "PROPFIND"} {
		resp := h.CalDAVRequest(method, "/.well-known/caldav", nil, "")
		if resp.Status/100 != 3 || resp.Header.Get("Location") != "/caldav/" {
			t.Errorf("%s well-known = %d to %q, want a redirect to /caldav/", method, resp.Status, resp.Header.Get("Location"))
		}
	}
}

func TestTheHomebaseCalendarAdvertisesItsPropertiesForPhones(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	cal := household.Member.ConnectCalendar()

	ms := cal.Propfind(cal.HomePath(), "1",
		apitest.DisplayName, apitest.ResourceType, apitest.GetCTag, apitest.CalendarColor,
		apitest.CurrentUserPrivilegeSet, apitest.SupportedComponentSet, apitest.SupportedReportSet)

	props, ok := ms[cal.CollectionPath("homebase")]
	if !ok {
		t.Fatalf("home listing %v has no homebase collection", ms)
	}
	if p := props[apitest.DisplayName]; p.Status != http.StatusOK || p.Text != "Homebase" {
		t.Errorf("displayname = %+v, want Homebase", p)
	}
	if p := props[apitest.ResourceType]; !p.Has("{urn:ietf:params:xml:ns:caldav}calendar") || !p.Has("{DAV:}collection") {
		t.Errorf("resourcetype = %+v, want a calendar collection", p)
	}
	if p := props[apitest.GetCTag]; p.Status != http.StatusOK || p.Text == "" {
		t.Errorf("getctag = %+v, want a change tag", p)
	}
	if p := props[apitest.CalendarColor]; p.Status != http.StatusOK || len(p.Text) != len("#RRGGBBAA") || p.Text[0] != '#' {
		t.Errorf("calendar-color = %+v, want #RRGGBBAA", p)
	}
	// Read-only until writes from devices arrive.
	if p := props[apitest.CurrentUserPrivilegeSet]; !p.Has("{DAV:}read") || p.Has("{DAV:}write") || p.Has("{DAV:}all") {
		t.Errorf("privileges = %+v, want read only", p)
	}
	if p := props[apitest.SupportedReportSet]; !p.Has("{urn:ietf:params:xml:ns:caldav}calendar-multiget") || !p.Has("{urn:ietf:params:xml:ns:caldav}calendar-query") || p.Has("{DAV:}sync-collection") {
		t.Errorf("supported reports = %+v, want multiget and query only", p)
	}
}

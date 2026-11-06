package api_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"homebase/internal/apitest"
)

func TestCalDAVLoginNeedsTheMembersEmailAndAnActiveConnectionSecret(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	cal := household.Member.ConnectCalendar()
	other := household.Owner.ConnectCalendar()
	propfind := func(authorization string) *apitest.Response {
		return h.CalDAVRequest("PROPFIND", cal.HomePath(), http.Header{"Authorization": {authorization}, "Depth": {"0"}}, "")
	}

	if got := propfind(apitest.BasicAuth(cal.Username, cal.Secret)).Status; got != http.StatusMultiStatus {
		t.Fatalf("right email and secret: status %d, want 207", got)
	}
	if got := propfind(apitest.BasicAuth(strings.ToUpper(cal.Username), cal.Secret)).Status; got != http.StatusMultiStatus {
		t.Errorf("email in another case: status %d, want 207", got)
	}

	refused := map[string]string{
		"wrong secret":               apitest.BasicAuth(cal.Username, cal.Secret+"x"),
		"wrong username":             apitest.BasicAuth(household.Owner.User.Email, cal.Secret),
		"another Member's secret":    apitest.BasicAuth(cal.Username, other.Secret),
		"an API token as the secret": apitest.BasicAuth(cal.Username, apiToken(t, household.Member)),
		"a connection secret bearer": "Bearer " + cal.Secret,
		"no credentials":             "",
	}
	for name, authorization := range refused {
		resp := propfind(authorization)
		if resp.Status != http.StatusUnauthorized {
			t.Errorf("%s: status %d, want 401", name, resp.Status)
			continue
		}
		if challenge := resp.Header.Get("WWW-Authenticate"); !strings.HasPrefix(challenge, "Basic ") {
			t.Errorf("%s: WWW-Authenticate = %q, want a Basic challenge", name, challenge)
		}
	}
}

func apiToken(t *testing.T, c *apitest.Client) string {
	t.Helper()
	created := apitest.Decode[struct {
		Token string `json:"token"`
	}](t, c.Post("/api/v1/api-tokens", map[string]any{"name": "script", "scope": "read"}).Expect(http.StatusCreated))
	return created.Token
}

func TestADisconnectedConnectionCanNoLongerLogIn(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	cal := household.Member.ConnectCalendar()
	cal.Do("PROPFIND", cal.HomePath(), http.Header{"Depth": {"0"}}, "").Expect(http.StatusMultiStatus)

	household.Member.Delete(fmt.Sprintf("/api/v1/calendar-connections/%d", cal.Connection.ID)).Expect(http.StatusOK)

	cal.Do("PROPFIND", cal.HomePath(), http.Header{"Depth": {"0"}}, "").Expect(http.StatusUnauthorized)
}

func TestARemovedMembersConnectionsStopWorking(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	cal := household.Member.ConnectCalendar()

	household.Owner.Delete(fmt.Sprintf("/api/v1/members/%d", household.Member.User.ID)).Expect(http.StatusOK)

	cal.Do("PROPFIND", cal.HomePath(), http.Header{"Depth": {"0"}}, "").Expect(http.StatusUnauthorized)
}

func TestASessionCookieIsNotACalDAVLogin(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	cal := household.Member.ConnectCalendar()

	household.Member.Do("PROPFIND", cal.HomePath(), nil).Expect(http.StatusUnauthorized)
	household.Member.Get(cal.CollectionPath("homebase")).Expect(http.StatusUnauthorized)
}

func TestAConnectionSecretIsNotAnAPILogin(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	cal := household.Member.ConnectCalendar()

	cal.Do(http.MethodGet, "/api/v1/me", nil, "").Expect(http.StatusUnauthorized)
	cal.Do(http.MethodGet, "/api/v1/me", http.Header{"Authorization": {"Bearer " + cal.Secret}}, "").Expect(http.StatusUnauthorized)
}

func TestAMemberCannotReadAnotherMembersCalendar(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	mine := household.Member.ConnectCalendar()
	theirs := household.Owner.ConnectCalendar()

	mine.Do("PROPFIND", theirs.CollectionPath("homebase"), http.Header{"Depth": {"1"}}, "").Expect(http.StatusForbidden)
	mine.Do("REPORT", theirs.CollectionPath("homebase"), nil, `<?xml version="1.0"?><C:calendar-query xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:"><D:prop><D:getetag/></D:prop><C:filter><C:comp-filter name="VCALENDAR"/></C:filter></C:calendar-query>`).Expect(http.StatusForbidden)
}

func TestEachCalDAVLoginRecordsWhenTheConnectionWasLastUsed(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	cal := household.Member.ConnectCalendar()
	listed := func() []calendarConnection {
		return apitest.Decode[[]calendarConnection](t, household.Member.Get("/api/v1/calendar-connections").Expect(http.StatusOK))
	}
	if got := listed(); got[0].LastUsedAt != nil {
		t.Fatalf("last used = %v before any use, want none", *got[0].LastUsedAt)
	}

	h.Clock.Advance(time.Hour)
	cal.Do("PROPFIND", cal.HomePath(), http.Header{"Depth": {"0"}}, "").Expect(http.StatusMultiStatus)

	got := listed()
	if got[0].LastUsedAt == nil {
		t.Fatalf("last used not recorded")
	}
	used, err := time.Parse(time.RFC3339Nano, *got[0].LastUsedAt)
	if want := apitest.DefaultNow.Add(time.Hour); err != nil || !used.Equal(want) {
		t.Errorf("last used = %s, want %s", *got[0].LastUsedAt, want)
	}
}

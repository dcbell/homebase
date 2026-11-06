package api_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"homebase/internal/apitest"
)

type createdIPhoneConnection struct {
	Connection calendarConnection `json:"connection"`
	ProfileURL string             `json:"profile_url"`
	Secret     string             `json:"secret"`
}

// setUpIPhone creates an ios Calendar Connection and returns the API's answer.
func setUpIPhone(t *testing.T, c *apitest.Client, name string) createdIPhoneConnection {
	t.Helper()
	return apitest.Decode[createdIPhoneConnection](t, c.Post("/api/v1/calendar-connections", map[string]any{
		"name": name, "kind": "ios",
	}).Expect(http.StatusCreated))
}

// downloadProfile fetches a profile link the way Safari would: no session.
func downloadProfile(t *testing.T, h *apitest.Harness, profileURL string) *apitest.Response {
	t.Helper()
	path, ok := strings.CutPrefix(profileURL, apitest.CalDAVBaseURL)
	if !ok {
		t.Fatalf("profile_url = %q, want it on the public web base URL %s", profileURL, apitest.CalDAVBaseURL)
	}
	return h.Anonymous().Get(path)
}

func TestAnIPhoneConnectionDownloadsAProfileThatLogsIn(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()

	created := setUpIPhone(t, household.Member, "Dave's iPhone")
	if created.Connection.Kind != "ios" || created.Connection.Name != "Dave's iPhone" {
		t.Errorf("connection = %+v, want Dave's iPhone / ios", created.Connection)
	}
	if created.Secret != "" {
		t.Errorf("response carries secret %q; it must exist only inside the profile", created.Secret)
	}

	resp := downloadProfile(t, h, created.ProfileURL).Expect(http.StatusOK)
	if got := resp.Header.Get("Content-Type"); got != "application/x-apple-aspen-config" {
		t.Errorf("Content-Type = %q, want application/x-apple-aspen-config", got)
	}
	if got := resp.Header.Get("Content-Disposition"); !strings.HasPrefix(got, "attachment;") || !strings.Contains(got, ".mobileconfig") {
		t.Errorf("Content-Disposition = %q, want an attachment named *.mobileconfig", got)
	}

	profile := apitest.ParseProfile(t, resp.Body)
	if profile.Keys["PayloadType"] != "Configuration" || profile.Keys["PayloadVersion"] != int64(1) {
		t.Errorf("top level = %v, want a version 1 Configuration", profile.Keys)
	}
	if len(profile.Payloads) != 1 {
		t.Fatalf("payloads = %v, want exactly one CalDAV account", profile.Payloads)
	}
	account := profile.Payloads[0]
	want := map[string]any{
		"PayloadType":              "com.apple.caldav.account",
		"CalDAVHostName":           "homebase.test",
		"CalDAVPort":               int64(80),
		"CalDAVUseSSL":             false,
		"CalDAVUsername":           household.Member.User.Email,
		"CalDAVPrincipalURL":       fmt.Sprintf("http://homebase.test/caldav/%d/", household.Member.User.ID),
		"CalDAVAccountDescription": "Homebase",
	}
	for key, value := range want {
		if account[key] != value {
			t.Errorf("CalDAV payload %s = %#v, want %#v", key, account[key], value)
		}
	}
	for _, key := range []string{"PayloadIdentifier", "PayloadUUID"} {
		if profile.Keys[key] == "" || profile.Keys[key] == nil || account[key] == "" || account[key] == nil {
			t.Errorf("%s missing: top %v, payload %v", key, profile.Keys[key], account[key])
		}
	}

	secret, _ := account["CalDAVPassword"].(string)
	cal := h.CalDAVRequest("PROPFIND", fmt.Sprintf("/caldav/%d/calendars/", household.Member.User.ID), http.Header{
		"Authorization": {apitest.BasicAuth(household.Member.User.Email, secret)},
		"Depth":         {"0"},
	}, "")
	if cal.Status != http.StatusMultiStatus {
		t.Errorf("logging in with the profile's secret: status %d, want 207", cal.Status)
	}
}

func TestAProfileLinkWorksOnce(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	created := setUpIPhone(t, household.Member, "iPhone")

	downloadProfile(t, h, created.ProfileURL).Expect(http.StatusOK)
	downloadProfile(t, h, created.ProfileURL).Expect(http.StatusNotFound)
}

func TestAProfileLinkExpires(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	created := setUpIPhone(t, household.Member, "iPhone")

	h.Clock.Advance(10 * time.Minute)
	downloadProfile(t, h, created.ProfileURL).Expect(http.StatusNotFound)
}

func TestAProfileLinkForADisconnectedConnectionIsDead(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	created := setUpIPhone(t, household.Member, "iPhone")
	household.Member.Delete(fmt.Sprintf("/api/v1/calendar-connections/%d", created.Connection.ID)).Expect(http.StatusOK)

	downloadProfile(t, h, created.ProfileURL).Expect(http.StatusNotFound)
}

func TestEachIPhoneProfileHasItsOwnIdentifiers(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	first := apitest.ParseProfile(t, downloadProfile(t, h, setUpIPhone(t, household.Member, "iPhone").ProfileURL).Expect(http.StatusOK).Body)
	second := apitest.ParseProfile(t, downloadProfile(t, h, setUpIPhone(t, household.Member, "iPad").ProfileURL).Expect(http.StatusOK).Body)

	for _, key := range []string{"PayloadIdentifier", "PayloadUUID"} {
		if first.Keys[key] == second.Keys[key] {
			t.Errorf("both profiles have %s %v", key, first.Keys[key])
		}
		if first.Payloads[0][key] == second.Payloads[0][key] {
			t.Errorf("both CalDAV payloads have %s %v", key, first.Payloads[0][key])
		}
	}
}

func TestTheProfileEscapesTheDeviceName(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	created := setUpIPhone(t, household.Member, `Tom & Jerry's <iPhone>`)

	profile := apitest.ParseProfile(t, downloadProfile(t, h, created.ProfileURL).Expect(http.StatusOK).Body)
	if name, _ := profile.Keys["PayloadDisplayName"].(string); !strings.Contains(name, `Tom & Jerry's <iPhone>`) {
		t.Errorf("PayloadDisplayName = %q, want it to name the device", name)
	}
}

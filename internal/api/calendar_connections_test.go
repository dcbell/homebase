package api_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"homebase/internal/apitest"
)

type calendarConnection struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	Kind       string  `json:"kind"`
	Secret     string  `json:"secret"`
	CreatedAt  string  `json:"created_at"`
	LastUsedAt *string `json:"last_used_at"`
	RevokedAt  *string `json:"revoked_at"`
}

type createdCalendarConnection struct {
	Connection calendarConnection `json:"connection"`
	ServerURL  string             `json:"server_url"`
	Username   string             `json:"username"`
	Secret     string             `json:"secret"`
}

func TestAnOtherDeviceConnectionShowsItsDetailsOnce(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()

	created := apitest.Decode[createdCalendarConnection](t, household.Member.Post("/api/v1/calendar-connections", map[string]any{
		"name": "Thunderbird",
		"kind": "other",
	}).Expect(http.StatusCreated))

	if created.Connection.Name != "Thunderbird" || created.Connection.Kind != "other" {
		t.Errorf("connection = %+v, want Thunderbird / other", created.Connection)
	}
	if created.ServerURL != "http://homebase.test/caldav/" {
		t.Errorf("server_url = %q, want http://homebase.test/caldav/", created.ServerURL)
	}
	if created.Username != household.Member.User.Email {
		t.Errorf("username = %q, want the Member's email %q", created.Username, household.Member.User.Email)
	}
	if len(created.Secret) < 32 {
		t.Errorf("secret = %q, want a long generated secret", created.Secret)
	}

	listed := household.Member.Get("/api/v1/calendar-connections").Expect(http.StatusOK)
	if strings.Contains(string(listed.Body), created.Secret) {
		t.Errorf("listing shows the secret: %s", listed.Body)
	}
	connections := apitest.Decode[[]calendarConnection](t, listed)
	if len(connections) != 1 || connections[0].ID != created.Connection.ID || connections[0].Secret != "" {
		t.Errorf("connections = %+v, want just the new one without a secret", connections)
	}
}

func TestAMemberSeesOnlyTheirOwnCalendarConnections(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	household.Owner.Post("/api/v1/calendar-connections", map[string]any{"name": "Owner's phone", "kind": "other"}).Expect(http.StatusCreated)

	connections := apitest.Decode[[]calendarConnection](t, household.Member.Get("/api/v1/calendar-connections").Expect(http.StatusOK))
	if len(connections) != 0 {
		t.Errorf("Member sees %+v, want none of the Owner's connections", connections)
	}
}

func TestCalendarConnectionsNeedANameAndAKnownKind(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()

	household.Member.Post("/api/v1/calendar-connections", map[string]any{"name": " ", "kind": "other"}).Expect(http.StatusBadRequest)
	household.Member.Post("/api/v1/calendar-connections", map[string]any{"name": "Phone", "kind": "fax"}).Expect(http.StatusBadRequest)
}

func TestCalendarConnectionsNeedABrowserSession(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	token := household.Member.WithToken("write")

	token.Get("/api/v1/calendar-connections").Expect(http.StatusForbidden)
	token.Post("/api/v1/calendar-connections", map[string]any{"name": "Phone", "kind": "other"}).Expect(http.StatusForbidden)
}

func TestDisconnectingACalendarConnection(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	created := apitest.Decode[createdCalendarConnection](t, household.Member.Post("/api/v1/calendar-connections", map[string]any{
		"name": "Tablet", "kind": "other",
	}).Expect(http.StatusCreated))
	path := fmt.Sprintf("/api/v1/calendar-connections/%d", created.Connection.ID)

	household.Owner.Delete(path).Expect(http.StatusNotFound)
	household.Member.Delete(path).Expect(http.StatusOK)
	household.Member.Delete(path).Expect(http.StatusNotFound)

	connections := apitest.Decode[[]calendarConnection](t, household.Member.Get("/api/v1/calendar-connections").Expect(http.StatusOK))
	if len(connections) != 1 || connections[0].RevokedAt == nil {
		t.Errorf("connections = %+v, want the one connection marked disconnected", connections)
	}
}

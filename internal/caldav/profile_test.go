package caldav_test

import (
	"testing"

	"homebase/internal/apitest"
	"homebase/internal/caldav"
)

func TestTheProfileTakesHostPortAndTLSFromTheWebBaseURL(t *testing.T) {
	cases := []struct {
		base      string
		host      string
		port      int64
		ssl       bool
		principal string
	}{
		{"https://home.example.com", "home.example.com", 443, true, "https://home.example.com/caldav/7/"},
		{"https://home.example.com:8443", "home.example.com", 8443, true, "https://home.example.com:8443/caldav/7/"},
		{"http://192.168.1.20:8080/", "192.168.1.20", 8080, false, "http://192.168.1.20:8080/caldav/7/"},
	}
	for _, tc := range cases {
		body, err := caldav.Profile{WebBaseURL: tc.base, ConnectionID: 3, DeviceName: "iPhone", UserID: 7, Username: "a@example.test", Secret: "s"}.Render()
		if err != nil {
			t.Fatalf("%s: %v", tc.base, err)
		}
		account := apitest.ParseProfile(t, body).Payloads[0]
		if account["CalDAVHostName"] != tc.host || account["CalDAVPort"] != tc.port || account["CalDAVUseSSL"] != tc.ssl || account["CalDAVPrincipalURL"] != tc.principal {
			t.Errorf("%s: host %v port %v ssl %v principal %v, want %s %d %v %s", tc.base,
				account["CalDAVHostName"], account["CalDAVPort"], account["CalDAVUseSSL"], account["CalDAVPrincipalURL"], tc.host, tc.port, tc.ssl, tc.principal)
		}
	}
}

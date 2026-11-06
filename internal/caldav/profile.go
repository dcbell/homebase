package caldav

import (
	"bytes"
	"crypto/sha256"
	"encoding/xml"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

// ProfileContentType is what iOS needs to offer a configuration profile for
// installation.
const ProfileContentType = "application/x-apple-aspen-config"

// ProfileFilename names a downloaded setup profile.
const ProfileFilename = "Homebase.mobileconfig"

// ProfileAccountDescription is what the CalDAV account is called on the
// device.
const ProfileAccountDescription = "Homebase"

// Profile describes an iPhone setup profile for one Calendar Connection.
type Profile struct {
	// WebBaseURL is the public web base URL; the account's host, port and
	// TLS come from it.
	WebBaseURL   string
	ConnectionID int64
	// DeviceName is the Calendar Connection's name.
	DeviceName string
	UserID     int64
	Username   string
	Secret     string
}

// Render returns the profile as an unsigned .mobileconfig: an XML property
// list with one CalDAV account payload. Its identifiers are unique to the
// connection, so installing a new connection's profile never replaces another.
func (p Profile) Render() ([]byte, error) {
	base, err := url.Parse(p.WebBaseURL)
	if err != nil || base.Hostname() == "" {
		return nil, fmt.Errorf("web base URL %q has no host", p.WebBaseURL)
	}
	useSSL := base.Scheme == "https"
	port := 80
	if useSSL {
		port = 443
	}
	if raw := base.Port(); raw != "" {
		if port, err = strconv.Atoi(raw); err != nil {
			return nil, fmt.Errorf("web base URL %q: bad port", p.WebBaseURL)
		}
	}
	principalURL := strings.TrimRight(p.WebBaseURL, "/") + principalPath(p.UserID)

	labels := strings.Split(base.Hostname(), ".")
	slices.Reverse(labels)
	identifier := fmt.Sprintf("%s.homebase.calendar-connection.%d", strings.Join(labels, "."), p.ConnectionID)

	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>PayloadContent</key>
	<array>
		<dict>
`)
	account := plistDict{
		{"CalDAVAccountDescription", ProfileAccountDescription},
		{"CalDAVHostName", base.Hostname()},
		{"CalDAVPassword", p.Secret},
		{"CalDAVPort", port},
		{"CalDAVPrincipalURL", principalURL},
		{"CalDAVUseSSL", useSSL},
		{"CalDAVUsername", p.Username},
		{"PayloadDescription", "Shows your Homebase calendar in the Calendar app."},
		{"PayloadDisplayName", "Homebase calendar"},
		{"PayloadIdentifier", identifier + ".caldav"},
		{"PayloadType", "com.apple.caldav.account"},
		{"PayloadUUID", profileUUID(identifier + ".caldav")},
		{"PayloadVersion", 1},
	}
	account.write(&b, "\t\t\t")
	b.WriteString("\t\t</dict>\n\t</array>\n")
	top := plistDict{
		{"PayloadDescription", "Adds your Homebase calendar to this device over CalDAV."},
		{"PayloadDisplayName", "Homebase calendar (" + p.DeviceName + ")"},
		{"PayloadIdentifier", identifier},
		{"PayloadRemovalDisallowed", false},
		{"PayloadType", "Configuration"},
		{"PayloadUUID", profileUUID(identifier)},
		{"PayloadVersion", 1},
	}
	top.write(&b, "\t")
	b.WriteString("</dict>\n</plist>\n")
	return b.Bytes(), nil
}

type plistDict []struct {
	key   string
	value any
}

func (d plistDict) write(b *bytes.Buffer, indent string) {
	for _, entry := range d {
		fmt.Fprintf(b, "%s<key>%s</key>\n%s", indent, entry.key, indent)
		switch v := entry.value.(type) {
		case string:
			b.WriteString("<string>")
			_ = xml.EscapeText(b, []byte(v))
			b.WriteString("</string>")
		case int:
			fmt.Fprintf(b, "<integer>%d</integer>", v)
		case bool:
			if v {
				b.WriteString("<true/>")
			} else {
				b.WriteString("<false/>")
			}
		}
		b.WriteString("\n")
	}
}

// profileUUID derives a stable UUID (version 4 layout) from an identifier, so
// a connection's profile always carries the same UUIDs.
func profileUUID(identifier string) string {
	sum := sha256.Sum256([]byte(identifier))
	u := sum[:16]
	u[6] = u[6]&0x0f | 0x40
	u[8] = u[8]&0x3f | 0x80
	return strings.ToUpper(fmt.Sprintf("%x-%x-%x-%x-%x", u[0:4], u[4:6], u[6:8], u[8:10], u[10:16]))
}

package apitest

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
	"testing"
)

// Profile is a parsed configuration profile (.mobileconfig): its top-level
// keys and each payload in PayloadContent. Values are string, int64, bool,
// []any or map[string]any, as in the plist.
type Profile struct {
	Keys     map[string]any
	Payloads []map[string]any
}

// ParseProfile parses an unsigned XML property list configuration profile,
// failing the test if it isn't one.
func ParseProfile(t testing.TB, body []byte) Profile {
	t.Helper()
	dec := xml.NewDecoder(bytes.NewReader(body))
	for {
		tok, err := dec.Token()
		if err != nil {
			t.Fatalf("profile: no <plist>: %v\n%s", err, body)
		}
		if start, ok := tok.(xml.StartElement); ok {
			if start.Name.Local != "plist" {
				t.Fatalf("profile: root element <%s>, want <plist>", start.Name.Local)
			}
			break
		}
	}
	value, err := plistValue(dec)
	if err != nil {
		t.Fatalf("profile: %v\n%s", err, body)
	}
	keys, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("profile: top level is %T, want a dict", value)
	}
	p := Profile{Keys: keys}
	content, _ := keys["PayloadContent"].([]any)
	for _, item := range content {
		payload, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("profile: payload is %T, want a dict", item)
		}
		p.Payloads = append(p.Payloads, payload)
	}
	return p
}

// errEnd marks the end of a dict or array.
var errEnd = io.EOF

// plistValue reads the next value element.
func plistValue(dec *xml.Decoder) (any, error) {
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch tok := tok.(type) {
		case xml.EndElement:
			return nil, errEnd
		case xml.StartElement:
			return plistElement(dec, tok)
		}
	}
}

func plistElement(dec *xml.Decoder, start xml.StartElement) (any, error) {
	switch start.Name.Local {
	case "dict":
		dict := map[string]any{}
		for {
			key, err := plistValue(dec)
			if err == errEnd {
				return dict, nil
			}
			if err != nil {
				return nil, err
			}
			name, ok := key.(plistKey)
			if !ok {
				return nil, fmt.Errorf("dict entry %v is not a <key>", key)
			}
			value, err := plistValue(dec)
			if err != nil {
				return nil, fmt.Errorf("key %s: %w", name, err)
			}
			dict[string(name)] = value
		}
	case "array":
		array := []any{}
		for {
			value, err := plistValue(dec)
			if err == errEnd {
				return array, nil
			}
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
	case "true", "false":
		if err := dec.Skip(); err != nil {
			return nil, err
		}
		return start.Name.Local == "true", nil
	}
	var text string
	if err := dec.DecodeElement(&text, &start); err != nil {
		return nil, err
	}
	switch start.Name.Local {
	case "key":
		return plistKey(text), nil
	case "string":
		return text, nil
	case "integer":
		return strconv.ParseInt(strings.TrimSpace(text), 10, 64)
	}
	return nil, fmt.Errorf("unsupported plist element <%s>", start.Name.Local)
}

type plistKey string

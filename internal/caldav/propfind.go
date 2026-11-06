package caldav

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// We answer PROPFIND ourselves rather than through go-webdav, because phones
// need properties it doesn't have: getctag, calendar-color, read-only
// privileges and supported-report-set.

const (
	nsDAV    = "DAV:"
	nsCalDAV = "urn:ietf:params:xml:ns:caldav"
	nsCS     = "http://calendarserver.org/ns/"
	nsApple  = "http://apple.com/ns/ical/"
)

// prefixes are the namespace prefixes declared on every multistatus.
var prefixes = map[string]string{nsDAV: "D", nsCalDAV: "C", nsCS: "CS", nsApple: "A"}

type propfindRequest struct {
	XMLName  xml.Name  `xml:"DAV: propfind"`
	Prop     *propList `xml:"DAV: prop"`
	AllProp  *struct{} `xml:"DAV: allprop"`
	PropName *struct{} `xml:"DAV: propname"`
}

type propList struct {
	Names []struct {
		XMLName xml.Name
	} `xml:",any"`
}

// resource is one href in a multistatus with the properties it has. Each
// property value is inner XML using the prefixes above.
type resource struct {
	href  string
	props map[xml.Name]string
	// order is the property order for allprop and propname.
	order []xml.Name
}

func (r *resource) set(space, local, inner string) {
	name := xml.Name{Space: space, Local: local}
	if _, ok := r.props[name]; !ok {
		r.order = append(r.order, name)
	}
	r.props[name] = inner
}

func newResource(href string) *resource {
	return &resource{href: href, props: map[xml.Name]string{}}
}

func (s *Server) propfind(w http.ResponseWriter, r *http.Request, p davPath) {
	var req propfindRequest
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "failed to read request", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(string(body)) == "" {
		req.AllProp = &struct{}{}
	} else if err := xml.Unmarshal(body, &req); err != nil {
		http.Error(w, "malformed PROPFIND body", http.StatusBadRequest)
		return
	}

	depth := r.Header.Get("Depth")
	if depth == "" {
		depth = "infinity"
	}
	// Depth infinity is treated as 1: nothing here is deeper than a listing
	// clients need.
	children := depth != "0"

	resources, status, err := s.resources(r, p, children)
	if err != nil {
		s.logger.Error("caldav propfind", "path", r.URL.Path, "error", err)
		http.Error(w, "failed to load calendar", http.StatusInternalServerError)
		return
	}
	if status != http.StatusOK {
		http.Error(w, http.StatusText(status), status)
		return
	}

	var out strings.Builder
	out.WriteString(xml.Header)
	out.WriteString(`<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:CS="http://calendarserver.org/ns/" xmlns:A="http://apple.com/ns/ical/">`)
	for _, res := range resources {
		writeResponse(&out, res, req)
	}
	out.WriteString(`</D:multistatus>`)

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(http.StatusMultiStatus)
	_, _ = io.WriteString(w, out.String())
}

func writeResponse(out *strings.Builder, res *resource, req propfindRequest) {
	out.WriteString(`<D:response><D:href>`)
	_ = xml.EscapeText(out, []byte(res.href))
	out.WriteString(`</D:href>`)

	var found, missing []xml.Name
	switch {
	case req.Prop != nil:
		for _, n := range req.Prop.Names {
			if _, ok := res.props[n.XMLName]; ok {
				found = append(found, n.XMLName)
			} else {
				missing = append(missing, n.XMLName)
			}
		}
	default:
		found = res.order
	}

	if len(found) > 0 {
		out.WriteString(`<D:propstat><D:prop>`)
		for _, name := range found {
			inner := res.props[name]
			if req.PropName != nil {
				inner = ""
			}
			writeElement(out, name, inner)
		}
		out.WriteString(`</D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat>`)
	}
	if len(missing) > 0 {
		out.WriteString(`<D:propstat><D:prop>`)
		for _, name := range missing {
			writeElement(out, name, "")
		}
		out.WriteString(`</D:prop><D:status>HTTP/1.1 404 Not Found</D:status></D:propstat>`)
	}
	out.WriteString(`</D:response>`)
}

func writeElement(out *strings.Builder, name xml.Name, inner string) {
	tag := name.Local
	open := tag
	if prefix, ok := prefixes[name.Space]; ok {
		tag = prefix + ":" + name.Local
		open = tag
	} else {
		var ns strings.Builder
		_ = xml.EscapeText(&ns, []byte(name.Space))
		open = fmt.Sprintf(`%s xmlns="%s"`, name.Local, ns.String())
	}
	if inner == "" {
		fmt.Fprintf(out, "<%s/>", open)
		return
	}
	fmt.Fprintf(out, "<%s>%s</%s>", open, inner, tag)
}

func escape(text string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(text))
	return b.String()
}

func hrefXML(href string) string {
	return "<D:href>" + escape(href) + "</D:href>"
}

// resources returns the resource at p and, when children is set, the
// resources directly inside it.
func (s *Server) resources(r *http.Request, p davPath, children bool) ([]*resource, int, error) {
	member := requestFrom(r.Context()).member
	switch p.kind {
	case kindRoot:
		res := []*resource{s.rootResource(member)}
		if children {
			res = append(res, s.principalResource(member))
		}
		return res, http.StatusOK, nil
	case kindPrincipal:
		res := []*resource{s.principalResource(member)}
		if children {
			res = append(res, s.homeResource(member))
		}
		return res, http.StatusOK, nil
	case kindHome:
		res := []*resource{s.homeResource(member)}
		if children {
			for _, c := range s.collections {
				cr, err := s.collectionResource(r, member, c)
				if err != nil {
					return nil, 0, err
				}
				res = append(res, cr)
			}
		}
		return res, http.StatusOK, nil
	case kindCollection:
		c := s.collection(p.collection)
		if c == nil {
			return nil, http.StatusNotFound, nil
		}
		cr, err := s.collectionResource(r, member, c)
		if err != nil {
			return nil, 0, err
		}
		res := []*resource{cr}
		if children {
			objects, err := s.objects(r.Context(), c)
			if err != nil {
				return nil, 0, err
			}
			for _, o := range objects {
				res = append(res, objectResource(c, o))
			}
		}
		return res, http.StatusOK, nil
	case kindObject:
		c := s.collection(p.collection)
		if c == nil {
			return nil, http.StatusNotFound, nil
		}
		objects, err := s.objects(r.Context(), c)
		if err != nil {
			return nil, 0, err
		}
		for _, o := range objects {
			if o.UID == p.uid {
				return []*resource{objectResource(c, o)}, http.StatusOK, nil
			}
		}
		return nil, http.StatusNotFound, nil
	}
	return nil, http.StatusNotFound, nil
}

func (s *Server) rootResource(m CalendarLogin) *resource {
	res := newResource(Root)
	res.set(nsDAV, "resourcetype", "<D:collection/>")
	res.set(nsDAV, "current-user-principal", hrefXML(principalPath(m.UserID)))
	return res
}

func (s *Server) principalResource(m CalendarLogin) *resource {
	res := newResource(principalPath(m.UserID))
	res.set(nsDAV, "resourcetype", "<D:collection/><D:principal/>")
	res.set(nsDAV, "displayname", escape(m.Name))
	res.set(nsDAV, "current-user-principal", hrefXML(principalPath(m.UserID)))
	res.set(nsDAV, "principal-URL", hrefXML(principalPath(m.UserID)))
	res.set(nsCalDAV, "calendar-home-set", hrefXML(homePath(m.UserID)))
	return res
}

func (s *Server) homeResource(m CalendarLogin) *resource {
	res := newResource(homePath(m.UserID))
	res.set(nsDAV, "resourcetype", "<D:collection/>")
	res.set(nsDAV, "current-user-principal", hrefXML(principalPath(m.UserID)))
	res.set(nsDAV, "current-user-privilege-set", privileges(false))
	return res
}

func (s *Server) collectionResource(r *http.Request, m CalendarLogin, c Collection) (*resource, error) {
	objects, err := s.objects(r.Context(), c)
	if err != nil {
		return nil, err
	}
	res := newResource(collectionPath(m.UserID, c.Name()))
	res.set(nsDAV, "resourcetype", "<D:collection/><C:calendar/>")
	res.set(nsDAV, "displayname", escape(c.DisplayName()))
	res.set(nsDAV, "current-user-principal", hrefXML(principalPath(m.UserID)))
	res.set(nsDAV, "owner", hrefXML(principalPath(m.UserID)))
	// Devices show a collection read-only unless it is Writable.
	_, writable := c.(Writable)
	res.set(nsDAV, "current-user-privilege-set", privileges(writable))
	res.set(nsDAV, "supported-report-set",
		"<D:supported-report><D:report><C:calendar-multiget/></D:report></D:supported-report>"+
			"<D:supported-report><D:report><C:calendar-query/></D:report></D:supported-report>")
	res.set(nsCalDAV, "supported-calendar-component-set", `<C:comp name="`+escape(c.Component())+`"/>`)
	res.set(nsCS, "getctag", escape(ctag(objects)))
	res.set(nsApple, "calendar-color", escape(c.Color()))
	res.set(nsApple, "calendar-order", strconv.Itoa(s.order(c)))
	return res, nil
}

func (s *Server) order(c Collection) int {
	for i, other := range s.collections {
		if other.Name() == c.Name() {
			return i + 1
		}
	}
	return 0
}

func objectResource(c Collection, o object) *resource {
	res := newResource(o.href)
	res.set(nsDAV, "resourcetype", "")
	res.set(nsDAV, "getetag", escape(`"`+o.etag+`"`))
	res.set(nsDAV, "getcontenttype", escape("text/calendar; charset=utf-8; component="+strings.ToLower(c.Component())))
	res.set(nsDAV, "getcontentlength", strconv.Itoa(len(o.data)))
	if !o.ModTime.IsZero() {
		res.set(nsDAV, "getlastmodified", escape(o.ModTime.UTC().Format(http.TimeFormat)))
	}
	res.set(nsCalDAV, "calendar-data", escape(string(o.data)))
	return res
}

func privileges(write bool) string {
	set := "<D:privilege><D:read/></D:privilege><D:privilege><D:read-current-user-privilege-set/></D:privilege>"
	if write {
		set += "<D:privilege><D:write/></D:privilege><D:privilege><D:write-content/></D:privilege>" +
			"<D:privilege><D:bind/></D:privilege><D:privilege><D:unbind/></D:privilege>"
	}
	return set
}

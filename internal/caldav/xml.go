// SPDX-License-Identifier: AGPL-3.0-or-later

package caldav

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// The namespaces tix answers in, each written with a stable prefix.
const (
	nsDAV    = "DAV:"
	nsCalDAV = "urn:ietf:params:xml:ns:caldav"
	nsCS     = "http://calendarserver.org/ns/"
	nsApple  = "http://apple.com/ns/ical/"
)

// Bounds of a PROPFIND, REPORT or PROPPATCH body.
const (
	maxBodyBytes = 64 << 10
	maxDepth     = 32
	maxElements  = 2000
)

// prefixes maps each namespace tix writes to its prefix.
var prefixes = map[string]string{nsDAV: "d", nsCalDAV: "c", nsCS: "cs", nsApple: "ical"}

// xmlContentType is the media type of every XML answer.
const xmlContentType = "application/xml; charset=utf-8"

// errBadBody is what every refused request body wraps: the handler answers it
// with 400 and never evaluates the body.
var errBadBody = errors.New("bad request body")

// element is one decoded XML element with its children and its text.
type element struct {
	name     xml.Name
	attrs    []xml.Attr
	children []*element
	text     string
}

// is reports whether the element has the namespace and local name.
func (e *element) is(space, local string) bool {
	return e != nil && e.name.Space == space && e.name.Local == local
}

// child returns the first child of a name, or nil.
func (e *element) child(space, local string) *element {
	if e == nil {
		return nil
	}
	for _, c := range e.children {
		if c.is(space, local) {
			return c
		}
	}
	return nil
}

// all returns every child of a name.
func (e *element) all(space, local string) []*element {
	var out []*element
	for _, c := range e.children {
		if c.is(space, local) {
			out = append(out, c)
		}
	}
	return out
}

// attr returns the value of an attribute in no namespace.
func (e *element) attr(local string) (string, bool) {
	for _, a := range e.attrs {
		if a.Name.Local == local && (a.Name.Space == "" || a.Name.Space == e.name.Space) {
			return a.Value, true
		}
	}
	return "", false
}

// readBody reads a request body of at most maxBodyBytes and decodes it. An
// empty body is a nil element and no error.
func readBody(r io.Reader) (*element, error) {
	raw, err := io.ReadAll(io.LimitReader(r, maxBodyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: reading: %w", errBadBody, err)
	}
	return decodeBody(raw)
}

// decodeBody decodes a body under the bounds, refusing any document type
// declaration, directive, entity, nesting past maxDepth and more than
// maxElements elements. An empty body is a nil element.
func decodeBody(raw []byte) (*element, error) {
	if len(raw) > maxBodyBytes {
		return nil, fmt.Errorf("%w: larger than %d bytes", errBadBody, maxBodyBytes)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	d := xml.NewDecoder(bytes.NewReader(raw))
	d.Strict = true
	var (
		root  *element
		stack []*element
		count int
	)
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w: %w", errBadBody, err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			count++
			if count > maxElements {
				return nil, fmt.Errorf("%w: more than %d elements", errBadBody, maxElements)
			}
			if len(stack) >= maxDepth {
				return nil, fmt.Errorf("%w: nested deeper than %d", errBadBody, maxDepth)
			}
			e := &element{name: t.Name, attrs: t.Copy().Attr}
			if len(stack) == 0 {
				if root != nil {
					return nil, fmt.Errorf("%w: more than one root element", errBadBody)
				}
				root = e
			} else {
				parent := stack[len(stack)-1]
				parent.children = append(parent.children, e)
			}
			stack = append(stack, e)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].text += string(t)
			} else if len(bytes.TrimSpace(t)) > 0 {
				return nil, fmt.Errorf("%w: text outside the root element", errBadBody)
			}
		case xml.Directive:
			return nil, fmt.Errorf("%w: a directive", errBadBody)
		case xml.ProcInst:
			if t.Target != "xml" || root != nil {
				return nil, fmt.Errorf("%w: a processing instruction", errBadBody)
			}
		}
	}
	if root == nil {
		return nil, fmt.Errorf("%w: no root element", errBadBody)
	}
	return root, nil
}

// prop is one property as answered: its name and its value as XML.
type prop struct {
	name  xml.Name
	value string
}

// propstat is a group of properties sharing one status.
type propstat struct {
	status int
	props  []prop
}

// response is one href of a multistatus: either a status of its own or
// propstats.
type response struct {
	href      string
	status    int
	propstats []propstat
}

// statusLine is the DAV:status text of an HTTP status.
func statusLine(code int) string {
	return fmt.Sprintf("HTTP/1.1 %d %s", code, http.StatusText(code))
}

// textEscaper writes XML text: markup characters escaped, and carriage
// returns too, which an XML reader would otherwise drop from calendar-data.
var textEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\r", "&#13;")

// attrEscaper writes a double-quoted attribute value.
var attrEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")

// escape is s as XML text.
func escape(s string) string { return textEscaper.Replace(s) }

// openTag and closeTag write an element name with the prefix of its
// namespace, declaring a namespace tix does not know on the element itself.
func openTag(n xml.Name) string {
	if p, ok := prefixes[n.Space]; ok {
		return "<" + p + ":" + n.Local + ">"
	}
	return "<" + n.Local + ` xmlns="` + attrEscaper.Replace(n.Space) + `">`
}

func closeTag(n xml.Name) string {
	if p, ok := prefixes[n.Space]; ok {
		return "</" + p + ":" + n.Local + ">"
	}
	return "</" + n.Local + ">"
}

// emptyTag writes an element with no content.
func emptyTag(n xml.Name) string {
	if p, ok := prefixes[n.Space]; ok {
		return "<" + p + ":" + n.Local + "/>"
	}
	return "<" + n.Local + ` xmlns="` + attrEscaper.Replace(n.Space) + `"/>`
}

// wrap writes an element around inner XML.
func wrap(n xml.Name, inner string) string {
	if inner == "" {
		return emptyTag(n)
	}
	return openTag(n) + inner + closeTag(n)
}

// davName, calName, csName and appleName name an element in a known namespace.
func davName(local string) xml.Name   { return xml.Name{Space: nsDAV, Local: local} }
func calName(local string) xml.Name   { return xml.Name{Space: nsCalDAV, Local: local} }
func csName(local string) xml.Name    { return xml.Name{Space: nsCS, Local: local} }
func appleName(local string) xml.Name { return xml.Name{Space: nsApple, Local: local} }

// rootOpen is the start of an answer's root element, declaring every prefix.
func rootOpen(local string) string {
	return `<?xml version="1.0" encoding="utf-8"?>` + "\n" + `<d:` + local + ` xmlns:d="` + nsDAV +
		`" xmlns:c="` + nsCalDAV + `" xmlns:cs="` + nsCS + `" xmlns:ical="` + nsApple + `">`
}

// multistatus writes a 207 body. A non-nil syncToken adds DAV:sync-token.
func multistatus(responses []response, syncToken *string) []byte {
	var b strings.Builder
	b.WriteString(rootOpen("multistatus"))
	for _, r := range responses {
		b.WriteString("<d:response><d:href>" + escape(r.href) + "</d:href>")
		if r.status != 0 {
			b.WriteString("<d:status>" + statusLine(r.status) + "</d:status>")
		}
		for _, ps := range r.propstats {
			if len(ps.props) == 0 {
				continue
			}
			b.WriteString("<d:propstat><d:prop>")
			for _, p := range ps.props {
				b.WriteString(wrap(p.name, p.value))
			}
			b.WriteString("</d:prop><d:status>" + statusLine(ps.status) + "</d:status></d:propstat>")
		}
		b.WriteString("</d:response>")
	}
	if syncToken != nil {
		b.WriteString("<d:sync-token>" + escape(*syncToken) + "</d:sync-token>")
	}
	b.WriteString("</d:multistatus>\n")
	return []byte(b.String())
}

// errorBody writes a DAV:error body naming one precondition.
func errorBody(precondition xml.Name, inner string) []byte {
	return []byte(rootOpen("error") + wrap(precondition, inner) + "</d:error>\n")
}

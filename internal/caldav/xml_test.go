// SPDX-License-Identifier: AGPL-3.0-or-later

package caldav

import (
	"encoding/xml"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func nested(n int) string {
	return strings.Repeat("<d:x>", n-1) + strings.Repeat("</d:x>", n-1)
}

func TestTheDecoderRefusesWhatBreaksItsBounds(t *testing.T) {
	const open = `<d:propfind xmlns:d="DAV:">`
	const end = `</d:propfind>`
	tests := []struct {
		name string
		body string
		ok   bool
	}{
		{"a plain propfind", open + `<d:prop><d:displayname/></d:prop>` + end, true},
		{"an xml declaration", `<?xml version="1.0" encoding="utf-8"?>` + open + end, true},
		{"an empty body", "  \n", true},
		{"exactly the size bound", open + strings.Repeat(" ", maxBodyBytes-len(open)-len(end)) + end, true},
		{"one byte past the size bound", open + strings.Repeat(" ", maxBodyBytes-len(open)-len(end)+1) + end, false},
		{"exactly the depth bound", open + nested(maxDepth) + end, true},
		{"nested one deeper than the bound", open + nested(maxDepth+1) + end, false},
		{"exactly the element bound", open + strings.Repeat("<d:x/>", maxElements-1) + end, true},
		{"one element past the bound", open + strings.Repeat("<d:x/>", maxElements) + end, false},
		{"a doctype", `<!DOCTYPE propfind>` + open + end, false},
		{"an entity declaration", `<!DOCTYPE d [<!ENTITY x "boom">]>` + open + `<d:prop>&x;</d:prop>` + end, false},
		{"a bare entity declaration", `<!ENTITY x "boom">` + open + end, false},
		{"another directive", open + `<!ELEMENT x ANY>` + end, false},
		{"an undefined entity", open + `<d:prop>&x;</d:prop>` + end, false},
		{"a processing instruction", open + `<?php echo 1 ?>` + end, false},
		{"two roots", open + end + open + end, false},
		{"text outside the root", "x" + open + end, false},
		{"unclosed", open, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeBody([]byte(tc.body))
			if tc.ok && err != nil {
				t.Fatalf("refused: %v", err)
			}
			if !tc.ok && !errors.Is(err, errBadBody) {
				t.Fatalf("accepted, or refused as %v rather than a bad body", err)
			}
		})
	}
}

func TestReadBodyStopsAtTheBound(t *testing.T) {
	body := strings.NewReader(`<d:propfind xmlns:d="DAV:">` + strings.Repeat(" ", 10*maxBodyBytes) + `</d:propfind>`)
	if _, err := readBody(body); !errors.Is(err, errBadBody) {
		t.Fatalf("a body ten times the bound was read: %v", err)
	}
}

// walk counts a decoded tree's elements and its depth.
func walk(e *element, d int) (count, depth int) {
	count, depth = 1, d
	for _, c := range e.children {
		n, cd := walk(c, d+1)
		count += n
		depth = max(depth, cd)
	}
	return count, depth
}

func FuzzDecodeBody(f *testing.F) {
	for _, seed := range []string{
		`<d:propfind xmlns:d="DAV:"><d:prop><d:getetag/></d:prop></d:propfind>`,
		`<?xml version="1.0"?><c:calendar-query xmlns:c="urn:ietf:params:xml:ns:caldav"><c:filter/></c:calendar-query>`,
		`<!DOCTYPE x [<!ENTITY a "b">]><x>&a;</x>`,
		`<a><b><c/></b></a>`,
		``,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		root, err := decodeBody(raw)
		if err != nil {
			if !errors.Is(err, errBadBody) {
				t.Fatalf("refused without errBadBody: %v", err)
			}
			return
		}
		if len(raw) > maxBodyBytes {
			t.Fatalf("accepted %d bytes", len(raw))
		}
		if root == nil {
			return
		}
		count, depth := walk(root, 1)
		if count > maxElements || depth > maxDepth {
			t.Fatalf("accepted %d elements %d deep", count, depth)
		}
	})
}

func TestMultistatusWritesStablePrefixesAndDeclaresUnknownNamespaces(t *testing.T) {
	token := "https://tix.red/ns/sync/ab"
	got := string(multistatus([]response{{href: "/dav/a b/", propstats: []propstat{
		{status: http.StatusOK, props: []prop{{name: davName("displayname"), value: escape("R&D <ops>")},
			{name: csName("getctag"), value: "1"}, {name: appleName("calendar-color"), value: "#112233ff"},
			{name: calName("calendar-data"), value: escape("A\r\nB")}}},
		{status: http.StatusNotFound, props: []prop{{name: xml.Name{Space: "urn:x", Local: "odd"}}}},
		{status: http.StatusForbidden},
	}}, {href: "/gone.ics", status: http.StatusNotFound}}, &token))
	for _, want := range []string{
		`xmlns:d="DAV:"`, `xmlns:c="urn:ietf:params:xml:ns:caldav"`, `xmlns:cs="http://calendarserver.org/ns/"`,
		`xmlns:ical="http://apple.com/ns/ical/"`,
		`<d:displayname>R&amp;D &lt;ops&gt;</d:displayname>`, `<cs:getctag>1</cs:getctag>`,
		`<ical:calendar-color>#112233ff</ical:calendar-color>`, `<c:calendar-data>A&#13;` + "\nB</c:calendar-data>",
		`<odd xmlns="urn:x"/>`, `<d:status>HTTP/1.1 404 Not Found</d:status>`,
		`<d:response><d:href>/gone.ics</d:href><d:status>HTTP/1.1 404 Not Found</d:status></d:response>`,
		`<d:sync-token>https://tix.red/ns/sync/ab</d:sync-token></d:multistatus>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in\n%s", want, got)
		}
	}
	if strings.Contains(got, "403") {
		t.Errorf("an empty propstat was written:\n%s", got)
	}
	var parsed struct {
		Responses []struct {
			Href string `xml:"DAV: href"`
		} `xml:"DAV: response"`
	}
	if err := xml.Unmarshal([]byte(got), &parsed); err != nil || len(parsed.Responses) != 2 {
		t.Fatalf("the answer does not parse: %v, %+v", err, parsed)
	}
}

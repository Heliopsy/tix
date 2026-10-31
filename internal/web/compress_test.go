// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/web"
)

// TestScreensAndAssetsGoOutCompressed asks for a screen and the stylesheet the
// way a browser does and reads the bytes off the wire, since Go's client
// would otherwise inflate them and hide whether they were ever compressed.
func TestScreensAndAssetsGoOutCompressed(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.createTask("infra", "a row to send")

	for _, path := range []string{"/tasks", web.RouteTasksLive, "/assets/app.css", "/assets/htmx.min.js"} {
		resp := getEncoded(b, path, "gzip, deflate, br")
		wantStatus(t, resp, http.StatusOK)
		if got := resp.Header.Get("Content-Encoding"); got != "gzip" {
			t.Errorf("%s: Content-Encoding = %q, want gzip", path, got)
		}
		if vary := strings.Join(resp.Header.Values("Vary"), ","); !strings.Contains(vary, "Accept-Encoding") {
			t.Errorf("%s: Vary = %q, so a shared cache may hand gzip to a client that cannot read it", path, vary)
		}
		wire := readAll(t, resp)
		zr, err := gzip.NewReader(strings.NewReader(wire))
		if err != nil {
			t.Fatalf("%s: the body is not gzip: %v", path, err)
		}
		plain, err := io.ReadAll(zr)
		if err != nil {
			t.Fatalf("%s: inflating: %v", path, err)
		}
		if len(wire)*3 > len(plain) {
			t.Errorf("%s: %d bytes on the wire for %d of content, want at most a third", path, len(wire), len(plain))
		}
	}
}

// TestAClientThatRefusesGzipGetsPlainBytes keeps the compression an offer
// rather than an imposition.
func TestAClientThatRefusesGzipGetsPlainBytes(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	for _, encoding := range []string{"identity", "gzip;q=0", ""} {
		for _, path := range []string{"/tasks", "/assets/app.css"} {
			resp := getEncoded(b, path, encoding)
			wantStatus(t, resp, http.StatusOK)
			if got := resp.Header.Get("Content-Encoding"); got != "" {
				t.Errorf("%s with Accept-Encoding %q: Content-Encoding = %q, want none", path, encoding, got)
			}
			if page := readAll(t, resp); !strings.Contains(page, "<") {
				t.Errorf("%s with Accept-Encoding %q: body is not readable text", path, encoding)
			}
		}
	}
}

// TestARedirectIsNotCompressedIntoSomethingElse posts a form and checks the
// See Other still reads as one: the wrapper must not swallow the status or
// the Location.
func TestARedirectIsNotCompressedIntoSomethingElse(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "star me")

	resp := b.post("/tasks/"+ref+"/star", url.Values{"next": {"/tasks"}})
	wantStatus(t, resp, http.StatusSeeOther)
	if loc := resp.Header.Get("Location"); !strings.HasPrefix(loc, "/tasks") {
		t.Errorf("Location = %q, want the listing", loc)
	}
}

var formToken = regexp.MustCompile(`name="csrf_token" value="([^"]+)"`)

// TestTheCSRFValueOnAPageIsMaskedPerResponse is what makes compressing a
// screen safe. A compressed page that reflects input and carries a constant
// secret gives the secret away to anyone who can steer requests and watch
// their sizes, so the secret must never be on the page as itself, and must
// read differently on every render.
func TestTheCSRFValueOnAPageIsMaskedPerResponse(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.createTask("infra", "a row with forms")
	cookie := b.csrf()

	first := formToken.FindStringSubmatch(b.page("/tasks"))
	second := formToken.FindStringSubmatch(b.page("/tasks"))
	if first == nil || second == nil {
		t.Fatal("the listing carries no form token to compare")
	}
	if first[1] == cookie || strings.Contains(b.page("/tasks"), cookie) {
		t.Error("the CSRF cookie's own value is on the page")
	}
	if first[1] == second[1] {
		t.Error("two renders carry the same form token, so compression can recover it")
	}

	ref := b.createTask("infra", "star me")
	for _, tc := range []struct {
		name  string
		token string
		want  int
	}{
		{"a token from one page", first[1], http.StatusSeeOther},
		{"a token from another", second[1], http.StatusSeeOther},
		{"the cookie value itself", cookie, http.StatusSeeOther},
		{"a token with one byte changed", flipOne(first[1]), http.StatusForbidden},
	} {
		resp := b.postRaw("/tasks/"+ref+"/star", url.Values{"csrf_token": {tc.token}, "next": {"/tasks"}})
		_ = resp.Body.Close()
		if resp.StatusCode != tc.want {
			t.Errorf("%s: status %d, want %d", tc.name, resp.StatusCode, tc.want)
		}
	}
}

// flipOne changes one character early in a base64 value to another, where
// every one of its six bits is data.
func flipOne(s string) string {
	swap := byte('A')
	if s[5] == 'A' {
		swap = 'B'
	}
	return s[:5] + string(swap) + s[6:]
}

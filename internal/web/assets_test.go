// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"testing"
)

// TestEveryAssetCarriesACacheValidator pins the fix for an upgrade that served
// new markup against the previous release's stylesheet.
//
// The asset URLs never change between releases. Served without Cache-Control
// and without a validator, a browser cached them heuristically, so after an
// upgrade a returning reader ran the old CSS against the current HTML and
// reported the interface as broken.
func TestEveryAssetCarriesACacheValidator(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	tests := []struct {
		path        string
		contentType string
	}{
		{"/assets/app.css", "text/css; charset=utf-8"},
		{"/assets/copy.js", "text/javascript; charset=utf-8"},
		{"/assets/htmx.min.js", "text/javascript; charset=utf-8"},
		{"/assets/live.js", "text/javascript; charset=utf-8"},
		{"/assets/decide.js", "text/javascript; charset=utf-8"},
		{"/assets/shortcuts.js", "text/javascript; charset=utf-8"},
		{"/assets/tix.svg", "image/svg+xml"},
	}
	seen := make(map[string]string, len(tests))
	for _, tt := range tests {
		resp := b.get(tt.path)
		wantStatus(t, resp, http.StatusOK)
		content := body(t, resp)

		etag := resp.Header.Get("ETag")
		switch {
		case etag == "":
			t.Errorf("%s carries no ETag, so a browser cannot revalidate it", tt.path)
		case strings.HasPrefix(etag, "W/"):
			t.Errorf("%s carries the weak ETag %s, want a strong one", tt.path, etag)
		case !strings.HasPrefix(etag, `"`) || !strings.HasSuffix(etag, `"`):
			t.Errorf("%s carries the unquoted ETag %s", tt.path, etag)
		}
		if cc := resp.Header.Get("Cache-Control"); !strings.Contains(cc, "max-age=") {
			t.Errorf("%s carries Cache-Control %q, which names no freshness lifetime", tt.path, cc)
		}
		if got := resp.Header.Get("Content-Type"); got != tt.contentType {
			t.Errorf("%s served as %q, want %q", tt.path, got, tt.contentType)
		}
		if content == "" {
			t.Errorf("%s served an empty body", tt.path)
		}
		if prev, ok := seen[etag]; ok {
			t.Errorf("%s and %s share the ETag %s, so one cannot invalidate without the other",
				tt.path, prev, etag)
		}
		seen[etag] = tt.path
	}
}

// TestAConditionalAssetRequestIsAnswered304 proves the validator is honoured
// rather than merely announced.
func TestAConditionalAssetRequestIsAnswered304(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	for _, path := range []string{"/assets/app.css", "/assets/htmx.min.js", "/assets/tix.svg"} {
		first := b.get(path)
		wantStatus(t, first, http.StatusOK)
		etag := first.Header.Get("ETag")
		fresh := body(t, first)

		resp := getConditional(b, path, etag)
		if resp.StatusCode != http.StatusNotModified {
			t.Errorf("%s with If-None-Match %s = %d, want 304", path, etag, resp.StatusCode)
		}
		if got := body(t, resp); got != "" {
			t.Errorf("%s answered 304 with a %d byte body", path, len(got))
		}

		stale := getConditional(b, path, `"0000000000000000"`)
		wantStatus(t, stale, http.StatusOK)
		if got := body(t, stale); got != fresh {
			t.Errorf("%s served different bytes to a stale validator", path)
		}
	}
}

// TestAnAssetETagTracksItsBytes proves the validator fingerprints content, so
// an edited asset cannot be served under the fingerprint of the old one.
func TestAnAssetETagTracksItsBytes(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	resp := b.get("/assets/app.css")
	wantStatus(t, resp, http.StatusOK)
	etag := resp.Header.Get("ETag")
	sheet := body(t, resp)
	if want := assetFingerprint([]byte(sheet)); etag != want {
		t.Errorf("app.css ETag = %s, want %s derived from the bytes served;\n"+
			"an ETag that is not a digest of the body cannot invalidate on an upgrade",
			etag, want)
	}
}

// TestAnUnknownAssetIs404 keeps a missing or renamed asset from reaching the
// error path as a server fault.
func TestAnUnknownAssetIs404(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	for _, path := range []string{"/assets/app.deadbeef.css", "/assets/nothing.js", "/assets/"} {
		resp := b.get(path)
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, resp.StatusCode)
		}
	}
}

// getConditional fetches an asset presenting a validator the browser already
// holds.
func getConditional(b *browser, path, etag string) *http.Response {
	b.t.Helper()
	req, err := http.NewRequest(http.MethodGet, b.fix.server.URL+path, nil)
	if err != nil {
		b.t.Fatalf("building request: %v", err)
	}
	req.Header.Set("If-None-Match", etag)
	resp, err := b.client.Do(req)
	if err != nil {
		b.t.Fatalf("getting %s: %v", path, err)
	}
	return resp
}

// assetFingerprint recomputes the validator an asset is expected to carry,
// independently of the code under test.
func assetFingerprint(body []byte) string {
	sum := sha256.Sum256(body)
	return `"` + hex.EncodeToString(sum[:16]) + `"`
}

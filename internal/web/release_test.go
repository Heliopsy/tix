// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestNewerRelease pins the comparison, including the cases where staying
// quiet is the right answer. Telling somebody to upgrade on the strength of a
// string nobody parsed is worse than saying nothing.
func TestNewerRelease(t *testing.T) {
	t.Parallel()
	cases := []struct {
		running, latest string
		want            bool
	}{
		{"0.1.0", "0.2.0", true},
		{"0.2.0", "0.2.0", false},
		{"0.2.0", "0.1.0", false},
		{"0.2.0", "1.0.0", true},
		{"1.9.0", "1.10.0", true}, // not a string comparison
		{"0.2.0", "0.2.1", true},
		{"dev", "0.2.0", true}, // any release beats a development build
		{"dev", "", false},     // ...but not to a release nobody fetched
		{"0.2.0", "", false},   // no check has succeeded yet
		{"0.2.0", "not-a-version", false},
		{"not-a-version", "0.2.0", false},
		{"0.2.0-rc.1", "0.2.0", false}, // suffixes are ignored, so these are equal
		// ...which means the version carrying one still has to be readable.
		// Every case above answers false when the suffix is not trimmed, so
		// none of them can tell trimming from refusing to parse at all.
		{"0.2.0-rc.1", "0.3.0", true},
		{"0.2.0+build.7", "0.3.0", true},
		{"0.3.0-rc.1", "0.2.0", false},
	}
	for _, tc := range cases {
		if got := newerRelease(tc.running, tc.latest); got != tc.want {
			t.Errorf("newerRelease(%q, %q) = %v, want %v", tc.running, tc.latest, got, tc.want)
		}
	}
}

// TestReleaseStateNeverBlocks is the property that matters at render time: the
// first call returns immediately with whatever is known, which is nothing.
func TestReleaseStateNeverBlocks(t *testing.T) {
	t.Parallel()
	w := newReleaseWatch()
	got := w.state()
	if got.Running == "" {
		t.Error("state() gave no running version")
	}
	if got.Newer {
		t.Error("state() claimed an upgrade before any check had succeeded")
	}
}

// TestNilReleaseWatchStillRenders covers a handler built without one, since a
// page that panics is worse than a page with no version on it.
func TestNilReleaseWatchStillRenders(t *testing.T) {
	t.Parallel()
	var w *releaseWatch
	if got := w.state(); got.Running == "" {
		t.Error("a nil watch gave no running version")
	}
}

// stubRelease answers the release endpoint without a network, and counts what
// it was asked.
type stubRelease struct {
	calls  atomic.Int32
	status int
	body   string
}

func (s *stubRelease) RoundTrip(*http.Request) (*http.Response, error) {
	s.calls.Add(1)
	return &http.Response{
		StatusCode: s.status,
		Body:       io.NopCloser(strings.NewReader(s.body)),
		Header:     http.Header{},
	}, nil
}

// watchServedBy is a release watch talking to a stub rather than to GitHub.
func watchServedBy(status int, body string) (*releaseWatch, *stubRelease) {
	stub := &stubRelease{status: status, body: body}
	return &releaseWatch{client: &http.Client{Transport: stub}}, stub
}

// The cache decides whether a render triggers a call at all, and the decision
// is made from how long ago the last one was. state() marks the refresh in
// flight before it starts one, so the flag it leaves behind says which way
// the decision went without waiting on the goroutine.
func TestOnlyAStaleReleaseCheckStartsARefresh(t *testing.T) {
	t.Parallel()

	fresh, _ := watchServedBy(http.StatusOK, `{"tag_name":"v9.9.9"}`)
	fresh.checked = time.Now()
	_ = fresh.state()
	fresh.mu.Lock()
	started := fresh.inFlight
	fresh.mu.Unlock()
	if started {
		t.Error("a check made just now was refreshed again")
	}

	stale, _ := watchServedBy(http.StatusOK, `{"tag_name":"v9.9.9"}`)
	stale.checked = time.Now().Add(-2 * releaseCheckEvery)
	_ = stale.state()
	stale.mu.Lock()
	started = stale.inFlight
	stale.mu.Unlock()
	if !started {
		t.Error("a check older than the interval was not refreshed")
	}
}

// A refresh that answers has to be read, and one that does not has to be
// ignored. Both halves matter: the watch fails silent, so the only evidence
// either way is whether the cached tag moved.
func TestARefreshKeepsOnlyASuccessfulAnswer(t *testing.T) {
	t.Parallel()

	ok, served := watchServedBy(http.StatusOK, `{"tag_name":"v9.9.9"}`)
	ok.refresh()
	if served.calls.Load() != 1 {
		t.Fatalf("the refresh made %d calls, want 1", served.calls.Load())
	}
	if ok.latest != "9.9.9" {
		t.Errorf("the refresh kept %q, want the tag it was served", ok.latest)
	}

	refused, _ := watchServedBy(http.StatusInternalServerError, `{"tag_name":"v9.9.9"}`)
	refused.refresh()
	if refused.latest != "" {
		t.Errorf("a refused answer was read as a release: %q", refused.latest)
	}
}

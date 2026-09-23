// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/heliopsy/tix/internal/version"
)

// latestReleaseURL is the GitHub endpoint that names the newest tag. It is the
// public API, so an unauthenticated read is allowed and rate limited by IP.
const latestReleaseURL = "https://api.github.com/repos/heliopsy/tix/releases/latest"

// releaseCheckEvery bounds how often the check runs. A page render must never
// wait on GitHub and a busy instance must not hammer it, so the answer is
// cached and refreshed at most this often. Six hours: new enough that somebody
// learns about a release the day it lands, rare enough that a server nobody
// restarts makes four calls a day.
const releaseCheckEvery = 6 * time.Hour

// releaseCheckTimeout bounds the call itself. It runs off the request path, so
// this only stops a hung connection holding a goroutine indefinitely.
const releaseCheckTimeout = 5 * time.Second

// upgrade is what the footer says about the running version.
type upgrade struct {
	// Running is this binary's version, always set.
	Running string
	// Latest is the newest published release, empty until a check succeeds.
	Latest string
	// Newer reports whether Latest is ahead of Running.
	Newer bool
}

// releaseWatch caches the newest published release.
//
// Three properties, each one a way this could have gone wrong:
//
// It never blocks a render. The page reads whatever is cached, including
// nothing at all on the first request, and a refresh happens behind it.
//
// It fails silent. An instance with no route to the internet, or one behind a
// proxy that refuses, is the normal case for a self-hosted tracker. It must
// not log an error every six hours, and it must not show the operator a
// warning about a check they never asked for.
//
// It asks at most once per interval even under load, because the refresh is
// guarded and a second caller finding one in flight simply returns.
type releaseWatch struct {
	client *http.Client

	mu       sync.Mutex
	latest   string
	checked  time.Time
	inFlight bool
}

func newReleaseWatch() *releaseWatch {
	return &releaseWatch{client: &http.Client{Timeout: releaseCheckTimeout}}
}

// state returns what to show, and starts a refresh when the cache is stale.
func (w *releaseWatch) state() upgrade {
	running := strings.TrimPrefix(version.Version, "v")
	up := upgrade{Running: running}
	if w == nil {
		return up
	}

	w.mu.Lock()
	latest, checked, busy := w.latest, w.checked, w.inFlight
	stale := time.Since(checked) > releaseCheckEvery
	if stale && !busy {
		w.inFlight = true
	}
	w.mu.Unlock()

	if stale && !busy {
		go w.refresh()
	}

	up.Latest = latest
	up.Newer = newerRelease(running, latest)
	return up
}

func (w *releaseWatch) refresh() {
	defer func() {
		w.mu.Lock()
		w.inFlight = false
		w.checked = time.Now()
		w.mu.Unlock()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), releaseCheckTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestReleaseURL, nil)
	if err != nil {
		return
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := w.client.Do(req)
	if err != nil {
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return
	}

	var body struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return
	}
	tag := strings.TrimSpace(strings.TrimPrefix(body.TagName, "v"))
	if tag == "" {
		return
	}
	w.mu.Lock()
	w.latest = tag
	w.mu.Unlock()
}

// newerRelease reports whether latest is ahead of running.
//
// A development build reports "dev" and any published release is ahead of it.
// Anything it cannot read as a version is treated as not newer: telling
// somebody to upgrade on the strength of a string nobody parsed is worse than
// staying quiet.
func newerRelease(running, latest string) bool {
	if latest == "" || running == latest {
		return false
	}
	if running == "dev" || running == "" {
		return true
	}
	a, aok := semver(running)
	b, bok := semver(latest)
	if !aok || !bok {
		return false
	}
	for i := range a {
		if b[i] != a[i] {
			return b[i] > a[i]
		}
	}
	return false
}

// semver reads the leading major.minor.patch of a version, ignoring any
// pre-release or build suffix.
func semver(v string) ([3]int, bool) {
	var out [3]int
	parts := strings.SplitN(v, ".", 3)
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		if i == 2 {
			if cut := strings.IndexAny(p, "-+"); cut >= 0 {
				p = p[:cut]
			}
		}
		n := 0
		if p == "" {
			return out, false
		}
		for _, c := range p {
			if c < '0' || c > '9' {
				return out, false
			}
			n = n*10 + int(c-'0')
		}
		out[i] = n
	}
	return out, true
}

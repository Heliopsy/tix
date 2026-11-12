// SPDX-License-Identifier: AGPL-3.0-or-later

// Package release knows what the newest published tix release is and what
// this binary's own changelog says, for the browser's release screen and the
// API route that draws the same screen elsewhere. One Watch serves a process,
// so the two never disagree and GitHub is asked once.
package release

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/heliopsy/tix/internal/changelog"
	"github.com/heliopsy/tix/internal/version"
)

// latestReleaseURL is the GitHub endpoint that names the newest tag. It is the
// public API, so an unauthenticated read is allowed and rate limited by IP.
const latestReleaseURL = "https://api.github.com/repos/heliopsy/tix/releases/latest"

// checkEvery bounds how often the check runs. A page render must never wait
// on GitHub and a busy instance must not hammer it, so the answer is cached
// and refreshed at most this often. Six hours: new enough that somebody learns
// about a release the day it lands, rare enough that a server nobody restarts
// makes four calls a day.
const checkEvery = 6 * time.Hour

// checkTimeout bounds the call itself. It runs off the request path, so this
// only stops a hung connection holding a goroutine indefinitely.
const checkTimeout = 5 * time.Second

// State is what the check knows about the running version.
type State struct {
	// Running is this binary's version, always set.
	Running string
	// Latest is the newest published release, empty until a check succeeds.
	Latest string
	// Newer reports whether Latest is ahead of Running.
	Newer bool
	// Notes is what the newest release says about itself, with every link
	// taken out. Empty when no check has succeeded, which is also when
	// Newer is false, so nothing offers to show notes that are not there.
	Notes string
	// Checked reports whether a check has finished, whatever it answered.
	Checked bool
	// Failed reports whether the most recent check got no usable answer.
	Failed bool
}

// Status names what the release screen says about this installation: newer,
// current, unknown when the versions cannot be compared, failed when the feed
// could not be read, or pending when nothing has been asked yet.
func (s State) Status() string {
	switch {
	case s.Newer:
		return "newer"
	case s.Latest != "":
		_, rok := semver(s.Running)
		_, lok := semver(s.Latest)
		if s.Running == s.Latest || (rok && lok) {
			return "current"
		}
		return "unknown"
	case s.Failed:
		return "failed"
	default:
		return "pending"
	}
}

// Offer is what a newer release puts in front of the reader.
type Offer struct {
	// Notes are the newest release's notes as Markdown, without the heading
	// that only repeats its version.
	Notes string
	// Methods are the upgrade commands, one per install method.
	Methods []Method
}

// Offer is the newest release's notes and upgrade commands, both empty unless
// it is newer than the running one: when the installation is current the
// feed's notes are the running version's changelog entry a second time.
func (s State) Offer() Offer {
	if !s.Newer {
		return Offer{}
	}
	return Offer{Notes: notesBody(s.Notes, s.Latest), Methods: Methods(s.Latest)}
}

// Watch caches the newest published release.
//
// Three properties, each one a way this could have gone wrong:
//
// It never blocks a render. A reader gets whatever is cached, including
// nothing at all on the first request, and a refresh happens behind it.
//
// It fails silent. An instance with no route to the internet, or one behind a
// proxy that refuses, is the normal case for a self-hosted tracker. It must
// not log an error every six hours, and it must not show the operator a
// warning about a check they never asked for.
//
// It asks at most once per interval even under load, because the refresh is
// guarded and a second caller finding one in flight simply returns.
type Watch struct {
	client *http.Client

	mu       sync.Mutex
	latest   string
	notes    string
	failed   bool
	checked  time.Time
	inFlight bool
}

// NewWatch returns a watch that has not asked yet.
func NewWatch() *Watch {
	return &Watch{client: &http.Client{Timeout: checkTimeout}}
}

// State returns what is known, and starts a refresh when the cache is stale.
// A nil watch knows the running version alone.
func (w *Watch) State() State {
	running := strings.TrimPrefix(version.Version, "v")
	st := State{Running: running}
	if w == nil {
		return st
	}

	w.mu.Lock()
	latest, notes, checked, busy, failed := w.latest, w.notes, w.checked, w.inFlight, w.failed
	stale := time.Since(checked) > checkEvery
	if stale && !busy {
		w.inFlight = true
	}
	w.mu.Unlock()

	if stale && !busy {
		go w.refresh()
	}

	st.Latest = latest
	st.Notes = notes
	st.Newer = newerRelease(running, latest)
	st.Checked = !checked.IsZero()
	st.Failed = failed
	return st
}

func (w *Watch) refresh() {
	answered := false
	defer func() {
		w.mu.Lock()
		w.failed = !answered
		w.inFlight = false
		w.checked = time.Now()
		w.mu.Unlock()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
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
		Body    string `json:"body"`
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
	w.notes = StripLinks(body.Body)
	w.mu.Unlock()
	answered = true
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

// markdownLink matches [text](url), which is how release-please writes every
// reference in a changelog entry: the heading links to a compare view and each
// line ends with a short sha linking to a commit.
var markdownLink = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)

// bareURL matches a scheme or a www. address and everything up to whitespace,
// for a link written without markdown around it, which the Markdown renderer
// would otherwise turn back into one.
var bareURL = regexp.MustCompile(`(?:https?://|www\.)\S+`)

// trailingRef matches the " (sha)" left behind once a commit link has been
// reduced to its text, which is a bare hash saying nothing to somebody reading
// a release note in a browser.
var trailingRef = regexp.MustCompile(`\s*\(\s*[0-9a-f]{7,40}\s*\)`)

// StripLinks turns release notes into text that can be rendered on a page this
// install serves.
//
// Two reasons, and the weaker one is the test. TestPagesReferenceNoExternalOrigin
// greps every screen for "https://" and would fail the moment a changelog
// carrying commit links was rendered. The stronger reason is what that test
// defends: these notes come from a remote server, and a self-hosted tracker
// must not paste whoever controls that server's links into its own pages. The
// text of a link survives, which is the part that says what changed; the
// target does not.
//
// It runs once per refresh rather than once per render, so a reader gets text
// that was already safe when it was cached.
func StripLinks(body string) string {
	out := markdownLink.ReplaceAllString(body, "$1")
	out = bareURL.ReplaceAllString(out, "")
	out = trailingRef.ReplaceAllString(out, "")
	lines := strings.Split(out, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		kept = append(kept, strings.TrimRight(line, " \t"))
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}

// earlierChanges is how many entries before the running one are listed.
const earlierChanges = 10

// Changes is the "What's new" selection from the bundled changelog.
type Changes struct {
	// Running is the running version's own entry, nil unless this build is
	// a tagged release the changelog has an entry for.
	Running *Entry
	// Earlier are the entries listed under it, or the newest entries when
	// Running is nil.
	Earlier []Entry
	// Untagged reports that the running version is not a plain
	// major.minor.patch, so the build is not a tagged release.
	Untagged bool
}

// Entry is one changelog entry with its links stripped.
type Entry struct {
	Version string
	Date    string
	// Notes is the entry's Markdown after StripLinks.
	Notes string
}

// plainRelease matches a version that names a tag and nothing else: not dev,
// not a pre-release, not a build from an edited tree.
var plainRelease = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

// NewChanges picks the running version's entry and those before it. A build
// that is not a tagged release, or one the changelog does not know, gets the
// newest entries and none of them is called the running one: a 0.31.0+dirty
// build is not what the 0.31.0 notes describe.
func NewChanges(running string, entries []changelog.Entry) Changes {
	c := Changes{Untagged: !plainRelease.MatchString(running), Earlier: []Entry{}}
	from := 0
	if !c.Untagged {
		for i, e := range entries {
			if e.Version == running {
				own := newEntry(e)
				c.Running = &own
				from = i + 1
				break
			}
		}
	}
	limit := earlierChanges
	if c.Running == nil {
		limit++
	}
	for _, e := range entries[from:] {
		if len(c.Earlier) == limit {
			break
		}
		c.Earlier = append(c.Earlier, newEntry(e))
	}
	return c
}

// newEntry strips an entry's links by the same policy as the feed notes.
func newEntry(e changelog.Entry) Entry {
	return Entry{Version: e.Version, Date: e.Date, Notes: StripLinks(e.Body)}
}

// Method is one way tix is installed and the command that upgrades it.
type Method struct {
	// ID is binary, container or go.
	ID      string
	Name    string
	Help    string
	Command string
}

// Methods are the install methods docs/upgrading.md and the README describe,
// each with the command that moves it to version.
func Methods(version string) []Method {
	return []Method{
		{ID: "binary", Name: "Release binary",
			Help:    "Installed with install.sh or unpacked from a release archive. Checks the download against its published checksum before replacing the binary.",
			Command: "tix update"},
		{ID: "container", Name: "Container image",
			Help:    "Pull the new tag, then recreate the container on the same volume.",
			Command: "podman pull ghcr.io/heliopsy/tix:" + version},
		{ID: "go", Name: "Go toolchain",
			Help:    "Installed with go install. tix update refuses this copy, because the toolchain owns it.",
			Command: "go install github.com/heliopsy/tix@v" + version},
	}
}

// releaseHeading matches a first line that is a Markdown heading.
var releaseHeading = regexp.MustCompile(`^#{1,6}\s+(.*)$`)

// notesBody drops the heading release-please opens every changelog with,
// which names the release the section heading above it already names. Any
// other first line is the author's and is kept.
func notesBody(notes, latest string) string {
	first, rest, _ := strings.Cut(notes, "\n")
	m := releaseHeading.FindStringSubmatch(strings.TrimSpace(first))
	if latest == "" || m == nil || !strings.Contains(m[1], latest) {
		return notes
	}
	return strings.TrimSpace(rest)
}

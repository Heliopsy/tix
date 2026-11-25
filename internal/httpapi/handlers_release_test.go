// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/changelog"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/release"
	"github.com/heliopsy/tix/internal/wire"
)

// fixedRelease is a release check that already holds an answer.
type fixedRelease release.State

func (f fixedRelease) State() release.State { return release.State(f) }

// feedNotes is a release-please body after StripLinks, the only shape the
// watch ever caches.
const feedNotes = "## 0.30.0 (2026-11-01)\n\n### Features\n\n* **web:** task screen redesign"

// releaseWire is the response read by its JSON names, so a renamed field
// fails here rather than in a client.
type releaseWire struct {
	Running string  `json:"running"`
	Latest  string  `json:"latest"`
	Status  string  `json:"status"`
	Newer   *bool   `json:"newer"`
	Checked *bool   `json:"checked"`
	Failed  *bool   `json:"failed"`
	Notes   *string `json:"notes"`
	Methods []struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Help    string `json:"help"`
		Command string `json:"command"`
	} `json:"methods"`
	Changes *struct {
		Running  *entryWire  `json:"running"`
		Earlier  []entryWire `json:"earlier"`
		Untagged *bool       `json:"untagged"`
	} `json:"changes"`
}

// entryWire is one changelog entry by its JSON names.
type entryWire struct {
	Version string `json:"version"`
	Date    string `json:"date"`
	Notes   string `json:"notes"`
}

// getRelease reads the route as tenant A with src as the release check.
func getRelease(t *testing.T, src httpapi.ReleaseSource) releaseWire {
	t.Helper()
	f := newFixtureWith(t, func(_ *apiFixture, cfg *httpapi.Config) { cfg.Releases = src })
	resp := f.call(http.MethodGet, wire.RouteRelease, nil)
	mustStatus(t, resp, http.StatusOK)
	var got releaseWire
	decodeBody(t, resp, &got)
	if got.Newer == nil || got.Checked == nil || got.Failed == nil || got.Notes == nil || got.Changes == nil ||
		got.Changes.Untagged == nil || got.Methods == nil || got.Changes.Earlier == nil {
		t.Fatalf("the response lacks a field or carries null for a list: %+v", got)
	}
	return got
}

// Each state the check can be in reaches the client as the release screen
// names it, and only a newer release brings notes and upgrade commands.
func TestReleaseRouteReportsTheCheck(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		state   release.State
		status  string
		methods bool
	}{
		{"newer", release.State{Running: "0.28.0", Latest: "0.30.0", Newer: true, Checked: true, Notes: feedNotes}, "newer", true},
		{"current", release.State{Running: "0.30.0", Latest: "0.30.0", Checked: true, Notes: feedNotes}, "current", false},
		{"unknown", release.State{Running: "my-fork", Latest: "0.30.0", Checked: true, Notes: feedNotes}, "unknown", false},
		{"failed", release.State{Running: "0.28.0", Checked: true, Failed: true}, "failed", false},
		{"pending", release.State{Running: "0.28.0"}, "pending", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := getRelease(t, fixedRelease(tc.state))
			if got.Status != tc.status || got.Running != tc.state.Running || got.Latest != tc.state.Latest ||
				*got.Newer != tc.state.Newer || *got.Checked != tc.state.Checked || *got.Failed != tc.state.Failed {
				t.Errorf("got %+v, want status %q over %+v", got, tc.status, tc.state)
			}
			if !tc.methods {
				if len(got.Methods) != 0 || *got.Notes != "" {
					t.Errorf("state %q offers an upgrade: methods %+v, notes %q", tc.status, got.Methods, *got.Notes)
				}
				return
			}
			if *got.Notes != "### Features\n\n* **web:** task screen redesign" {
				t.Errorf("notes = %q, want the Markdown without the release heading", *got.Notes)
			}
			want := map[string]string{
				"binary":    "tix update",
				"container": "podman pull ghcr.io/heliopsy/tix:0.30.0",
				"go":        "go install github.com/heliopsy/tix@v0.30.0",
			}
			var ids []string
			for _, m := range got.Methods {
				ids = append(ids, m.ID)
				if m.Command != want[m.ID] || m.Name == "" || m.Help == "" {
					t.Errorf("method %+v, want command %q with a name and help", m, want[m.ID])
				}
			}
			if !slices.Equal(ids, []string{"binary", "container", "go"}) {
				t.Errorf("method ids = %v, want binary, container, go", ids)
			}
		})
	}
}

// releaseLog is the bundled changelog the route reads in these tests: package
// main assigns the real one, so a test binary has none. Thirteen releases,
// 0.13.0 newest, each linking a commit the route must not pass on.
func init() {
	var b strings.Builder
	b.WriteString("# Changelog\n")
	for i := 13; i > 0; i-- {
		fmt.Fprintf(&b, "\n## [0.%d.0](https://github.com/Heliopsy/tix/compare/v0.%d.0...v0.%d.0) (2026-11-%02d)\n\n"+
			"* **web:** change %d ([abc1234](https://github.com/Heliopsy/tix/commit/abc1234))\n", i, i-1, i, i, i)
	}
	changelog.Source = b.String()
}

// The changelog selection is the release screen's: a tagged build gets its
// own entry and up to ten before it, an untagged build the eleven newest and
// no running one, every note with its links taken out.
func TestReleaseRouteCarriesTheBundledChangelog(t *testing.T) {
	t.Parallel()
	tagged := getRelease(t, fixedRelease{Running: "0.12.0"})
	if tagged.Changes.Running == nil || tagged.Changes.Running.Version != "0.12.0" ||
		tagged.Changes.Running.Date != "2026-11-12" || *tagged.Changes.Untagged {
		t.Fatalf("a tagged build's changes are %+v, want 0.12.0 as the running entry", tagged.Changes)
	}
	if got := versions(tagged); !slices.Equal(got, []string{"0.11.0", "0.10.0", "0.9.0", "0.8.0", "0.7.0",
		"0.6.0", "0.5.0", "0.4.0", "0.3.0", "0.2.0"}) {
		t.Errorf("a tagged build lists %v, want the ten releases under 0.12.0", got)
	}

	dev := getRelease(t, fixedRelease{Running: "dev"})
	if dev.Changes.Running != nil || !*dev.Changes.Untagged {
		t.Errorf("an untagged build's changes are %+v, want no running entry and untagged", dev.Changes)
	}
	if got := versions(dev); len(got) != 11 || got[0] != "0.13.0" || got[10] != "0.3.0" {
		t.Errorf("an untagged build lists %v, want the eleven newest", got)
	}
	for _, e := range append(dev.Changes.Earlier, *tagged.Changes.Running) {
		if !strings.Contains(e.Notes, "**web:** change") || strings.Contains(e.Notes, "https://") ||
			strings.Contains(e.Notes, "](") || strings.Contains(e.Notes, "abc1234") {
			t.Errorf("entry %s is not its Markdown with the links taken out:\n%s", e.Version, e.Notes)
		}
	}
}

// versions are the earlier entries' versions in order.
func versions(r releaseWire) []string {
	var out []string
	for _, e := range r.Changes.Earlier {
		out = append(out, e.Version)
	}
	return out
}

// Without a check configured the route still names the running version.
func TestReleaseRouteWithoutAWatch(t *testing.T) {
	t.Parallel()
	got := getRelease(t, nil)
	if got.Running == "" || got.Status != "pending" || len(got.Methods) != 0 {
		t.Errorf("got %+v, want the running version alone, pending", got)
	}
}

// The route answers a signed-in caller only.
func TestReleaseRouteNeedsAuthentication(t *testing.T) {
	t.Parallel()
	f := newFixtureWith(t, func(_ *apiFixture, cfg *httpapi.Config) {
		cfg.Releases = fixedRelease{Running: "0.28.0", Latest: "0.30.0", Newer: true, Checked: true}
	})
	resp := f.do(http.MethodGet, wire.RouteRelease, f.hostA, "", nil)
	body := readBody(t, resp)
	if resp.StatusCode != http.StatusUnauthorized || strings.Contains(body, "0.30.0") {
		t.Errorf("an anonymous read answered %d: %s", resp.StatusCode, body)
	}
}

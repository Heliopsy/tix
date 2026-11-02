// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"strings"
	"testing"
)

// releaseNotes is a release-please body after stripLinks, which is the only
// shape the page is ever handed.
const releaseNotes = "## 0.30.0 (2026-11-01)\n\n### Features\n\n" +
	"* **web:** task screen redesign\n* **cli:** `tix update --check` reports the newest release\n\n" +
	"### Bug Fixes\n\n* **store:** stop the planner scanning every task"

// renderRelease draws the release screen's content block for one state.
func renderRelease(t *testing.T, up upgrade) string {
	t.Helper()
	tmpl := Handler(nil).(*handler).templates["release.html"]
	var out strings.Builder
	if err := tmpl.ExecuteTemplate(&out, "content", view{Upgrade: up, Data: newReleaseView(up)}); err != nil {
		t.Fatalf("rendering the release screen: %v", err)
	}
	return out.String()
}

// element is the markup of the first element opened by open, up to the
// close tag that ends it, so an assertion reads that element and not the
// page around it. Elements of the same name nested inside are counted.
func element(t *testing.T, page, open string) string {
	t.Helper()
	start := strings.Index(page, open)
	if start < 0 {
		t.Fatalf("no %q on the page:\n%s", open, page)
	}
	name := strings.Fields(strings.TrimPrefix(open, "<"))[0]
	depth, i := 0, start
	for i < len(page) {
		nextOpen := strings.Index(page[i+1:], "<"+name)
		nextClose := strings.Index(page[i+1:], "</"+name+">")
		if nextClose < 0 {
			break
		}
		if nextOpen >= 0 && nextOpen < nextClose {
			depth++
			i += 1 + nextOpen
			continue
		}
		i += 1 + nextClose
		if depth == 0 {
			return page[start : i+len("</"+name+">")]
		}
		depth--
	}
	t.Fatalf("%q is never closed:\n%s", open, page[start:])
	return ""
}

// The one card says which of the four answers the check has, and each answer
// is its own: an install that could not reach the feed is not told it is
// current, and one that has not asked yet is not told the ask failed.
func TestReleaseStatusCardNamesTheState(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		up       upgrade
		status   string
		headline string
		newest   string
	}{
		{"an upgrade", upgrade{Running: "0.28.0", Latest: "0.30.0", Newer: true, Checked: true}, "newer", "v0.30.0 is available", "v0.30.0"},
		{"current", upgrade{Running: "0.30.0", Latest: "0.30.0", Checked: true}, "current", "This installation is up to date", "v0.30.0"},
		{"the check failed", upgrade{Running: "0.28.0", Checked: true, Failed: true}, "failed", "The release feed could not be reached", "not known"},
		{"not asked yet", upgrade{Running: "0.28.0"}, "pending", "Not checked yet", "not known"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.up.Status(); got != tc.status {
				t.Fatalf("Status() = %q, want %q", got, tc.status)
			}
			card := element(t, renderRelease(t, tc.up), `<section class="relstatus`)
			if !strings.Contains(card, `data-status="`+tc.status+`"`) {
				t.Errorf("the card does not carry status %q:\n%s", tc.status, card)
			}
			if !strings.Contains(card, tc.headline) {
				t.Errorf("the card does not say %q:\n%s", tc.headline, card)
			}
			versions := element(t, card, `<dl class="relversions"`)
			if !strings.Contains(versions, "v"+tc.up.Running) || !strings.Contains(versions, tc.newest) {
				t.Errorf("the card's versions do not read v%s and %s:\n%s", tc.up.Running, tc.newest, versions)
			}
		})
	}
}

// A version this build cannot compare is neither current nor behind, so the
// page says it does not know rather than calling it up to date.
func TestAnIncomparableVersionIsNotCalledCurrent(t *testing.T) {
	t.Parallel()
	up := upgrade{Running: "my-fork", Latest: "0.30.0", Checked: true}
	if got := up.Status(); got != "unknown" {
		t.Fatalf("Status() = %q, want unknown", got)
	}
}

// The notes are Markdown and are drawn as Markdown, through the sanitising
// renderer the task screen uses, in their own column. The release heading
// release-please opens with repeats the section's own title and is dropped.
func TestReleaseNotesRenderAsMarkdown(t *testing.T) {
	t.Parallel()
	up := upgrade{Running: "0.28.0", Latest: "0.30.0", Newer: true, Checked: true, Notes: releaseNotes}
	page := renderRelease(t, up)
	if strings.Contains(page, "<pre") {
		t.Errorf("the notes are still preformatted text:\n%s", page)
	}
	notes := element(t, page, `<div class="md relnotes"`)
	for _, want := range []string{"<strong>web:</strong>", "<li>", "<code>tix update --check</code>", ">Bug Fixes</h5>"} {
		if !strings.Contains(notes, want) {
			t.Errorf("the rendered notes lack %q:\n%s", want, notes)
		}
	}
	for _, gone := range []string{"**", "###", "0.30.0 (2026-11-01)"} {
		if strings.Contains(notes, gone) {
			t.Errorf("the rendered notes still carry %q:\n%s", gone, notes)
		}
	}
	section := element(t, page, `<section class="relsec relnotes-sec"`)
	if !strings.Contains(section, "What changed in v0.30.0") {
		t.Errorf("the notes section does not name the release:\n%s", section)
	}
}

// Notes written by hand are not touched: only a leading heading naming the
// release is redundant.
func TestOnlyTheLeadingReleaseHeadingIsDropped(t *testing.T) {
	t.Parallel()
	cases := []struct{ notes, latest, want string }{
		{"## 0.30.0 (2026-11-01)\n\n### Features", "0.30.0", "### Features"},
		{"## [0.30.0] notes\n\nbody", "0.30.0", "body"},
		{"## Highlights\n\nbody", "0.30.0", "## Highlights\n\nbody"},
		{"plain text about 0.30.0", "0.30.0", "plain text about 0.30.0"},
	}
	for _, tc := range cases {
		if got := notesBody(tc.notes, tc.latest); got != tc.want {
			t.Errorf("notesBody(%q) = %q, want %q", tc.notes, got, tc.want)
		}
	}
}

// Every install method gets its own command, carrying the version on offer,
// with a copy button pointed at that command and no other. The commands are
// shown only when there is something to upgrade to.
func TestUpgradeCommandsAreCopyablePerMethod(t *testing.T) {
	t.Parallel()
	newer := upgrade{Running: "0.28.0", Latest: "0.30.0", Newer: true, Checked: true}
	section := element(t, renderRelease(t, newer), `<section class="relsec relupgrade"`)
	for id, cmd := range map[string]string{
		"upgrade-binary":    "tix update",
		"upgrade-container": "podman pull ghcr.io/heliopsy/tix:0.30.0",
		"upgrade-go":        "go install github.com/heliopsy/tix@v0.30.0",
	} {
		code := element(t, section, `<code id="`+id+`"`)
		if !strings.Contains(code, ">"+cmd+"</code>") {
			t.Errorf("%s reads %q, want %q", id, code, cmd)
		}
		if !strings.Contains(section, `data-copy-target="`+id+`"`) {
			t.Errorf("no copy button for %s:\n%s", id, section)
		}
	}

	for _, up := range []upgrade{
		{Running: "0.30.0", Latest: "0.30.0", Checked: true},
		{Running: "0.28.0", Checked: true, Failed: true},
	} {
		if page := renderRelease(t, up); strings.Contains(page, "relupgrade") || strings.Contains(page, "data-copy-target") {
			t.Errorf("state %q offers upgrade commands:\n%s", up.Status(), page)
		}
	}
}

// A refresh that fails says so, and one that answers clears it, so the page
// can tell "could not reach the feed" from "has not asked yet".
func TestARefreshRecordsWhetherItFailed(t *testing.T) {
	t.Parallel()
	refused, _ := watchServedBy(http.StatusInternalServerError, `{}`)
	refused.refresh()
	if got := refused.state(); !got.Checked || !got.Failed {
		t.Errorf("a refused refresh reads Checked=%v Failed=%v, want both", got.Checked, got.Failed)
	}

	ok, _ := watchServedBy(http.StatusOK, `{"tag_name":"v9.9.9"}`)
	ok.failed = true
	ok.refresh()
	if got := ok.state(); !got.Checked || got.Failed {
		t.Errorf("a refresh that answered reads Checked=%v Failed=%v, want checked and not failed", got.Checked, got.Failed)
	}
}

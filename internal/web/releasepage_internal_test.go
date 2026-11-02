// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"fmt"
	"html"
	"net/http"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/changelog"
)

// releaseNotes is a release-please body after stripLinks, which is the only
// shape the page is ever handed.
const releaseNotes = "## 0.30.0 (2026-11-01)\n\n### Features\n\n" +
	"* **web:** task screen redesign\n* **cli:** `tix update --check` reports the newest release\n\n" +
	"### Bug Fixes\n\n* **store:** stop the planner scanning every task"

// renderRelease draws the release screen's content block for one state.
func renderRelease(t *testing.T, up upgrade) string {
	t.Helper()
	return renderReleaseWith(t, up, nil)
}

// renderReleaseWith draws it with a bundled changelog.
func renderReleaseWith(t *testing.T, up upgrade, log []changelog.Entry) string {
	t.Helper()
	tmpl := Handler(nil).(*handler).templates["release.html"]
	var out strings.Builder
	if err := tmpl.ExecuteTemplate(&out, "content", view{Upgrade: up, Data: newReleaseView(up, log)}); err != nil {
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

// changelogFixture is three release-please entries, the newest carrying a
// link, a commit hash and raw HTML the page must not pass through.
var changelogFixture = changelog.Parse(`# Changelog

## [0.31.0](https://github.com/Heliopsy/tix/compare/v0.30.0...v0.31.0) (2026-11-02)


### Features

* **web:** redesign the release screen ([#149](https://github.com/Heliopsy/tix/issues/149)) ([768a287](https://github.com/Heliopsy/tix/commit/768a287a0e79d5c61259e927c397304e745d6d8f))
* <script>alert(1)</script> see www.example.com

## [0.30.0](https://github.com/Heliopsy/tix/compare/v0.29.1...v0.30.0) (2026-11-01)


### Bug Fixes

* **web:** fold sidebar groups

## 0.29.1 (2026-11-01)


### Bug Fixes

* stop tix ssh hanging
`)

// The running version's entry comes first and is the one marked as running
// here; the releases before it are folded under it.
func TestTheRunningVersionsEntryIsHighlighted(t *testing.T) {
	t.Parallel()
	page := renderReleaseWith(t, upgrade{Running: "0.30.0", Latest: "0.31.0", Newer: true, Checked: true}, changelogFixture)
	section := element(t, page, `<section class="relsec relchanges"`)
	running := element(t, section, `<article class="relentry relentry-running"`)
	if !strings.Contains(running, "v0.30.0") || !strings.Contains(running, "You are running this") || !strings.Contains(running, "fold sidebar groups") {
		t.Errorf("the highlighted entry is not 0.30.0 marked as running:\n%s", running)
	}
	if strings.Contains(running, "0.31.0") || strings.Contains(running, "0.29.1") {
		t.Errorf("the highlighted entry carries another release:\n%s", running)
	}
	earlier := element(t, section, `<div class="relentries"`)
	if !strings.Contains(earlier, "<details") || !strings.Contains(earlier, "v0.29.1") || strings.Contains(earlier, "0.31.0") || strings.Contains(earlier, "0.30.0") {
		t.Errorf("the earlier entries are not 0.29.1 alone, folded:\n%s", earlier)
	}
	if strings.Contains(earlier, " open") {
		t.Errorf("an earlier entry starts unfolded:\n%s", earlier)
	}
	if strings.Index(page, `class="relstatus"`) > strings.Index(page, `id="changes"`) {
		t.Error("the update card is not above the changes")
	}
}

// A build that is not a tagged release lists the newest entries and claims
// none of them, including one whose numbers match a release.
func TestAnUntaggedBuildListsTheNewestEntries(t *testing.T) {
	t.Parallel()
	for _, running := range []string{"dev", "0.31.0+dirty", "0.31.0-rc.1"} {
		t.Run(running, func(t *testing.T) {
			t.Parallel()
			section := element(t, renderReleaseWith(t, upgrade{Running: running}, changelogFixture), `<section class="relsec relchanges"`)
			if strings.Contains(section, "relentry-running") || strings.Contains(section, "You are running this") {
				t.Errorf("an untagged build marks an entry as running:\n%s", section)
			}
			if !strings.Contains(html.UnescapeString(section), "v"+running+" is not a tagged release") {
				t.Errorf("the section does not say v%s is not a tagged release:\n%s", running, section)
			}
			first := element(t, section, `<details class="relentry"`)
			if !strings.Contains(first, " open>") || !strings.Contains(first, "v0.31.0") {
				t.Errorf("the newest entry is not first and unfolded:\n%s", first)
			}
		})
	}
}

// Entries go through stripLinks and the sanitising renderer: link text stays,
// no target, no commit hash and no raw HTML reach the page.
func TestChangelogEntriesAreSanitised(t *testing.T) {
	t.Parallel()
	section := element(t, renderReleaseWith(t, upgrade{Running: "0.31.0"}, changelogFixture), `<section class="relsec relchanges"`)
	running := element(t, section, `<article class="relentry relentry-running"`)
	if !strings.Contains(running, "<strong>web:</strong>") || !strings.Contains(running, "(#149)") {
		t.Errorf("the entry lost its Markdown or its link text:\n%s", running)
	}
	for _, gone := range []string{"https://", "www.example.com", "768a287", "<script", "href="} {
		if strings.Contains(running, gone) {
			t.Errorf("the entry still carries %q:\n%s", gone, running)
		}
	}
}

// Only ten earlier entries are listed, however long the changelog is.
func TestEarlierEntriesAreBounded(t *testing.T) {
	t.Parallel()
	var log []changelog.Entry
	for i := 40; i > 0; i-- {
		log = append(log, changelog.Entry{Version: fmt.Sprintf("0.%d.0", i), Body: "* x"})
	}
	if got := newChangesView("0.40.0", log); got.Running == nil || len(got.Earlier) != earlierChanges {
		t.Errorf("a tagged build lists %d earlier entries, want %d", len(got.Earlier), earlierChanges)
	}
}

// When the installation is current the feed's notes are the running entry a
// second time, so they are left out.
func TestFeedNotesOnlyForANewerRelease(t *testing.T) {
	t.Parallel()
	page := renderReleaseWith(t, upgrade{Running: "0.31.0", Latest: "0.31.0", Checked: true, Notes: releaseNotes}, changelogFixture)
	if strings.Contains(page, "relnotes-sec") {
		t.Errorf("a current installation repeats the feed notes:\n%s", page)
	}
}

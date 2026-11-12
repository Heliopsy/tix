// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// wayBackMarks are the ways back a screen state draws: a form's Cancel, a
// dialog's Cancel, a result's way out, and the error screen's links.
var wayBackMarks = []string{"data-form-cancel", "data-dlg-close", "data-way-back"}

var anchorTag = regexp.MustCompile(`<a\b[^>]*>`)
var hrefAttr = regexp.MustCompile(`href="([^"]*)"`)

// waysBack returns the hrefs of the ways back inside block: links carrying one
// of wayBackMarks, plus every link in a back link paragraph and, when block is
// the error screen, in its actions. Any other actions row is not counted,
// because it can hold links that lead onwards rather than back.
func waysBack(block string) []string {
	var out []string
	add := func(tag string) {
		if m := hrefAttr.FindStringSubmatch(tag); m != nil {
			out = append(out, html.UnescapeString(m[1]))
		}
	}
	for _, tag := range anchorTag.FindAllString(block, -1) {
		for _, mark := range wayBackMarks {
			if strings.Contains(tag, mark) {
				add(tag)
				break
			}
		}
	}
	wraps := map[string]string{`<p class="backlink">`: "</p>"}
	if strings.HasPrefix(block, `<section class="failure">`) {
		wraps[`<div class="actions">`] = "</div>"
	}
	for wrap, closer := range wraps {
		for rest := block; ; {
			i := strings.Index(rest, wrap)
			if i < 0 {
				break
			}
			rest = rest[i+len(wrap):]
			end := strings.Index(rest, closer)
			if end < 0 {
				break
			}
			for _, tag := range anchorTag.FindAllString(rest[:end], -1) {
				add(tag)
			}
		}
	}
	return out
}

// stateBlock is the part of the page that is the state under test, from start
// to the first end after it, so a Cancel in a closed dialog elsewhere on the
// page cannot stand in for the way back the state itself is missing.
func stateBlock(t *testing.T, page, start, end string) string {
	t.Helper()
	i := strings.Index(page, start)
	if i < 0 {
		t.Fatalf("the state has no %s:\n%s", start, page)
	}
	j := strings.Index(page[i:], end)
	if j < 0 {
		t.Fatalf("%s is never closed by %s", start, end)
	}
	return page[i : i+j+len(end)]
}

// Every state a multi-step screen can be in, results, previews and refusals
// included, draws a way back, and that way back leads to a screen. A sync run
// refused for its mapping file used to end on the error screen, whose only
// way back pointed at /sync/run, which has no screen.
func TestEveryScreenStateHasAWayBack(t *testing.T) {
	t.Parallel()
	type state struct {
		status     int
		start, end string
		// replace is the address the page asks htmx to show instead of the
		// action it was posted to, when it has to.
		replace string
	}
	cases := map[string]struct {
		walk func(t *testing.T, b *browser) *http.Response
		want state
	}{
		"sync run refused for its mapping file": {
			walk: func(t *testing.T, b *browser) *http.Response {
				saveSource(t, b, url.Values{"system": {"generic"}, "name": {"ops"}, "mapping_path": {"missing.yaml"}})
				return b.postFrom("/sync", "/sync/run", url.Values{"source_id": {"src-ops"}, "dry_run": {"1"}})
			},
			want: state{http.StatusBadRequest, `<section class="runresult"`, `</section>`, "/sync"},
		},
		"sync dry run result": {
			walk: func(t *testing.T, b *browser) *http.Response {
				addSource(t, b, "generic", "ops")
				return b.postFrom("/sync", "/sync/run", url.Values{"source_id": {"src-ops"}, "dry_run": {"1"}})
			},
			want: state{http.StatusOK, `<section class="runresult"`, `</section>`, "/sync"},
		},
		"sync run result": {
			walk: func(t *testing.T, b *browser) *http.Response {
				addSource(t, b, "generic", "ops")
				return b.postFrom("/sync", "/sync/run", url.Values{"source_id": {"src-ops"}})
			},
			want: state{http.StatusOK, `<section class="runresult"`, `</section>`, "/sync"},
		},
		"sync source refused": {
			walk: func(t *testing.T, b *browser) *http.Response {
				b.page("/sync")
				return b.postFrom("/sync", "/sync/sources", url.Values{"system": {"generic"}, "name": {"ops"}, "url": {"https://x.example/"}})
			},
			want: state{http.StatusBadRequest, `<dialog id="source-new"`, `</dialog>`, "/sync?dialog=source-new"},
		},
		"error after an action with no screen": {
			walk: func(t *testing.T, b *browser) *http.Response {
				b.page("/sync")
				return b.postFrom("/sync", "/sync/sources/delete", url.Values{"id": {"nope"}})
			},
			want: state{http.StatusNotFound, `<section class="failure">`, `</section>`, ""},
		},
		"project delete": {
			walk: func(t *testing.T, b *browser) *http.Response {
				return b.get("/projects/infra/delete")
			},
			want: state{http.StatusOK, `<div class="delsheet">`, `</form>`, ""},
		},
		"project delete mapping preview": {
			walk: func(t *testing.T, b *browser) *http.Response {
				return b.get("/projects/infra/delete?choice=move&move_to=ops")
			},
			want: state{http.StatusOK, `<div class="delsheet">`, `</form>`, ""},
		},
		"live events filter naming no project": {
			walk: func(t *testing.T, b *browser) *http.Response {
				return b.get("/events?project=nope&type=task.*")
			},
			want: state{http.StatusOK, `<p class="flash error liveevents-problem"`, `</p>`, ""},
		},
		"project delete refused": {
			walk: func(t *testing.T, b *browser) *http.Response {
				return b.post("/projects/infra/delete", url.Values{"intent": {"delete"}, "choice": {"delete"}, "confirm": {"ops"}})
			},
			want: state{http.StatusBadRequest, `<div class="delsheet">`, `</form>`, ""},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, b := deletionFixture(t)
			resp := tc.walk(t, b)
			page := body(t, resp)
			wantStatus(t, resp, tc.want.status)
			if got := resp.Header.Get("HX-Replace-Url"); got != tc.want.replace {
				t.Errorf("the state asks htmx to show %q, want %q", got, tc.want.replace)
			}
			block := stateBlock(t, page, tc.want.start, tc.want.end)
			if strings.HasPrefix(tc.want.start, `<dialog`) && !strings.Contains(stateBlock(t, block, "<dialog", ">"), " open") {
				t.Fatalf("the refused dialog is not drawn open:\n%s", block)
			}
			if strings.HasPrefix(tc.want.start, `<section class="runresult"`) && !strings.Contains(stateBlock(t, block, "<section", ">"), "data-scroll-here") {
				t.Errorf("the run's answer does not ask to be scrolled into view, so a phone keeps it below the fold")
			}
			ways := waysBack(block)
			if len(ways) == 0 {
				t.Fatalf("the state draws no way back:\n%s", block)
			}
			for _, href := range ways {
				back := b.get(href)
				_ = back.Body.Close()
				if back.StatusCode != http.StatusOK {
					t.Errorf("the way back %q answers %d, not a screen", href, back.StatusCode)
				}
			}
		})
	}
}

// saveSource saves a source with the fields given, as the source dialog does.
func saveSource(t *testing.T, b *browser, form url.Values) {
	t.Helper()
	b.page("/sync")
	resp := b.post("/sync/sources", form)
	defer func() { _ = resp.Body.Close() }()
	wantStatus(t, resp, http.StatusSeeOther)
}

// A refused deletion keeps the choice the reader made, so the way back into
// the form is not a way back to an empty one.
func TestARefusedDeletionKeepsTheChoice(t *testing.T) {
	t.Parallel()
	_, b := deletionFixture(t)
	resp := b.post("/projects/infra/delete", url.Values{"intent": {"delete"}, "choice": {"delete"}, "confirm": {"ops"}})
	page := body(t, resp)
	if !strings.Contains(page, `value="delete" checked`) {
		t.Errorf("the refused deletion forgot the choice to delete the tasks:\n%s", page)
	}
}

// The mapping preview describes the project the picker shows. A target named
// in another case than its key kept that spelling, so the picker selected
// nothing and showed the first project while the table described another,
// and Delete would have moved the tasks to the one shown.
func TestTheMappingPreviewMatchesThePickedTarget(t *testing.T) {
	t.Parallel()
	_, b := deletionFixture(t)
	page := b.page("/projects/infra/delete?choice=move&move_to=OPS")
	if !strings.Contains(page, `<option value="ops" selected>`) {
		t.Errorf("the picker does not show the target the mapping describes:\n%s", stateBlock(t, page, `<select id="move_to"`, `</select>`))
	}
	if !strings.Contains(page, "<caption>Statuses in ops</caption>") {
		t.Errorf("the mapping does not describe the picked target:\n%s", stateBlock(t, page, `<table class="delmap">`, `</caption>`))
	}
}

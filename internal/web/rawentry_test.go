// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// Raw on an activity row was a <details> that the row's flex line sized to
// the word "Raw", so the entry it opened was read down a column a few
// letters wide. It is a link to the screen with a wide dialog drawn open, and
// the task's History opens the same dialog in place of its popover.

var rawOpenerTag = regexp.MustCompile(`<a class="btn link small rawopen" href="([^"]+)" data-dlg-open="(raw-(\d+))" aria-haspopup="dialog" hx-get="([^"]+)" hx-select="\.rawwrap" hx-target="next \.rawslot"`)

// hostileTitle carries markup, quotes and one long unbroken word.
var hostileTitle = `<script>alert("x")</script> 'quoted' ` + strings.Repeat("unbroken", 40)

// rawFixture is a task created with a hostile title and then retitled, so
// its newest entry has a before and an after that differ.
func rawFixture(t *testing.T) (*browser, string) {
	t.Helper()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", hostileTitle)
	saved := b.post("/tasks/"+ref, url.Values{"title": {"plain again"}, "priority": {"1"}, "version": {"1"}})
	_ = saved.Body.Close()
	wantStatus(t, saved, http.StatusSeeOther)
	return b, ref
}

// firstRawOpener is the first Raw control on a page: its link, dialog id and
// the address it fetches.
func firstRawOpener(t *testing.T, page string) (href, id, get string) {
	t.Helper()
	m := rawOpenerTag.FindStringSubmatch(page)
	if m == nil {
		t.Fatalf("the page has no Raw control opening a dialog")
	}
	return m[1], m[2], m[4]
}

func TestTheRawControlOpensADialogNotAFold(t *testing.T) {
	t.Parallel()
	b, ref := rawFixture(t)
	for _, tc := range []struct{ name, path, here string }{
		{"activity", "/activity", "/activity"},
		{"task history", "/tasks/" + ref, "/tasks/" + ref},
	} {
		page := b.page(tc.path)
		for _, gone := range []string{`<details class="panel raw`, `class="flowmenu rawpop"`, `popovertarget="raw-`} {
			if strings.Contains(page, gone) {
				t.Errorf("%s: the Raw control still opens %s", tc.name, gone)
			}
		}
		href, id, get := firstRawOpener(t, page)
		want := tc.here + "?dialog=" + id + "#" + id
		if href != want || get != want {
			t.Errorf("%s: Raw links to %q and fetches %q, want %q", tc.name, href, get, want)
		}
		if !strings.Contains(page, `<div class="rawslot">`) {
			t.Errorf("%s: there is no slot for the fetched dialog", tc.name)
		}
		if dialogBlock(page, id) != "" {
			t.Errorf("%s: the dialog is drawn before it is asked for", tc.name)
		}
	}
}

func TestARawDialogIsAPageWithAWayBack(t *testing.T) {
	t.Parallel()
	b, ref := rawFixture(t)
	for _, tc := range []struct{ name, path string }{
		{"activity", "/activity"},
		{"task history", "/tasks/" + ref},
	} {
		_, id, _ := firstRawOpener(t, b.page(tc.path))
		page := b.page(tc.path + "?dialog=" + id)
		tag := dialogTag(t, page, id)
		if !strings.Contains(tag, "actiondlg wide rawdlg is-inplace") || !strings.HasSuffix(tag, " open") || !strings.Contains(tag, "data-dlg") {
			t.Errorf("%s: the dialog is not drawn open in place: %s", tc.name, tag)
		}
		dlg := dialogBlock(page, id)
		for _, want := range []string{
			`<h2 id="` + id + `-title">Raw entry</h2>`,
			`<a class="actiondlg-x" href="` + tc.path + `" data-dlg-close aria-label="Close"`,
			`<a class="btn" href="` + tc.path + `" data-dlg-close>Close</a>`,
			`<span class="mono">task.update</span>`,
			`<dt>Actor</dt>`, `<dt>Action</dt>`, `<dt>Subject</dt>`, `<dt>Time</dt>`, `<dt>Source</dt>`, `<dt>Sequence</dt>`,
			`data-copy-target="` + id + `-1-before"`, `data-copy-target="` + id + `-1-after"`,
			`>Copy all as JSON</button>`,
		} {
			if !strings.Contains(dlg, want) {
				t.Errorf("%s: the dialog lacks %s:\n%s", tc.name, want, dlg)
			}
		}
		if tc.name == "task history" && !strings.Contains(page, `id="history" open>`) {
			t.Errorf("the History fold holding the dialog is closed")
		}
	}
}

func TestARawDialogPrintsTheSnapshotsEscapedAndMarksWhatChanged(t *testing.T) {
	t.Parallel()
	b, _ := rawFixture(t)
	_, id, _ := firstRawOpener(t, b.page("/activity"))
	dlg := dialogBlock(b.page("/activity?dialog="+id), id)

	before := preBlock(dlg, id+"-1-before")
	if !strings.Contains(before, ">{\n  &#34;") || !strings.Contains(before, "\n  &#34;title&#34;: ") {
		t.Errorf("the before snapshot is not indented JSON:\n%s", before)
	}
	if !strings.Contains(before, `&lt;script&gt;alert(\&#34;x\&#34;)&lt;/script&gt; &#39;quoted&#39; `+strings.Repeat("unbroken", 40)) {
		t.Errorf("the hostile title is not shown escaped and whole:\n%s", before)
	}
	if strings.Contains(dlg, "<script>alert") || strings.Contains(dlg, `\u003cscript`) {
		t.Errorf("the title reaches the page as markup or as \\u escapes:\n%s", dlg)
	}

	row := diffRow(dlg, "title")
	if !strings.Contains(row, `class="rawdiff-row is-changed"`) || !strings.Contains(row, ">changed</span>") ||
		!strings.Contains(row, `&#34;plain again&#34;`) || !strings.Contains(row, `&lt;script&gt;`) {
		t.Errorf("the title is not marked changed with both values:\n%s", row)
	}
	if prio := diffRow(dlg, "priority"); !strings.Contains(prio, "is-changed") {
		t.Errorf("the priority is not marked changed:\n%s", prio)
	}
	if strings.Contains(dlg, `<code>tenant_id</code>`) {
		t.Errorf("an unchanged field is listed as changed:\n%s", dlg)
	}
}

// A creation has an after and nothing to set it beside.
func TestARawDialogOfACreationHasNoDiff(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.createTask("infra", "fresh")
	_, id, _ := firstRawOpener(t, b.page("/activity"))
	dlg := dialogBlock(b.page("/activity?dialog="+id), id)
	if strings.Contains(dlg, `class="rawdiff"`) || strings.Contains(dlg, id+"-1-before") || !strings.Contains(dlg, id+"-1-after") {
		t.Errorf("a creation's dialog should hold its after alone:\n%s", dlg)
	}
}

// preBlock is one <pre> by id.
func preBlock(page, id string) string {
	start := strings.Index(page, `<pre id="`+id+`"`)
	if start < 0 {
		return ""
	}
	rest := page[start:]
	end := strings.Index(rest, "</pre>")
	if end < 0 {
		return rest
	}
	return rest[:end]
}

// diffRow is the changed-fields row for one key.
func diffRow(page, key string) string {
	at := strings.Index(page, `<code>`+key+`</code>`)
	if at < 0 {
		return ""
	}
	start := strings.LastIndex(page[:at], `<div class="rawdiff-row`)
	end := strings.Index(page[at:], "</div>")
	if start < 0 || end < 0 {
		return ""
	}
	return page[start : at+end]
}

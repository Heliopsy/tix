// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"encoding/json"
	"html"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
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
		if tc.name == "task history" && !strings.Contains(page, `data-tab="history" data-tab-forced>`) {
			t.Errorf("the History tab holding the dialog is not the one shown")
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

var rawCopyTag = regexp.MustCompile(`<button type="button" class="link rawcopy" data-copy-text="([^"]*)" aria-label="([^"]+)" data-needs-js>Copy JSON</button>`)

// rowCopies is every row's Copy JSON button on a page: the JSON it copies,
// unescaped as the browser reads the attribute, and its accessible name.
func rowCopies(t *testing.T, page string) (texts, labels []string) {
	t.Helper()
	for _, m := range rawCopyTag.FindAllStringSubmatch(page, -1) {
		texts = append(texts, html.UnescapeString(m[1]))
		labels = append(labels, m[2])
	}
	return texts, labels
}

// sameJSON reports whether two documents decode to the same value.
func sameJSON(t *testing.T, a, b []byte) bool {
	t.Helper()
	var left, right any
	if err := json.Unmarshal(a, &left); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, a)
	}
	if err := json.Unmarshal(b, &right); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, b)
	}
	return reflect.DeepEqual(left, right)
}

// Each history row copies its audit entry as the API lists it, indented,
// without opening Raw: the whole record, snapshots and channel included,
// with a hostile title carried as text in an attribute.
func TestEachHistoryRowCopiesItsEntryAsJSON(t *testing.T) {
	t.Parallel()
	b, ref := rawFixture(t)
	parsed, err := core.ParseTaskRef(ref)
	if err != nil {
		t.Fatal(err)
	}
	task, err := b.fix.svc.GetTask(b.fix.ctx(), parsed)
	if err != nil {
		t.Fatal(err)
	}
	entries, _, err := b.fix.svc.ListAudit(b.fix.ctx(), core.AuditFilter{SubjectType: "task", SubjectID: task.ID,
		Page: core.Page{Sort: "seq", Direction: core.Descending, Limit: 1}})
	if err != nil || len(entries) != 1 {
		t.Fatalf("reading the newest entry: %v, %d entries", err, len(entries))
	}
	api, err := json.Marshal(entries[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/activity", "/tasks/" + ref} {
		page := b.page(path)
		texts, labels := rowCopies(t, page)
		if len(texts) == 0 {
			t.Fatalf("%s: no row has a Copy JSON button", path)
		}
		if strings.Contains(page, `data-copy-text="{`+"\n"+`  "seq"`) || strings.Contains(page, `<script>alert`) {
			t.Errorf("%s: the copied JSON reaches the page unescaped", path)
		}
		var found bool
		for i, text := range texts {
			if !strings.Contains(text, `"seq": `+strconv.FormatInt(entries[0].Seq, 10)+",") {
				continue
			}
			found = true
			if !strings.HasPrefix(text, "{\n  \"seq\": ") {
				t.Errorf("%s: the copied entry is not indented JSON:\n%s", path, text)
			}
			if !sameJSON(t, []byte(text), api) {
				t.Errorf("%s: the row copies\n%s\nwhile the API lists\n%s", path, text, api)
			}
			if !strings.Contains(text, hostileTitle[:8]) {
				t.Errorf("%s: the hostile title is not in the copy as written:\n%s", path, text)
			}
			if labels[i] != "Copy JSON of this entry" {
				t.Errorf("%s: the button is named %q", path, labels[i])
			}
		}
		if !found {
			t.Errorf("%s: no row copies entry %d", path, entries[0].Seq)
		}
	}
}

// A row of several steps copies them all as an array, which is what the
// dialog's Copy all as JSON copies, and the dialog copies each step alone.
func TestAMultiStepRowCopiesEveryStepAndEachAlone(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "walked")
	done := b.post("/tasks/"+ref+"/complete", url.Values{})
	_ = done.Body.Close()
	wantStatus(t, done, http.StatusSeeOther)

	page := b.page("/tasks/" + ref)
	_, id, _ := firstRawOpener(t, page)
	texts, labels := rowCopies(t, page)
	if len(texts) == 0 {
		t.Fatalf("no row has a Copy JSON button")
	}
	var steps []map[string]any
	if err := json.Unmarshal([]byte(texts[0]), &steps); err != nil || len(steps) != 2 {
		t.Fatalf("the move's row copies %d steps (%v), want an array of 2:\n%s", len(steps), err, texts[0])
	}
	if labels[0] != "Copy JSON of these entries" {
		t.Errorf("the move's button is named %q", labels[0])
	}

	dlg := dialogBlock(b.page("/tasks/"+ref+"?dialog="+id), id)
	all := regexp.MustCompile(`data-copy-text="([^"]*)">Copy all as JSON</button>`).FindStringSubmatch(dlg)
	if all == nil || html.UnescapeString(all[1]) != texts[0] {
		t.Errorf("the row and the dialog's Copy all as JSON copy different things")
	}
	for i := range 2 {
		step := regexp.MustCompile(`data-copy-text="([^"]*)" aria-label="Copy JSON of step ` + strconv.Itoa(i+1) + `">Copy JSON</button>`).FindStringSubmatch(dlg)
		if step == nil {
			t.Errorf("step %d of the dialog has no Copy JSON:\n%s", i+1, dlg)
			continue
		}
		one, err := json.Marshal(steps[i])
		if err != nil {
			t.Fatal(err)
		}
		if !sameJSON(t, []byte(html.UnescapeString(step[1])), one) {
			t.Errorf("step %d copies something other than its entry", i+1)
		}
	}
}

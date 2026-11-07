// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/web"
)

// tabButton is one activity tab: its name, whether it is selected, and the
// count it carries, with the class saying whether that count is zero.
var tabButton = regexp.MustCompile(`<button type="button" role="tab" id="(?:dlg-)?tab-(\w+)" aria-controls="(?:dlg-)?(\w+)" data-tab-for="\w+" aria-selected="(true|false)"[^>]*><span class="ttab-name">[^<]+</span><span class="tsec-count( is-zero)?" id="task-count-\w+">([^<]*)</span></button>`)

// tabStrip is the tab list and nothing after it.
func tabStrip(t *testing.T, page string) string {
	t.Helper()
	at := strings.Index(page, `<div class="tabset-list ttabs-list" role="tablist"`)
	if at < 0 {
		t.Fatalf("the task screen has no activity tab strip")
	}
	end := strings.Index(page[at:], "</div>")
	return page[at : at+end]
}

// tabBox is the opening tag of the activity tabs.
func tabBox(t *testing.T, page string) string {
	t.Helper()
	at := strings.Index(page, `<div class="tsec ttabs"`)
	if at < 0 {
		t.Fatalf("the task screen has no activity tabs")
	}
	return page[at : at+strings.Index(page[at:], ">")+1]
}

type shownTab struct {
	name, selected, zero, count string
}

func tabsOf(t *testing.T, page string) []shownTab {
	t.Helper()
	var got []shownTab
	for _, m := range tabButton.FindAllStringSubmatch(tabStrip(t, page), -1) {
		if m[1] != m[2] {
			t.Errorf("tab %s controls panel %s", m[1], m[2])
		}
		got = append(got, shownTab{m[1], m[3], m[4], m[5]})
	}
	return got
}

func TestTheTaskActivityIsTabsWithCounts(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "talked about")
	for _, body := range []string{"one", "two", "three"} {
		resp := b.post("/tasks/"+ref+"/comments", url.Values{"body": {body}})
		_ = resp.Body.Close()
	}
	page := b.page("/tasks/" + ref)
	got := tabsOf(t, page)
	want := []shownTab{{"comments", "true", "", "3"}, {"history", "false", "", "1"}, {"artifacts", "false", " is-zero", "0"}}
	if len(got) != len(want) {
		t.Fatalf("tabs = %+v, want %+v", got, want)
	}
	for i := range want {
		if i == 1 && got[i].count != "" && got[i].count != "0" {
			want[i].count = got[i].count
		}
		if got[i] != want[i] {
			t.Errorf("tab %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if !strings.Contains(tabStrip(t, page), `aria-label="Activity" data-needs-js>`) {
		t.Errorf("the strip is drawn without scripting, or has no name")
	}
	if box := tabBox(t, page); box != `<div class="tsec ttabs" data-task-tabs data-tab="comments">` {
		t.Errorf("the tabs open as %s, want Comments and nothing forced", box)
	}
}

// Without scripting nothing is hidden: the strip is not drawn and every
// panel stands under its own heading.
func TestTheTaskActivityPanelsAllShowWithoutScripting(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	page := b.page("/tasks/" + b.createTask("infra", "no script"))
	for _, name := range []string{"comments", "history", "artifacts"} {
		open := `<section class="ttab-panel" role="tabpanel" id="` + name + `" aria-labelledby="tab-` + name + `" data-tab-panel="` + name + `">`
		at := strings.Index(page, open)
		if at < 0 {
			t.Errorf("no %s panel, or one drawn hidden", name)
			continue
		}
		head := page[at+len(open):]
		if !strings.HasPrefix(strings.TrimSpace(head), `<div class="tsec-head ttab-head"><h2>`) {
			t.Errorf("the %s panel opens without its heading: %.80s", name, strings.TrimSpace(head))
		}
	}
	if regexp.MustCompile(`data-tab-panel="\w+"[^>]*hidden`).MatchString(page) {
		t.Errorf("a panel is drawn hidden, which only the script may do")
	}
}

// A raw entry the address names opens on History whatever the browser last
// chose; an older page of history is checked by
// TestTaskHistoryPagesBackToTheOldestChange.
func TestAHistoryAddressForcesTheHistoryTab(t *testing.T) {
	t.Parallel()
	b, ref := rawFixture(t)
	href, _, _ := firstRawOpener(t, b.page("/tasks/"+ref))
	for _, path := range []string{href} {
		page := b.page(path)
		if box := tabBox(t, page); box != `<div class="tsec ttabs" data-task-tabs data-tab="history" data-tab-forced>` {
			t.Errorf("%s opens the tabs as %s, want History forced", path, box)
		}
		if tabs := tabsOf(t, page); len(tabs) != 3 || tabs[1].selected != "true" || tabs[0].selected != "false" {
			t.Errorf("%s selects %+v, want History", path, tabs)
		}
	}
}

// The task dialog draws the same tabs, with its own ids.
func TestTheTaskDialogCarriesTheActivityTabs(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "in a dialog")
	req, err := http.NewRequest(http.MethodGet, f.server.URL+"/tasks/"+ref, nil)
	if err != nil {
		t.Fatal(err)
	}
	b.stamp(req)
	req.Header.Set(web.DialogHeader, "1")
	resp, err := b.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	page := string(raw)
	if !strings.Contains(page, `id="dlg-tab-history" aria-controls="dlg-history"`) || !strings.Contains(page, `id="dlg-history" aria-labelledby="dlg-tab-history"`) {
		t.Errorf("the dialog has no History tab of its own:\n%s", tabStrip(t, page))
	}
	if len(tabsOf(t, page)) != 3 {
		t.Errorf("the dialog draws %d tabs, want 3", len(tabsOf(t, page)))
	}
}

// A live update redraws the lists and counts, never the strip or the panels
// that hold the reader's choice.
func TestTheLiveTaskFragmentCarriesTheTabCountsNotTheTabs(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "live")
	resp := b.post("/tasks/"+ref+"/comments", url.Values{"body": {"hello"}})
	_ = resp.Body.Close()
	live := b.page("/tasks/" + ref + "/live")
	for _, want := range []string{`<span class="tsec-count" id="task-count-comments">1</span>`, `id="task-count-history">`, `<span class="tsec-count is-zero" id="task-count-artifacts">0</span>`, `id="task-artifacts"`} {
		if !strings.Contains(live, want) {
			t.Errorf("the live fragment lacks %s:\n%s", want, live)
		}
	}
	for _, gone := range []string{`role="tab"`, `data-tab-panel`, `data-task-tabs`} {
		if strings.Contains(live, gone) {
			t.Errorf("the live fragment carries %s, which would reset the reader's tab", gone)
		}
	}
}

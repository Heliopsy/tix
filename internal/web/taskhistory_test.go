// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

var (
	historyRowSeq = regexp.MustCompile(`data-dlg-open="raw-(\d+)"`)
	showOlder     = regexp.MustCompile(`<a class="btn secondary small" href="([^"]+)" hx-get="([^"]+)"`)
)

// historyRegion is the task screen's history table and nothing else.
func historyRegion(t *testing.T, page string) string {
	t.Helper()
	region := between(t, page, `id="task-history"`, `</table>`)
	if region == "" {
		t.Fatalf("the task screen has no history table")
	}
	return region
}

// historySeqs lists the audit entry each history row is drawn from, in the
// order the rows appear.
func historySeqs(t *testing.T, rows string) []int64 {
	t.Helper()
	var out []int64
	for _, m := range historyRowSeq.FindAllStringSubmatch(rows, -1) {
		n, err := strconv.ParseInt(m[1], 10, 64)
		if err != nil {
			t.Fatalf("row id %q: %v", m[0], err)
		}
		out = append(out, n)
	}
	return out
}

// olderLinks reads the Show older control: where it goes without JavaScript
// and what it fetches with it. Both are empty when the control is absent.
func olderLinks(rows string) (href, rowsHref string) {
	m := showOlder.FindStringSubmatch(rows)
	if m == nil {
		return "", ""
	}
	return html.UnescapeString(m[1]), html.UnescapeString(m[2])
}

// editTimes writes n edits to a task, each its own audit entry.
func editTimes(t *testing.T, f *fixture, ref string, n int) {
	t.Helper()
	parsed, err := core.ParseTaskRef(ref)
	if err != nil {
		t.Fatal(err)
	}
	for i := range n {
		title := fmt.Sprintf("edit %d", i)
		if _, err := f.svc.UpdateTask(f.ctx(), parsed, core.UpdateTaskInput{Title: &title}); err != nil {
			t.Fatalf("edit %d: %v", i, err)
		}
	}
}

// taskAudit is every audit entry recorded against a task, newest first.
func taskAudit(t *testing.T, f *fixture, ref string) []int64 {
	t.Helper()
	parsed, err := core.ParseTaskRef(ref)
	if err != nil {
		t.Fatal(err)
	}
	task, err := f.svc.GetTask(f.ctx(), parsed)
	if err != nil {
		t.Fatal(err)
	}
	entries, next, err := f.svc.ListAudit(f.ctx(), core.AuditFilter{SubjectType: "task", SubjectID: task.ID,
		Page: core.Page{Sort: "seq", Direction: core.Descending, Limit: core.MaxPageLimit}})
	if err != nil || next != "" {
		t.Fatalf("reading the trail: %v, next %q", err, next)
	}
	out := make([]int64, len(entries))
	for i, e := range entries {
		out[i] = e.Seq
	}
	return out
}

// A task with more history than one page draws the newest page and a Show
// older control, and following it, with JavaScript or without, reaches the
// oldest change with every change drawn exactly once, newest first.
func TestTaskHistoryPagesBackToTheOldestChange(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "long history")
	editTimes(t, f, ref, 119)
	want := taskAudit(t, f, ref)
	if len(want) != 120 {
		t.Fatalf("the task has %d audit entries, want 120", len(want))
	}

	first := historyRegion(t, b.page("/tasks/"+ref))
	if got := len(historySeqs(t, first)); got != core.DefaultDisplayLimit {
		t.Fatalf("the first page draws %d rows, want %d", got, core.DefaultDisplayLimit)
	}
	if strings.Contains(first, "Newest changes") {
		t.Errorf("the newest page offers a way back to itself")
	}

	t.Run("with JavaScript", func(t *testing.T) {
		got, rows, fetches := historySeqs(t, first), first, 0
		for {
			_, next := olderLinks(rows)
			if next == "" {
				break
			}
			if fetches++; fetches > 10 {
				t.Fatalf("Show older never runs out")
			}
			if !strings.HasPrefix(next, "/tasks/"+ref+"/history?") {
				t.Fatalf("Show older fetches %q", next)
			}
			rows = b.page(next)
			if strings.Contains(rows, "<table") || strings.Contains(rows, "<html") {
				t.Fatalf("Show older answered more than rows:\n%s", rows)
			}
			got = append(got, historySeqs(t, rows)...)
		}
		if !slices.Equal(got, want) {
			t.Errorf("rows drawn = %v\nwant %v", got, want)
		}
		if fetches != 4 {
			t.Errorf("Show older was followed %d times, want 4", fetches)
		}
	})

	t.Run("without JavaScript", func(t *testing.T) {
		got, rows, visits := historySeqs(t, first), first, 0
		for {
			next, _ := olderLinks(rows)
			if next == "" {
				break
			}
			if visits++; visits > 10 {
				t.Fatalf("Show older never runs out")
			}
			u, err := url.Parse(next)
			if err != nil || u.Path != "/tasks/"+ref || u.Query().Get("history") == "" {
				t.Fatalf("Show older links to %q", next)
			}
			page := b.page(next)
			if !strings.Contains(page, `id="history" open`) {
				t.Errorf("an older page of history arrives folded away")
			}
			rows = historyRegion(t, page)
			if !strings.Contains(rows, `href="/tasks/`+ref+`#history">Newest changes</a>`) {
				t.Errorf("an older page has no way back to the newest changes:\n%s", rows)
			}
			got = append(got, historySeqs(t, rows)...)
		}
		if !slices.Equal(got, want) {
			t.Errorf("rows drawn = %v\nwant %v", got, want)
		}
	})
}

// A history that ends exactly on a page boundary offers no Show older on its
// last page: the store hands back a cursor for a full page whether or not
// anything follows it, and a control leading to an empty page is a lie.
func TestTaskHistoryOffersNoOlderPageWhenThereIsNone(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	short := b.createTask("infra", "short history")
	editTimes(t, f, short, 3)
	if href, rows := olderLinks(historyRegion(t, b.page("/tasks/"+short))); href != "" || rows != "" {
		t.Errorf("a four-entry history offers Show older: %q %q", href, rows)
	}

	exact := b.createTask("infra", "exactly two pages")
	editTimes(t, f, exact, 2*core.DefaultDisplayLimit-1)
	_, next := olderLinks(historyRegion(t, b.page("/tasks/"+exact)))
	if next == "" {
		t.Fatalf("a two-page history offers no Show older")
	}
	last := b.page(next)
	if got := len(historySeqs(t, last)); got != core.DefaultDisplayLimit {
		t.Errorf("the second page draws %d rows, want %d", got, core.DefaultDisplayLimit)
	}
	if href, rows := olderLinks(last); href != "" || rows != "" {
		t.Errorf("the last page offers Show older: %q %q", href, rows)
	}
}

// A multi-hop move is one row even when its hops fall either side of where a
// page would end: the older hop joins the page holding the newer one rather
// than opening the next page as a move of its own.
func TestTaskHistoryKeepsAMoveWholeAcrossAPageBoundary(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "moved then edited")
	done := b.post("/tasks/"+ref+"/complete", url.Values{})
	_ = done.Body.Close()
	wantStatus(t, done, http.StatusSeeOther)
	editTimes(t, f, ref, core.DefaultDisplayLimit-1)
	trail := taskAudit(t, f, ref)
	if len(trail) != core.DefaultDisplayLimit+2 {
		t.Fatalf("the trail holds %d entries, want a create, two hops and %d edits", len(trail), core.DefaultDisplayLimit-1)
	}

	first := historyRegion(t, b.page("/tasks/"+ref))
	seqs := historySeqs(t, first)
	if len(seqs) != core.DefaultDisplayLimit || seqs[len(seqs)-1] != trail[len(trail)-3] {
		t.Fatalf("the first page draws %v, want %d rows ending on the move's last hop %d",
			seqs, core.DefaultDisplayLimit, trail[len(trail)-3])
	}
	if !strings.Contains(first, "from todo to done (2 steps)") {
		t.Errorf("the move split across the page boundary:\n%s", first)
	}
	_, next := olderLinks(first)
	if next == "" {
		t.Fatalf("the creation is left with no page to show it on")
	}
	rest := b.page(next)
	if got := historySeqs(t, rest); !slices.Equal(got, trail[len(trail)-1:]) {
		t.Errorf("the older page draws %v, want only the creation %d", got, trail[len(trail)-1])
	}
	if strings.Contains(rest, "moved this") {
		t.Errorf("the older page draws part of the move:\n%s", rest)
	}
}

// A reader who may read the task but not its audit trail is told so by the
// rows Show older fetches, and is handed none of them.
func TestTaskHistoryRowsRefuseAReaderWithoutAuditScope(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "audited")
	editTimes(t, f, ref, core.DefaultDisplayLimit+5)
	_, next := olderLinks(historyRegion(t, b.page("/tasks/"+ref)))
	if next == "" {
		t.Fatalf("no Show older to follow")
	}

	rows := f.as("tokenkeeper").page(next)
	if !strings.Contains(rows, "You do not hold the audit read scope.") {
		t.Errorf("the refusal is not said:\n%s", rows)
	}
	if seqs := historySeqs(t, rows); len(seqs) != 0 {
		t.Errorf("a reader without audit scope was handed rows %v", seqs)
	}
	if href, more := olderLinks(rows); href != "" || more != "" {
		t.Errorf("a reader without audit scope is offered Show older")
	}
}

// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// auditPages answers ListAudit from a fixed list of store pages, so a scan
// that reads more than one of them can be observed without a store. Every
// other Service method is left to the embedded nil interface: scanAudit calls
// none of them, and a call that appeared would panic rather than pass.
type auditPages struct {
	core.Service
	pages [][]core.AuditEntry
}

// cursorFor names the page that follows the given index, empty at the end.
func (a *auditPages) cursorFor(i int) string {
	if i+1 >= len(a.pages) {
		return ""
	}
	return fmt.Sprintf("page-%d", i+1)
}

func (a *auditPages) ListAudit(_ context.Context, f core.AuditFilter) ([]core.AuditEntry, string, error) {
	for i := range a.pages {
		want := ""
		if i > 0 {
			want = fmt.Sprintf("page-%d", i)
		}
		if f.Page.Cursor == want {
			return a.pages[i], a.cursorFor(i), nil
		}
	}
	return nil, "", nil
}

// auditEntry builds one entry whose snapshot carries the given word, which is
// where matchesText looks.
func auditEntry(seq int64, word string) core.AuditEntry {
	return core.AuditEntry{
		Seq: seq, Action: "task.create", SubjectType: "task",
		SubjectID: fmt.Sprintf("t%d", seq), Source: core.SourceWeb,
		After: []byte(fmt.Sprintf(`{"title":%q}`, word)),
	}
}

// A feed walked from its first page to its last has to show every entry that
// answered the filter. The free-text box is applied here rather than by the
// store, so one page of the feed is assembled from more than one page of the
// store, and the cursor the screen carries forward has to name the first
// entry it did not show -- not the end of the last store page it read.
func TestTheActivityFeedShowsEveryMatchItScanned(t *testing.T) {
	t.Parallel()

	const word = "zebra"
	var first, second []core.AuditEntry
	seq := int64(1000)
	// The newest store page holds few matches, so the scan reads on; the one
	// behind it holds enough to carry the total past a screenful.
	for i := 0; i < activityPageSize; i++ {
		title := "other"
		if i < 10 {
			title = word
		}
		first = append(first, auditEntry(seq, title))
		seq--
	}
	for i := 0; i < activityPageSize; i++ {
		title := "other"
		if i < 45 {
			title = word
		}
		second = append(second, auditEntry(seq, title))
		seq--
	}
	matched := map[string]bool{}
	for _, e := range append(append([]core.AuditEntry{}, first...), second...) {
		if matchesText(e, word) {
			matched[e.SubjectID] = true
		}
	}
	if len(matched) != 55 {
		t.Fatalf("the fixture matched %d entries, want 55", len(matched))
	}

	h := &handler{svc: &auditPages{pages: [][]core.AuditEntry{first, second, nil}}}
	query := activityQuery{Text: word}

	shown := map[string]bool{}
	cursor := ""
	for page := 0; page < 10; page++ {
		r := httptest.NewRequest(http.MethodGet, RouteActivity, nil)
		entries, next, err := h.scanAudit(r, query, cursor)
		if err != nil {
			t.Fatalf("scanning page %d: %v", page+1, err)
		}
		for _, e := range entries {
			if shown[e.SubjectID] {
				t.Errorf("entry %s was shown on two pages of the walk", e.SubjectID)
			}
			shown[e.SubjectID] = true
		}
		if next == "" || next == cursor {
			break
		}
		cursor = next
	}

	var missing []string
	for id := range matched {
		if !shown[id] {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		t.Errorf("walking the feed to its end showed %d of %d matching entries; %d never appeared on any page (%v)",
			len(shown), len(matched), len(missing), missing)
	}
}

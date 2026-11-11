// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// reactionsOf cuts one comment's reactions out of the task screen: from the
// region's own id to the comment's edit popover, or the end of the comment
// for a reader who cannot edit.
func reactionsOf(t *testing.T, page, commentID string) string {
	t.Helper()
	_, rest, ok := strings.Cut(page, `<div class="reactions" id="reactions-`+commentID+`">`)
	if !ok {
		t.Fatalf("the comment %s has no reactions region", commentID)
	}
	for _, end := range []string{`class="flowmenu commentpop"`, `</li>`} {
		if before, _, found := strings.Cut(rest, end); found {
			rest = before
		}
	}
	return rest
}

func TestCommentReactionsToggleUnderTheirComment(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "reacted to")
	added := b.post("/tasks/"+ref+"/comments", url.Values{"body": {"ship it"}})
	_ = added.Body.Close()
	id := between(t, b.page("/tasks/"+ref), `<input type="hidden" name="id" value="`, `"`)
	heart := core.ReactionHeart.Emoji()

	region := reactionsOf(t, b.page("/tasks/"+ref), id)
	if strings.Contains(region, `class="reaction-count"`) {
		t.Fatalf("a comment nobody reacted to shows a count:\n%s", region)
	}
	if !strings.Contains(region, `class="reaction-add"`) || strings.Count(region, `class="reaction-pick`) != len(core.Reactions()) {
		t.Fatalf("the picker does not offer every reaction:\n%s", region)
	}
	if !strings.Contains(region, `hx-target="#reactions-`+id+`" hx-select="#reactions-`+id+`"`) {
		t.Fatalf("a reaction form does not swap only its own comment's reactions:\n%s", region)
	}

	reacted := b.post("/tasks/"+ref+"/comments/react", url.Values{"id": {id}, "reaction": {"heart"}})
	_ = reacted.Body.Close()
	wantStatus(t, reacted, http.StatusSeeOther)
	if loc := reacted.Header.Get("Location"); !strings.HasSuffix(loc, "#comment-"+id) {
		t.Fatalf("the reaction returned the reader to %q, not to the comment", loc)
	}
	region = reactionsOf(t, b.page("/tasks/"+ref), id)
	chip, _, _ := strings.Cut(region, `class="reaction-add"`)
	if !strings.Contains(chip, `action="/tasks/`+ref+`/comments/unreact"`) ||
		!strings.Contains(chip, `class="reaction is-mine" aria-pressed="true"`) ||
		!strings.Contains(chip, heart) || !strings.Contains(chip, `>1</span>`) || !strings.Contains(chip, `title="alice`) {
		t.Fatalf("the reader's heart is not drawn as theirs, with its count and who gave it:\n%s", chip)
	}

	viewer := reactionsOf(t, f.as("viewer").page("/tasks/"+ref), id)
	if strings.Contains(viewer, "<form") || strings.Contains(viewer, `class="reaction-add"`) ||
		!strings.Contains(viewer, `<span class="reaction"`) || !strings.Contains(viewer, heart) {
		t.Fatalf("a reader who may not comment is offered a toggle, or not shown the reaction:\n%s", viewer)
	}

	taken := b.post("/tasks/"+ref+"/comments/unreact", url.Values{"id": {id}, "reaction": {"heart"}})
	_ = taken.Body.Close()
	wantStatus(t, taken, http.StatusSeeOther)
	if region := reactionsOf(t, b.page("/tasks/"+ref), id); strings.Contains(region, `class="reaction-count"`) {
		t.Fatalf("the heart taken back is still drawn:\n%s", region)
	}
}

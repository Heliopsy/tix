// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// renderThread draws one task's comment thread for a reader.
func renderThread(t *testing.T, canComment bool) string {
	t.Helper()
	tmpl := Handler(nil).(*handler).templates["task.html"]
	c := core.Comment{ID: "c1", AuthorActorID: "ada", Body: "ship it", Reactions: []core.Reaction{
		{Emoji: "👍", Count: 2, ActorIDs: []string{"ada", "bob"}},
		{Emoji: "🎉", Count: 1, ActorIDs: []string{"bob"}},
	}}
	data := taskView{
		Task: core.Task{Ref: "infra-1"}, Comments: []core.Comment{c},
		ViewerID: "ada", CanComment: canComment,
		Names: actorNames{labels: map[string]string{"ada": "@ada", "bob": "@bob"}},
	}
	var out strings.Builder
	if err := tmpl.ExecuteTemplate(&out, "task-comments", view{CSRF: "tok", Data: data}); err != nil {
		t.Fatalf("rendering: %v", err)
	}
	return out.String()
}

// chip is the one form a reaction chip lives in, so an assertion about it
// cannot be satisfied by the picker or by the other chip.
func chip(t *testing.T, page, emoji string) string {
	t.Helper()
	marker := `name="emoji" value="` + emoji + `">`
	at := strings.Index(page, marker)
	if at < 0 {
		t.Fatalf("no chip for %s in:\n%s", emoji, page)
	}
	start := strings.LastIndex(page[:at], "<form")
	end := strings.Index(page[at:], "</form>")
	return page[start : at+end]
}

func TestReactionChipsShowCountsWhoAndTheReadersOwn(t *testing.T) {
	t.Parallel()
	page := renderThread(t, true)

	mine := chip(t, page, "👍")
	if !strings.Contains(mine, `action="/tasks/infra-1/comments/unreact"`) ||
		!strings.Contains(mine, `class="reaction mine"`) || !strings.Contains(mine, `aria-pressed="true"`) {
		t.Errorf("the reader's own reaction is not drawn pressed and taking back:\n%s", mine)
	}
	if !strings.Contains(mine, `title="@ada, @bob"`) || !strings.Contains(mine, `>2</span>`) {
		t.Errorf("the chip does not name who reacted or count them:\n%s", mine)
	}

	theirs := chip(t, page, "🎉")
	if !strings.Contains(theirs, `action="/tasks/infra-1/comments/react"`) ||
		strings.Contains(theirs, "mine") || !strings.Contains(theirs, `aria-pressed="false"`) {
		t.Errorf("somebody else's reaction is drawn as the reader's:\n%s", theirs)
	}

	for _, e := range QuickReactions {
		if !strings.Contains(page, `class="reaction-pick" aria-label="React with `+e+`">`) {
			t.Errorf("the picker does not offer %s", e)
		}
	}
	if !strings.Contains(page, `class="reaction-add" popovertarget=`) {
		t.Error("no add-reaction button")
	}
}

func TestAReaderWhoMayNotCommentCannotReact(t *testing.T) {
	t.Parallel()
	page := renderThread(t, false)
	if strings.Contains(page, "reaction-add") || strings.Contains(page, "reaction-pick") {
		t.Error("the picker is offered to a reader without comment:write")
	}
	if !strings.Contains(chip(t, page, "👍"), " disabled>") {
		t.Error("a chip can be pressed by a reader without comment:write")
	}
}

func TestTheTaskScreenSubscribesToReactions(t *testing.T) {
	t.Parallel()
	for _, typ := range []core.EventType{core.EventCommentReacted, core.EventCommentUnreacted} {
		e := core.Event{Type: typ, ProjectID: "p", SubjectID: "c"}
		if !(core.EventFilter{ProjectIDs: []string{"p"}, Types: liveDetailTypes}).Matches(e) {
			t.Errorf("the task screen's subscription drops %s", typ)
		}
	}
}

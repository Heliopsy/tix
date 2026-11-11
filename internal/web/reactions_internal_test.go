// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestReactionChoicesOfferTheFixedSetInOrder(t *testing.T) {
	t.Parallel()
	c := core.Comment{Reactions: []core.ReactionSummary{{Reaction: core.ReactionEyes, Emoji: core.ReactionEyes.Emoji(), Count: 3, Reacted: true}}}
	got := reactionChoices(c)
	if len(got) != len(core.Reactions()) {
		t.Fatalf("offered %d reactions, want %d", len(got), len(core.Reactions()))
	}
	for i, r := range core.Reactions() {
		if got[i].Reaction != r || got[i].Emoji != r.Emoji() || got[i].Label == "" {
			t.Fatalf("choice %d = %+v, want %s with its emoji and a label", i, got[i], r)
		}
	}
	if last := got[len(got)-1]; last.Count != 3 || !last.Reacted || got[0].Count != 0 || got[0].Reacted {
		t.Fatalf("the comment's own counts did not reach their choices: %+v", got)
	}
}

// A reaction given in one tab has to redraw the thread in the reader's other
// tabs, which it does only if the task screen subscribes to both events.
func TestTheTaskScreenSubscribesToReactions(t *testing.T) {
	t.Parallel()
	for _, typ := range []core.EventType{core.EventCommentReacted, core.EventCommentUnreacted} {
		e := core.Event{Type: typ, ProjectID: "p", SubjectID: "c"}
		if !(core.EventFilter{ProjectIDs: []string{"p"}, Types: liveDetailTypes}).Matches(e) {
			t.Errorf("the task screen's subscription drops %s", typ)
		}
	}
}

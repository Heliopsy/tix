// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

// reactWith presses the reaction key on the selected comment and applies emoji.
func reactWith(t *testing.T, m Model, emoji string) Model {
	t.Helper()
	m, _ = m.reduce(pressKey("&"))
	if m.prompt != promptReact {
		t.Fatalf("& opened prompt %v: %q", m.prompt, m.err)
	}
	m.input.SetValue(emoji)
	m, cmd := m.reduce(pressKey("enter"))
	if m.prompt != promptNone {
		t.Fatal("the prompt stayed open")
	}
	run(t, cmd)
	return m
}

func TestTheReactionKeyAddsOrTakesBackOnTheSelectedComment(t *testing.T) {
	m, svc := threadModel(t)
	m.actor = &core.Actor{ID: "a-ada"}
	m.detail.comments[1].Reactions = []core.Reaction{{Emoji: "👍", Count: 1, ActorIDs: []string{"a-ada"}}}
	m, _ = m.reduce(pressKey("right"))

	m = reactWith(t, m, "🎉")
	reactWith(t, m, "👍")
	want := []string{"+c-2 🎉", "-c-2 👍"}
	if !slices.Equal(svc.reactions, want) {
		t.Fatalf("reactions sent = %v, want %v", svc.reactions, want)
	}
}

func TestTheThreadShowsEachCommentsReactions(t *testing.T) {
	c := core.Comment{ID: "c", AuthorActorID: "a", Body: "ship it",
		Reactions: []core.Reaction{{Emoji: "👍", Count: 2}, {Emoji: "🎉", Count: 1}}}
	lines := commentLines([]core.Comment{c}, nil, output.TimeStyle{}, -1, 40)
	if len(lines) != 3 || strings.TrimSpace(lines[2]) != "👍 2  🎉 1" {
		t.Fatalf("thread = %q", lines)
	}
}

func TestTheReactionKeyIsOfferedOnlyToAReaderWhoMayReact(t *testing.T) {
	k := DefaultKeyMap()
	if !slices.Contains(keysOf(k.ViewHelp(viewDetail, permitAll)), "&") {
		t.Fatal("a reader who may do everything is not told about &")
	}
	if slices.Contains(keysOf(k.ViewHelp(viewDetail, permitOnly())), "&") {
		t.Fatal("a reader who may do nothing is told about &")
	}
}

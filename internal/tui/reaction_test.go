// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// reactedThread is the thread with the second comment carrying the reader's
// own thumbs up and somebody else's rocket.
func reactedThread(t *testing.T) (Model, *fakeService) {
	t.Helper()
	m, svc := threadModel(t)
	m.detail.comments[1].Reactions = []core.ReactionSummary{
		{Reaction: core.ReactionThumbsUp, Emoji: core.ReactionThumbsUp.Emoji(), Count: 2, Reacted: true, ActorIDs: []string{"a-ada", "me"}},
		{Reaction: core.ReactionRocket, Emoji: core.ReactionRocket.Emoji(), Count: 1, ActorIDs: []string{"a-ada"}},
	}
	return m, svc
}

func TestReactingPicksFromTheFixedSetAndTogglesTheReadersOwn(t *testing.T) {
	cases := []struct {
		name  string
		press string
		want  string
	}{
		{"a reaction the reader has not given is added", "8", "+c-2 eyes"},
		{"another's reaction is joined", "7", "+c-2 rocket"},
		{"the reader's own is taken back", "1", "-c-2 thumbs_up"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, svc := reactedThread(t)
			m, _ = m.reduce(pressKey("right"))
			m, _ = m.reduce(pressKey(")"))
			if m.choice != choiceReaction {
				t.Fatalf(") opened choice %v: %q", m.choice, m.err)
			}
			if len(m.choices) != len(core.Reactions()) {
				t.Fatalf("the picker offers %d reactions, want %d", len(m.choices), len(core.Reactions()))
			}
			m, cmd := m.reduce(pressKey(tc.press))
			run(t, cmd)
			if !slices.Equal(svc.reactions, []string{tc.want}) {
				t.Fatalf("the service was asked %v, want %q", svc.reactions, tc.want)
			}
		})
	}
}

func TestReactingOnTheBoardSaysToOpenTheTask(t *testing.T) {
	m := boardModel(t)
	svc := newFakeService()
	m.svc = svc
	m, cmd := m.reduce(pressKey(")"))
	if cmd != nil {
		cmd()
	}
	if m.choice != choiceNone || len(svc.reactions) != 0 {
		t.Fatalf("the board opened a picker or reacted: choice %v, calls %v", m.choice, svc.reactions)
	}
	if !strings.Contains(m.err, "open the task") {
		t.Fatalf("err = %q", m.err)
	}
}

func TestThreadShowsReactionsUnderTheirComment(t *testing.T) {
	m, _ := reactedThread(t)
	lines := commentLines(m.detail.comments, nil, m.timeStyle, 0, 60)
	want := "[" + core.ReactionThumbsUp.Emoji() + " 2]  " + core.ReactionRocket.Emoji() + " 1"
	last := lines[len(lines)-1]
	if strings.TrimSpace(last) != want {
		t.Fatalf("the last thread line is %q, want the reactions %q", last, want)
	}
	for _, l := range lines[:len(lines)-1] {
		if strings.Contains(l, core.ReactionRocket.Emoji()) {
			t.Fatalf("a reaction was drawn away from its comment: %q", l)
		}
	}
	if picks := ReactionChoices(m.detail.comments[1]); !strings.Contains(picks[0].Label, "yours") ||
		strings.Contains(picks[6].Label, "yours") {
		t.Fatalf("the picker does not say which reaction is the reader's: %v", picks)
	}
}

// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"slices"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

func TestReactionEmojiAcceptsOneEmojiGrapheme(t *testing.T) {
	cases := []struct {
		in string
		ok bool
	}{
		{"👍", true},
		{" 🎉 ", true},
		{"❤️", true},
		{"❤", true},
		{"👍🏽", true},
		{"👩\u200d💻", true},
		{"👨\u200d👩\u200d👧\u200d👦", true},
		{"🇧🇬", true},
		{"1️⃣", true},
		{"🏴\U000e0067\U000e0062\U000e0073\U000e0063\U000e0074\U000e007f", true},
		{"", false},
		{"a", false},
		{"ok", false},
		{"1", false},
		{"👍👍", false},
		{"👍 ", true},
		{"👍x", false},
		{"🇧", false},
		{"🇧🇬🇧", false},
		{"\u200d👍", false},
		{"👍\u200d", false},
		{"🏽", false},
		{"<script>", false},
	}
	for _, c := range cases {
		_, err := reactionEmoji(c.in)
		if (err == nil) != c.ok {
			t.Errorf("reactionEmoji(%q) err = %v, want ok = %v", c.in, err, c.ok)
		}
		if err != nil && !core.IsKind(err, core.KindInvalid) {
			t.Errorf("reactionEmoji(%q) kind = %v, want invalid", c.in, err)
		}
	}
}

func TestReactionsGroupByEmojiAndToggleOnce(t *testing.T) {
	l, ctx, scope, me, _ := newTaskFixture(t)
	bob := seedTaskActor(t, l, scope, "bob", core.RoleMember, core.ScopeAll)
	other := taskContext(bob)
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "discussed"})
	ref := core.TaskRef{ID: task.ID}
	c, err := l.AddComment(ctx, ref, "ship it")
	if err != nil {
		t.Fatal(err)
	}

	steps := []struct {
		name  string
		ctx   context.Context
		call  func(context.Context, string, string) (*core.Comment, error)
		emoji string
		want  core.EventType
	}{
		{"react", ctx, l.AddReaction, "👍", core.EventCommentReacted},
		{"react again", ctx, l.AddReaction, "👍", ""},
		{"a second emoji", ctx, l.AddReaction, "🎉", core.EventCommentReacted},
		{"another reader", other, l.AddReaction, "👍", core.EventCommentReacted},
		{"take back", ctx, l.RemoveReaction, "🎉", core.EventCommentUnreacted},
		{"take back again", ctx, l.RemoveReaction, "🎉", ""},
	}
	for _, step := range steps {
		events, audits := countRows(t, l, scope)
		if _, err := step.call(step.ctx, c.ID, step.emoji); err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}
		afterEvents, afterAudits := countRows(t, l, scope)
		write := step.want != ""
		if write != (afterEvents == events+1 && afterAudits == audits+1) ||
			!write && (afterEvents != events || afterAudits != audits) {
			t.Fatalf("%s: events %d -> %d, audits %d -> %d", step.name, events, afterEvents, audits, afterAudits)
		}
		if !write {
			continue
		}
		if err := l.store.View(context.Background(), scope, func(tx store.Tx) error {
			evs, err := tx.ReadEvents(context.Background(), 0, 1000)
			if err != nil {
				return err
			}
			last := evs[len(evs)-1]
			if last.Type != step.want || last.Payload["emoji"] != step.emoji || last.Payload["ref"] != task.Ref {
				t.Errorf("%s: event = %s %v", step.name, last.Type, last.Payload)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}

	listed, err := l.ListComments(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	got := listed[0].Reactions
	if len(got) != 1 || got[0].Emoji != "👍" || got[0].Count != 2 ||
		!slices.Equal(got[0].ActorIDs, []string{me.ID, bob.ID}) {
		t.Fatalf("reactions = %+v, want 👍 by both readers", got)
	}

	if _, err := l.AddReaction(ctx, c.ID, "nope"); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("a word as a reaction = %v, want invalid", err)
	}
	if _, err := l.AddReaction(ctx, "missingcomment01", "👍"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("reacting to a missing comment = %v, want not_found", err)
	}
	if _, err := l.AddReaction(t.Context(), c.ID, "👍"); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("reacting unauthenticated = %v", err)
	}
}

func TestDeletingACommentRemovesItsReactions(t *testing.T) {
	l, ctx, scope, _, _ := newTaskFixture(t)
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "discussed"})
	c, err := l.AddComment(ctx, core.TaskRef{ID: task.ID}, "regrettable")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.AddReaction(ctx, c.ID, "👀"); err != nil {
		t.Fatal(err)
	}
	if err := l.DeleteComment(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	if err := l.store.View(context.Background(), scope, func(tx store.Tx) error {
		rows, err := tx.ListReactions(context.Background(), []string{c.ID})
		if len(rows) != 0 {
			t.Errorf("reactions left on a deleted comment: %+v", rows)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := l.AddReaction(ctx, c.ID, "👍"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("reacting to a deleted comment = %v, want not_found", err)
	}
}

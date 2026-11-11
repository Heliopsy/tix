// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

func reactAs(t *testing.T, f fixture, commentID, actorID string, r core.Reaction, on bool) bool {
	t.Helper()
	var changed bool
	err := f.store.Update(context.Background(), f.scope, func(tx store.Tx) error {
		var err error
		if on {
			changed, err = tx.AddCommentReaction(context.Background(), commentID, actorID, r)
		} else {
			changed, err = tx.RemoveCommentReaction(context.Background(), commentID, actorID, r)
		}
		return err
	})
	if err != nil {
		t.Fatalf("reaction %v %s on %s by %s: %v", on, r, commentID, actorID, err)
	}
	return changed
}

func reactionsOf(t *testing.T, f fixture, scope core.TenantScope, ids ...string) []string {
	t.Helper()
	var out []string
	if err := f.store.View(context.Background(), scope, func(tx store.Tx) error {
		rows, err := tx.ListCommentReactions(context.Background(), ids)
		for _, r := range rows {
			out = append(out, r.CommentID+" "+r.ActorID+" "+string(r.Reaction))
		}
		return err
	}); err != nil {
		t.Fatalf("listing reactions: %v", err)
	}
	return out
}

func newComment(t *testing.T, f fixture, taskID, body string) core.Comment {
	t.Helper()
	c := core.Comment{TaskID: taskID, AuthorActorID: f.actor.ID, Body: body}
	if err := f.store.Update(context.Background(), f.scope, func(tx store.Tx) error {
		return tx.CreateComment(context.Background(), &c)
	}); err != nil {
		t.Fatalf("creating comment: %v", err)
	}
	return c
}

func TestCommentReactionsAreOnePerActorAndReaction(t *testing.T) {
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	other := core.Actor{Kind: core.ActorUser, Handle: "other"}
	if err := s.Update(context.Background(), f.scope, func(tx store.Tx) error {
		return tx.CreateActor(context.Background(), &other)
	}); err != nil {
		t.Fatal(err)
	}
	task := f.newTask(t, "reacted", core.PriorityNormal)
	first := newComment(t, f, task.ID, "first")
	second := newComment(t, f, task.ID, "second")

	steps := []struct {
		comment, actor string
		r              core.Reaction
		on, want       bool
	}{
		{first.ID, f.actor.ID, core.ReactionThumbsUp, true, true},
		{first.ID, f.actor.ID, core.ReactionThumbsUp, true, false},
		{first.ID, f.actor.ID, core.ReactionHeart, true, true},
		{first.ID, other.ID, core.ReactionThumbsUp, true, true},
		{second.ID, other.ID, core.ReactionEyes, true, true},
		{first.ID, f.actor.ID, core.ReactionHeart, false, true},
		{first.ID, f.actor.ID, core.ReactionHeart, false, false},
	}
	for i, st := range steps {
		clk.Advance(time.Second)
		if got := reactAs(t, f, st.comment, st.actor, st.r, st.on); got != st.want {
			t.Fatalf("step %d: changed = %v, want %v", i, got, st.want)
		}
	}
	want := []string{
		first.ID + " " + f.actor.ID + " thumbs_up",
		first.ID + " " + other.ID + " thumbs_up",
		second.ID + " " + other.ID + " eyes",
	}
	if got := reactionsOf(t, f, f.scope, first.ID, second.ID); !slices.Equal(got, want) {
		t.Fatalf("reactions = %v, want %v", got, want)
	}
	if got := reactionsOf(t, f, f.scope, second.ID); !slices.Equal(got, want[2:]) {
		t.Fatalf("reactions of one comment = %v, want %v", got, want[2:])
	}
	if got := reactionsOf(t, f, f.scope); len(got) != 0 {
		t.Fatalf("no comments named, yet read %v", got)
	}
}

func TestCommentReactionsStayInsideTheirTenant(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	a := seed(t, s, clk, "acme")
	b := seed(t, s, clk, "beta")
	victim := newComment(t, a, a.newTask(t, "a's task", core.PriorityNormal).ID, "a's comment")

	if err := s.Update(ctx, b.scope, func(tx store.Tx) error {
		_, err := tx.AddCommentReaction(ctx, victim.ID, b.actor.ID, core.ReactionRocket)
		return err
	}); err != nil {
		t.Fatalf("writing a cross-tenant reaction row: %v", err)
	}
	if got := reactionsOf(t, a, a.scope, victim.ID); len(got) != 0 {
		t.Fatalf("a reaction written under another tenant reached this one: %v", got)
	}
}

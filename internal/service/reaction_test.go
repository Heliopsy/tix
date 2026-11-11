// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

func summary(c *core.Comment) []string {
	var out []string
	for _, s := range c.Reactions {
		mark := ""
		if s.Reacted {
			mark = "*"
		}
		out = append(out, s.Emoji+string(s.Reaction)+mark+" "+string(rune('0'+s.Count)))
	}
	return out
}

func TestReactionsSumUpPerReaderInOfferOrder(t *testing.T) {
	l, clk, scope, me := newLocal(t)
	seedTaskProject(t, l, scope, "infra")
	ctx := taskContext(me)
	bob := seedTaskActor(t, l, scope, "bob", core.RoleMember, core.ScopeAll)
	bobC := taskContext(bob)
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "x"})
	c, err := l.AddComment(ctx, core.TaskRef{ID: task.ID}, "ship it")
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		ctx context.Context
		r   core.Reaction
	}{
		{ctx, core.ReactionRocket}, {bobC, "\U0001F44D"}, {ctx, "THUMBS_UP"}, {bobC, core.ReactionRocket},
	} {
		clk.Advance(time.Second)
		if _, err := l.AddCommentReaction(step.ctx, c.ID, step.r); err != nil {
			t.Fatalf("reacting %q: %v", step.r, err)
		}
	}
	thumbs, rocket := core.ReactionThumbsUp.Emoji(), core.ReactionRocket.Emoji()
	cases := []struct {
		name string
		ctx  context.Context
		want []string
	}{
		{"me", ctx, []string{thumbs + "thumbs_up* 2", rocket + "rocket* 2"}},
		{"bob", bobC, []string{thumbs + "thumbs_up* 2", rocket + "rocket* 2"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := l.ListComments(tc.ctx, core.TaskRef{ID: task.ID})
			if err != nil || len(got) != 1 {
				t.Fatalf("listing: %v %v", got, err)
			}
			if s := summary(&got[0]); !slices.Equal(s, tc.want) {
				t.Fatalf("reactions = %v, want %v", s, tc.want)
			}
			if ids := got[0].Reactions[0].ActorIDs; !slices.Equal(ids, []string{bob.ID, me.ID}) {
				t.Fatalf("thumbs up by %v, want bob then me", ids)
			}
		})
	}

	after, err := l.RemoveCommentReaction(bobC, c.ID, core.ReactionRocket)
	if err != nil {
		t.Fatal(err)
	}
	if s := summary(after); !slices.Equal(s, []string{thumbs + "thumbs_up* 2", rocket + "rocket 1"}) {
		t.Fatalf("after bob took back his rocket, bob sees %v", s)
	}
}

func TestReactionIsIdempotentRecordedOnceAndNotifiesNobody(t *testing.T) {
	l, ctx, scope, _, _ := newTaskFixture(t)
	bob := taskContext(seedTaskActor(t, l, scope, "bob", core.RoleMember, core.ScopeAll))
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "x"})
	if err := l.StarTask(bob, core.TaskRef{ID: task.ID}); err != nil {
		t.Fatal(err)
	}
	c, err := l.AddComment(ctx, core.TaskRef{ID: task.ID}, "ship it")
	if err != nil {
		t.Fatal(err)
	}
	inboxBefore := inboxCount(t, l, bob)

	steps := []struct {
		name  string
		call  func(context.Context, string, core.Reaction) (*core.Comment, error)
		write bool
		want  core.EventType
	}{
		{"react", l.AddCommentReaction, true, core.EventCommentReacted},
		{"react again", l.AddCommentReaction, false, ""},
		{"unreact", l.RemoveCommentReaction, true, core.EventCommentUnreacted},
		{"unreact again", l.RemoveCommentReaction, false, ""},
	}
	for _, step := range steps {
		events, audits := countRows(t, l, scope)
		if _, err := step.call(ctx, c.ID, core.ReactionHooray); err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}
		afterEvents, afterAudits := countRows(t, l, scope)
		wrote := afterEvents == events+1 && afterAudits == audits+1
		silent := afterEvents == events && afterAudits == audits
		if step.write && !wrote || !step.write && !silent {
			t.Fatalf("%s: events %d -> %d, audits %d -> %d, want a write = %v",
				step.name, events, afterEvents, audits, afterAudits, step.write)
		}
		if !step.write {
			continue
		}
		if err := l.store.View(context.Background(), scope, func(tx store.Tx) error {
			evs, err := tx.ReadEvents(context.Background(), 0, 1000)
			if err != nil {
				return err
			}
			last := evs[len(evs)-1]
			if last.Type != step.want || last.SubjectID != c.ID || last.Payload["ref"] != task.Ref ||
				last.Payload["comment_id"] != c.ID || last.Payload["reaction"] != string(core.ReactionHooray) {
				t.Errorf("%s: event = %s %s %v", step.name, last.Type, last.SubjectID, last.Payload)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	if got := inboxCount(t, l, bob); got != inboxBefore {
		t.Fatalf("a reaction reached the inbox of a reader who starred the task: %d -> %d", inboxBefore, got)
	}
}

func inboxCount(t *testing.T, l *Local, ctx context.Context) int {
	t.Helper()
	items, _, err := l.ListInbox(ctx, core.InboxFilter{Page: core.Page{Limit: 100}})
	if err != nil {
		t.Fatalf("reading the inbox: %v", err)
	}
	return len(items)
}

func TestReactionRefusals(t *testing.T) {
	l, ctx, scope, _, _ := newTaskFixture(t)
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "x"})
	c, err := l.AddComment(ctx, core.TaskRef{ID: task.ID}, "ship it")
	if err != nil {
		t.Fatal(err)
	}
	gone, err := l.AddComment(ctx, core.TaskRef{ID: task.ID}, "never mind")
	if err != nil {
		t.Fatal(err)
	}
	if err := l.DeleteComment(ctx, gone.ID); err != nil {
		t.Fatal(err)
	}
	viewer := taskContext(seedTaskActor(t, l, scope, "vee", core.RoleViewer, core.ScopeTaskRead))
	other := seedTaskProject(t, l, scope, "ops")
	pinned := taskContext(&core.Actor{ID: "pinned", TenantID: scope.TenantID, Kind: core.ActorAgent,
		ProjectID: other.ID, Scopes: []core.Scope{core.ScopeAll}})

	cases := []struct {
		name string
		ctx  context.Context
		id   string
		r    core.Reaction
		want core.Kind
	}{
		{"an unknown reaction", ctx, c.ID, "party_parrot", core.KindInvalid},
		{"a deleted comment", ctx, gone.ID, core.ReactionHeart, core.KindNotFound},
		{"no such comment", ctx, "nope", core.ReactionHeart, core.KindNotFound},
		{"a reader without comment:write", viewer, c.ID, core.ReactionHeart, core.KindForbidden},
		{"a token pinned to another project", pinned, c.ID, core.ReactionHeart, core.KindForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := l.AddCommentReaction(tc.ctx, tc.id, tc.r); !core.IsKind(err, tc.want) {
				t.Fatalf("react: %v, want %s", err, tc.want)
			}
			if _, err := l.RemoveCommentReaction(tc.ctx, tc.id, tc.r); !core.IsKind(err, tc.want) {
				t.Fatalf("unreact: %v, want %s", err, tc.want)
			}
		})
	}
}

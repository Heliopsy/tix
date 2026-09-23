// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestCommentsAddEditListAndDelete(t *testing.T) {
	l, ctx, scope, actor, _ := newTaskFixture(t)
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "discussed"})
	ref := core.TaskRef{ID: task.ID}

	beforeEvents, beforeAudits := countRows(t, l, scope)
	first, err := l.AddComment(ctx, ref, "  first  ")
	if err != nil {
		t.Fatalf("AddComment: %v", err)
	}
	afterEvents, afterAudits := countRows(t, l, scope)
	if afterEvents != beforeEvents+1 || afterAudits != beforeAudits+1 {
		t.Errorf("adding a comment wrote %d events and %d audit entries, want one of each",
			afterEvents-beforeEvents, afterAudits-beforeAudits)
	}
	if first.Body != "first" || first.AuthorActorID != actor.ID || first.CreatedAt.IsZero() {
		t.Errorf("comment = %+v", first)
	}

	second, err := l.AddComment(ctx, ref, "second")
	if err != nil {
		t.Fatalf("AddComment: %v", err)
	}

	edited, err := l.EditComment(ctx, first.ID, "first, revised")
	if err != nil {
		t.Fatalf("EditComment: %v", err)
	}
	if edited.Body != "first, revised" {
		t.Errorf("body = %q", edited.Body)
	}
	if !edited.UpdatedAt.After(edited.CreatedAt) && !edited.UpdatedAt.Equal(edited.CreatedAt) {
		t.Errorf("an edited comment must carry an update time: %+v", edited)
	}

	thread, err := l.ListComments(ctx, ref)
	if err != nil {
		t.Fatalf("ListComments: %v", err)
	}
	if len(thread) != 2 || thread[0].ID != first.ID || thread[1].ID != second.ID {
		t.Fatalf("thread = %+v, want both comments oldest first", thread)
	}

	if err := l.DeleteComment(ctx, first.ID); err != nil {
		t.Fatalf("DeleteComment: %v", err)
	}
	thread, err = l.ListComments(ctx, ref)
	if err != nil {
		t.Fatalf("ListComments: %v", err)
	}
	if len(thread) != 1 || thread[0].ID != second.ID {
		t.Errorf("thread = %+v, want only the surviving comment", thread)
	}
}

func TestCommentOperationsRejectBadInput(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "t"})
	ref := core.TaskRef{ID: task.ID}
	c, err := l.AddComment(ctx, ref, "hello")
	if err != nil {
		t.Fatalf("AddComment: %v", err)
	}

	if _, err := l.AddComment(ctx, ref, "   "); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("empty comment = %v, want invalid", err)
	}
	if _, err := l.AddComment(ctx, ref, strings.Repeat("x", core.MaxCommentLength+1)); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("oversized comment = %v, want invalid", err)
	}
	if _, err := l.AddComment(ctx, core.MustParseTaskRef("infra-99"), "hi"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("comment on a missing task = %v, want not found", err)
	}
	if _, err := l.EditComment(ctx, "missingcomment01", "x"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("editing a missing comment = %v, want not found", err)
	}
	if err := l.DeleteComment(ctx, "missingcomment01"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("deleting a missing comment = %v, want not found", err)
	}
	if err := l.DeleteComment(ctx, c.ID); err != nil {
		t.Fatalf("DeleteComment: %v", err)
	}
	if _, err := l.EditComment(ctx, c.ID, "x"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("editing a deleted comment = %v, want not found", err)
	}

	if _, err := l.AddComment(t.Context(), ref, "x"); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("AddComment unauthenticated = %v", err)
	}
	if _, err := l.ListComments(t.Context(), ref); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("ListComments unauthenticated = %v", err)
	}
	if _, err := l.EditComment(t.Context(), c.ID, "x"); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("EditComment unauthenticated = %v", err)
	}
	if err := l.DeleteComment(t.Context(), c.ID); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("DeleteComment unauthenticated = %v", err)
	}
}

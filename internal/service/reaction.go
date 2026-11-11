// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// AddCommentReaction gives the caller's reaction to a comment. A reaction the
// caller already gave is left as it is, with nothing written.
func (l *Local) AddCommentReaction(ctx context.Context, id string, reaction core.Reaction) (*core.Comment, error) {
	return l.setReaction(ctx, id, reaction, true)
}

// RemoveCommentReaction takes back the caller's reaction on a comment. A
// reaction the caller never gave is left as it is, with nothing written.
func (l *Local) RemoveCommentReaction(ctx context.Context, id string, reaction core.Reaction) (*core.Comment, error) {
	return l.setReaction(ctx, id, reaction, false)
}

func (l *Local) setReaction(ctx context.Context, id string, reaction core.Reaction, on bool) (*core.Comment, error) {
	actor, err := l.authorize(ctx, authz.ActionCommentWrite, authz.Resource{})
	if err != nil {
		return nil, err
	}
	r, err := core.ParseReaction(string(reaction))
	if err != nil {
		return nil, err
	}
	var out *core.Comment
	err = l.write(ctx, actor, func(m *mutation) error {
		c, task, err := liveComment(ctx, m.tx, id)
		if err != nil {
			return err
		}
		if err := l.authorizeTask(ctx, authz.ActionCommentWrite, task); err != nil {
			return err
		}
		change, action, typ := m.tx.RemoveCommentReaction, "comment.unreact", core.EventCommentUnreacted
		if on {
			change, action, typ = m.tx.AddCommentReaction, "comment.react", core.EventCommentReacted
		}
		changed, err := change(ctx, c.ID, actor.ID, r)
		if err != nil {
			return err
		}
		out = c
		if changed {
			row := map[string]any{"comment_id": c.ID, "actor_id": actor.ID, "reaction": r}
			before, after := any(nil), any(row)
			if !on {
				before, after = row, nil
			}
			if err := m.Record(action, typ, "comment", c.ID, task.ProjectID, before, after,
				map[string]any{"ref": task.Ref, "task_id": task.ID, "comment_id": c.ID, "reaction": r}); err != nil {
				return err
			}
		}
		return markReactions(ctx, m.tx, actor.ID, []*core.Comment{c})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// liveComment reads a comment that is not deleted, with its task.
func liveComment(ctx context.Context, tx store.Tx, id string) (*core.Comment, *core.Task, error) {
	c, err := tx.GetComment(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if c.DeletedAt != nil {
		return nil, nil, core.NotFound("comment %q", id)
	}
	task, err := tx.GetTask(ctx, core.TaskRef{ID: c.TaskID})
	if err != nil {
		return nil, nil, err
	}
	return c, task, nil
}

// markReactions sums up each comment's reactions for the reader.
func markReactions(ctx context.Context, tx store.Tx, readerID string, comments []*core.Comment) error {
	if len(comments) == 0 {
		return nil
	}
	ids := make([]string, len(comments))
	for i, c := range comments {
		ids[i] = c.ID
	}
	rows, err := tx.ListCommentReactions(ctx, ids)
	if err != nil {
		return err
	}
	byComment := map[string][]core.CommentReaction{}
	for _, row := range rows {
		byComment[row.CommentID] = append(byComment[row.CommentID], row)
	}
	for _, c := range comments {
		c.Reactions = core.SummarizeReactions(byComment[c.ID], readerID)
	}
	return nil
}

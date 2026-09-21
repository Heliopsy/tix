package service

import (
	"context"
	"strings"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// AddComment records a comment on a task.
func (l *Local) AddComment(ctx context.Context, ref core.TaskRef, body string) (*core.Comment, error) {
	text, err := commentBody(body)
	if err != nil {
		return nil, err
	}
	actor, err := l.authorize(ctx, authz.ActionCommentWrite, authz.Resource{})
	if err != nil {
		return nil, err
	}

	var out *core.Comment
	err = l.write(ctx, actor, func(m *mutation) error {
		task, err := liveTask(ctx, m.tx, ref)
		if err != nil {
			return err
		}
		if err := l.authorizeTask(ctx, authz.ActionCommentWrite, task); err != nil {
			return err
		}
		c := &core.Comment{TaskID: task.ID, AuthorActorID: actor.ID, Body: text}
		if err := m.tx.CreateComment(ctx, c); err != nil {
			return err
		}
		out = c
		return m.Record("comment.add", core.EventCommentAdded, "comment", c.ID, task.ProjectID,
			nil, c, map[string]any{"ref": task.Ref, "task_id": task.ID})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ListComments returns a task's live comments oldest first.
func (l *Local) ListComments(ctx context.Context, ref core.TaskRef) ([]core.Comment, error) {
	actor, err := l.authorize(ctx, authz.ActionTaskRead, authz.Resource{})
	if err != nil {
		return nil, err
	}
	var out []core.Comment
	err = l.read(ctx, actor, func(tx store.Tx) error {
		task, err := liveTask(ctx, tx, ref)
		if err != nil {
			return err
		}
		if err := l.authorizeTask(ctx, authz.ActionTaskRead, task); err != nil {
			return err
		}
		out, err = tx.ListComments(ctx, task.ID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// EditComment rewrites a comment's body, recording that it was edited.
func (l *Local) EditComment(ctx context.Context, id, body string) (*core.Comment, error) {
	text, err := commentBody(body)
	if err != nil {
		return nil, err
	}
	actor, err := l.authorize(ctx, authz.ActionCommentWrite, authz.Resource{})
	if err != nil {
		return nil, err
	}

	var out *core.Comment
	err = l.write(ctx, actor, func(m *mutation) error {
		c, err := m.tx.GetComment(ctx, id)
		if err != nil {
			return err
		}
		if c.DeletedAt != nil {
			return core.NotFound("comment %q", id)
		}
		task, err := m.tx.GetTask(ctx, core.TaskRef{ID: c.TaskID})
		if err != nil {
			return err
		}
		if err := l.authorizeTask(ctx, authz.ActionCommentWrite, task); err != nil {
			return err
		}
		before := *c
		c.Body = text
		if err := m.tx.UpdateComment(ctx, c); err != nil {
			return err
		}
		out = c
		return m.Record("comment.edit", core.EventCommentAdded, "comment", c.ID, task.ProjectID,
			before, c, map[string]any{"ref": task.Ref, "task_id": task.ID, "edited": true})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteComment soft deletes a comment.
func (l *Local) DeleteComment(ctx context.Context, id string) error {
	actor, err := l.authorize(ctx, authz.ActionCommentWrite, authz.Resource{})
	if err != nil {
		return err
	}
	return l.write(ctx, actor, func(m *mutation) error {
		c, err := m.tx.GetComment(ctx, id)
		if err != nil {
			return err
		}
		task, err := m.tx.GetTask(ctx, core.TaskRef{ID: c.TaskID})
		if err != nil {
			return err
		}
		if err := l.authorizeTask(ctx, authz.ActionCommentWrite, task); err != nil {
			return err
		}
		if err := m.tx.DeleteComment(ctx, c.ID); err != nil {
			return err
		}
		return m.Record("comment.delete", core.EventTaskUpdated, "comment", c.ID, task.ProjectID,
			c, nil, map[string]any{"ref": task.Ref, "task_id": task.ID, "deleted": true})
	})
}

// commentBody trims a comment and enforces its length.
func commentBody(body string) (string, error) {
	text := strings.TrimSpace(body)
	if text == "" {
		return "", core.Invalid("comment body is required")
	}
	if len(text) > core.MaxCommentLength {
		return "", core.Invalid("comment must be at most %d characters", core.MaxCommentLength)
	}
	return text, nil
}

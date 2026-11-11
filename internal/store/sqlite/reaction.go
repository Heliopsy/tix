// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"

	"github.com/heliopsy/tix/internal/core"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

// AddReaction records an actor's emoji on a comment, reporting whether it was
// new. It reads before it writes, as StarTask does, so a repeat never aborts
// the transaction on PostgreSQL.
func (t *tx) AddReaction(ctx context.Context, commentID, actorID, emoji string) (bool, error) {
	n, err := t.count(ctx, t.reactionRow(commentID, actorID, emoji), "reading reaction on comment %q", commentID)
	if err != nil {
		return false, err
	}
	if n > 0 {
		return false, nil
	}
	ins := t.insert("comment_reactions").
		Set("comment_id", commentID).
		Set("actor_id", actorID).
		Set("emoji", emoji).
		Set("created_at", t.now())
	if _, err := t.execInsert(ctx, ins, "reacting to comment %q", commentID); err != nil {
		return false, err
	}
	return true, nil
}

// RemoveReaction removes an actor's emoji on a comment, reporting whether there was one.
func (t *tx) RemoveReaction(ctx context.Context, commentID, actorID, emoji string) (bool, error) {
	n, err := t.execDelete(ctx, t.reactionRow(commentID, actorID, emoji), "removing reaction on comment %q", commentID)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// ClearReactions removes every reaction on a comment.
func (t *tx) ClearReactions(ctx context.Context, commentID string) error {
	b := t.builder("comment_reactions").Where("comment_reactions.comment_id = ?", commentID)
	_, err := t.execDelete(ctx, b, "clearing reactions on comment %q", commentID)
	return err
}

// ListReactions returns the reactions on the given comments, oldest first.
func (t *tx) ListReactions(ctx context.Context, commentIDs []string) ([]core.CommentReaction, error) {
	out := []core.CommentReaction{}
	if len(commentIDs) == 0 {
		return out, nil
	}
	b := t.builder("comment_reactions").
		Select("comment_reactions.comment_id", "comment_reactions.actor_id", "comment_reactions.emoji").
		OrderBy("comment_reactions.created_at", core.Ascending).
		OrderBy("comment_reactions.actor_id", core.Ascending).
		OrderBy("comment_reactions.emoji", core.Ascending)
	b.WhereIn("comment_reactions.comment_id", commentIDs)
	rows, err := t.query(ctx, b, "listing reactions")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var r core.CommentReaction
		if err := rows.Scan(&r.CommentID, &r.ActorID, &r.Emoji); err != nil {
			return nil, mapErr(err, "listing reactions")
		}
		out = append(out, r)
	}
	return out, mapRowsErr(rows, "listing reactions")
}

func (t *tx) reactionRow(commentID, actorID, emoji string) *sqlb.Builder {
	return t.builder("comment_reactions").
		Where("comment_reactions.comment_id = ?", commentID).
		Where("comment_reactions.actor_id = ?", actorID).
		Where("comment_reactions.emoji = ?", emoji)
}

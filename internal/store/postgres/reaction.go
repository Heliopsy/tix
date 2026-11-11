// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"database/sql"

	"github.com/heliopsy/tix/internal/core"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

// AddCommentReaction records an actor's reaction on a comment, reporting
// whether it was new. It reads before it writes, as StarTask does, so a
// repeat never aborts the transaction on PostgreSQL.
func (t *tx) AddCommentReaction(ctx context.Context, commentID, actorID string, r core.Reaction) (bool, error) {
	n, err := t.count(ctx, t.reactionRow(commentID, actorID, r), "reading reaction on comment %q", commentID)
	if err != nil {
		return false, err
	}
	if n > 0 {
		return false, nil
	}
	ins := t.insert("comment_reactions").
		Set("comment_id", commentID).
		Set("actor_id", actorID).
		Set("reaction", string(r)).
		Set("created_at", t.now())
	if _, err := t.execInsert(ctx, ins, "reacting to comment %q", commentID); err != nil {
		return false, err
	}
	return true, nil
}

// RemoveCommentReaction removes an actor's reaction on a comment, reporting
// whether there was one.
func (t *tx) RemoveCommentReaction(ctx context.Context, commentID, actorID string, r core.Reaction) (bool, error) {
	n, err := t.execDelete(ctx, t.reactionRow(commentID, actorID, r), "removing reaction on comment %q", commentID)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// ListCommentReactions returns the reactions on the given comments, earliest
// first.
func (t *tx) ListCommentReactions(ctx context.Context, commentIDs []string) ([]core.CommentReaction, error) {
	out := []core.CommentReaction{}
	if len(commentIDs) == 0 {
		return out, nil
	}
	b := t.builder("comment_reactions").
		Select("comment_reactions.comment_id", "comment_reactions.actor_id",
			"comment_reactions.reaction", "comment_reactions.created_at").
		OrderBy("comment_reactions.created_at", core.Ascending).
		OrderBy("comment_reactions.actor_id", core.Ascending)
	b.WhereIn("comment_reactions.comment_id", commentIDs)
	rows, err := t.query(ctx, b, "listing reactions")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var (
			v        core.CommentReaction
			reaction string
			created  sql.NullTime
		)
		if err := rows.Scan(&v.CommentID, &v.ActorID, &reaction, &created); err != nil {
			return nil, mapErr(err, "scanning reaction")
		}
		v.Reaction = core.Reaction(reaction)
		v.CreatedAt = scanTime(created)
		out = append(out, v)
	}
	return out, mapRowsErr(rows, "listing reactions")
}

func (t *tx) reactionRow(commentID, actorID string, r core.Reaction) *sqlb.Builder {
	return t.builder("comment_reactions").
		Where("comment_reactions.comment_id = ?", commentID).
		Where("comment_reactions.actor_id = ?", actorID).
		Where("comment_reactions.reaction = ?", string(r))
}

// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// maxEmojiBytes bounds a reaction. The longest emoji in use, a family or a
// tagged flag, stays well inside it.
const maxEmojiBytes = 64

// AddReaction adds the caller's emoji reaction to a comment.
func (l *Local) AddReaction(ctx context.Context, id, emoji string) (*core.Comment, error) {
	return l.react(ctx, id, emoji, true)
}

// RemoveReaction removes the caller's emoji reaction from a comment.
func (l *Local) RemoveReaction(ctx context.Context, id, emoji string) (*core.Comment, error) {
	return l.react(ctx, id, emoji, false)
}

func (l *Local) react(ctx context.Context, id, emoji string, add bool) (*core.Comment, error) {
	emoji, err := reactionEmoji(emoji)
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
		task, err := liveTask(ctx, m.tx, core.TaskRef{ID: c.TaskID})
		if err != nil {
			return err
		}
		if err := l.authorizeTask(ctx, authz.ActionCommentWrite, task); err != nil {
			return err
		}
		change, action, typ := m.tx.RemoveReaction, "comment.unreact", core.EventCommentUnreacted
		if add {
			change, action, typ = m.tx.AddReaction, "comment.react", core.EventCommentReacted
		}
		changed, err := change(ctx, c.ID, actor.ID, emoji)
		if err != nil {
			return err
		}
		out = c
		if err := withReactions(ctx, m.tx, []*core.Comment{c}); err != nil {
			return err
		}
		if !changed {
			return nil
		}
		row := map[string]any{"comment_id": c.ID, "actor_id": actor.ID, "emoji": emoji}
		before, after := any(nil), any(row)
		if !add {
			before, after = row, nil
		}
		return m.Record(action, typ, "comment", c.ID, task.ProjectID, before, after,
			map[string]any{"ref": task.Ref, "task_id": task.ID, "emoji": emoji, "reacted": add})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// withReactions fills each comment's Reactions, grouping the rows by emoji in
// the order each emoji was first used.
func withReactions(ctx context.Context, tx store.Tx, comments []*core.Comment) error {
	if len(comments) == 0 {
		return nil
	}
	ids := make([]string, len(comments))
	byID := make(map[string]*core.Comment, len(comments))
	for i, c := range comments {
		ids[i] = c.ID
		byID[c.ID] = c
		c.Reactions = nil
	}
	rows, err := tx.ListReactions(ctx, ids)
	if err != nil {
		return err
	}
	for _, r := range rows {
		c := byID[r.CommentID]
		if c == nil {
			continue
		}
		i := 0
		for i < len(c.Reactions) && c.Reactions[i].Emoji != r.Emoji {
			i++
		}
		if i == len(c.Reactions) {
			c.Reactions = append(c.Reactions, core.Reaction{Emoji: r.Emoji})
		}
		c.Reactions[i].Count++
		c.Reactions[i].ActorIDs = append(c.Reactions[i].ActorIDs, r.ActorID)
	}
	return nil
}

// reactionEmoji trims a reaction and refuses anything but one emoji grapheme.
func reactionEmoji(s string) (string, error) {
	e := strings.TrimSpace(s)
	if e == "" {
		return "", core.Invalid("emoji is required")
	}
	if len(e) > maxEmojiBytes || !utf8.ValidString(e) || !isOneEmoji([]rune(e)) {
		return "", core.Invalid("reaction must be a single emoji, not %q", e)
	}
	return e, nil
}

// isOneEmoji reports whether rs is exactly one emoji grapheme: a keycap, a
// flag, a tag sequence, or emoji joined by zero-width joiners, each with an
// optional presentation selector and skin tone.
func isOneEmoji(rs []rune) bool {
	if len(rs) == 0 {
		return false
	}
	if isKeycapBase(rs[0]) {
		rest := rs[1:]
		if len(rest) > 0 && rest[0] == 0xFE0F {
			rest = rest[1:]
		}
		return len(rest) == 1 && rest[0] == 0x20E3
	}
	if isRegional(rs[0]) {
		return len(rs) == 2 && isRegional(rs[1])
	}
	if rs[0] == 0x1F3F4 && len(rs) > 1 && isTag(rs[1]) {
		for _, r := range rs[1 : len(rs)-1] {
			if !isTag(r) {
				return false
			}
		}
		return rs[len(rs)-1] == 0xE007F
	}
	i := 0
	for {
		if i >= len(rs) || !isEmojiBase(rs[i]) {
			return false
		}
		i++
		if i < len(rs) && rs[i] == 0xFE0F {
			i++
		}
		if i < len(rs) && rs[i] >= 0x1F3FB && rs[i] <= 0x1F3FF {
			i++
		}
		if i == len(rs) {
			return true
		}
		if rs[i] != 0x200D {
			return false
		}
		i++
	}
}

func isKeycapBase(r rune) bool { return r == '#' || r == '*' || (r >= '0' && r <= '9') }

func isRegional(r rune) bool { return r >= 0x1F1E6 && r <= 0x1F1FF }

func isTag(r rune) bool { return r >= 0xE0020 && r <= 0xE007E }

func isEmojiBase(r rune) bool {
	switch {
	case r >= 0x1F000 && r <= 0x1FAFF && (r < 0x1F3FB || r > 0x1F3FF):
		return true
	case r >= 0x2190 && r <= 0x21FF, r >= 0x2300 && r <= 0x23FF,
		r >= 0x25A0 && r <= 0x27BF, r >= 0x2900 && r <= 0x297F,
		r >= 0x2B00 && r <= 0x2BFF:
		return true
	}
	switch r {
	case 0xA9, 0xAE, 0x203C, 0x2049, 0x2122, 0x2139, 0x24C2, 0x3030, 0x303D, 0x3297, 0x3299:
		return true
	}
	return false
}

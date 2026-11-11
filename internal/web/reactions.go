// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import "github.com/heliopsy/tix/internal/core"

// reactionChoice is one reaction as a comment's picker offers it.
type reactionChoice struct {
	core.ReactionSummary
	// Label is the reaction's name as words, for its accessible name.
	Label string
}

// reactionLabels name each reaction for a screen reader.
var reactionLabels = map[core.Reaction]string{
	core.ReactionThumbsUp:   "Thumbs up",
	core.ReactionThumbsDown: "Thumbs down",
	core.ReactionLaugh:      "Laugh",
	core.ReactionHooray:     "Hooray",
	core.ReactionConfused:   "Confused",
	core.ReactionHeart:      "Heart",
	core.ReactionRocket:     "Rocket",
	core.ReactionEyes:       "Eyes",
}

// reactionChoices lists every reaction for one comment in offer order, each
// with the comment's count and whether the reader gave it.
func reactionChoices(c core.Comment) []reactionChoice {
	given := map[core.Reaction]core.ReactionSummary{}
	for _, s := range c.Reactions {
		given[s.Reaction] = s
	}
	out := make([]reactionChoice, 0, len(core.Reactions()))
	for _, r := range core.Reactions() {
		s, ok := given[r]
		if !ok {
			s = core.ReactionSummary{Reaction: r, Emoji: r.Emoji()}
		}
		out = append(out, reactionChoice{ReactionSummary: s, Label: reactionLabels[r]})
	}
	return out
}

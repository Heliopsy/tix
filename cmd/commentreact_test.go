// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func decodeComment(t *testing.T, out string) core.Comment {
	t.Helper()
	var c core.Comment
	if err := json.Unmarshal([]byte(out), &c); err != nil {
		t.Fatalf("not json: %v\n%s", err, out)
	}
	return c
}

func TestCommentReactTogglesAndShowsInTheListing(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "reacted to")
	id := decodeComment(t, c.mustRun("comment", "add", "default-1", "ship it", "-o", "json").out).ID

	c.mustRun("comment", "react", id, "rocket", "--dry-run")
	if got := decodeComment(t, c.mustRun("comment", "react", id, "heart", "-o", "json").out); len(got.Reactions) != 1 ||
		got.Reactions[0].Reaction != core.ReactionHeart || !got.Reactions[0].Reacted || got.Reactions[0].Count != 1 {
		t.Fatalf("react answered %+v; the dry run must not have reacted", got.Reactions)
	}
	c.mustRun("comment", "react", id, core.ReactionRocket.Emoji())
	table := c.mustRun("comment", "ls", "default-1").out
	if !strings.Contains(table, "REACTIONS") || !strings.Contains(table, core.ReactionHeart.Emoji()+" 1") ||
		!strings.Contains(table, core.ReactionRocket.Emoji()+" 1") {
		t.Fatalf("the listing does not show the reactions:\n%s", table)
	}

	c.mustRun("comment", "unreact", id, "heart")
	c.mustRun("comment", "unreact", id, "heart")
	if got := decodeComment(t, c.mustRun("comment", "unreact", id, "rocket", "-o", "json").out); len(got.Reactions) != 0 {
		t.Fatalf("after taking both back the comment carries %+v", got.Reactions)
	}
	if got := c.run("comment", "react", id, "party_parrot"); got.code != core.ExitUsage {
		t.Fatalf("an unknown reaction exited %d, want %d", got.code, core.ExitUsage)
	}
	if got := c.run("comment", "react", "01JJJJJJJJJJJJJJJJJJJJJJJJ", "heart"); got.code != core.ExitNotFound {
		t.Fatalf("an unknown comment exited %d, want %d", got.code, core.ExitNotFound)
	}
}

// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/store/sqlite"
)

// seedInbox puts n items in the command line's own inbox, caused by another
// actor, the way a comment from somebody else would.
func seedInbox(t *testing.T, c *cli, n int) {
	t.Helper()
	var task core.Task
	if err := json.Unmarshal([]byte(c.mustRun("task", "add", "inbox target", "-o", "json").out), &task); err != nil {
		t.Fatal(err)
	}
	s, err := sqlite.Open(c.dbPath(), clock.Real{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()
	scope := core.TenantScope{TenantID: task.TenantID}
	if err := s.Update(ctx, scope, func(tx store.Tx) error {
		other := core.Actor{Kind: core.ActorUser, Handle: "somebody"}
		if err := tx.CreateActor(ctx, &other); err != nil {
			return err
		}
		for range n {
			if err := tx.CreateInboxItem(ctx, &core.InboxItem{RecipientID: task.CreatorActorID,
				Kind: core.InboxMention, TaskID: task.ID, ActorID: other.ID, Excerpt: "@you look"}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func inboxItems(t *testing.T, c *cli, args ...string) []core.InboxItem {
	t.Helper()
	var items []core.InboxItem
	got := c.mustRun(append([]string{"inbox", "ls", "-o", "json"}, args...)...)
	if err := json.Unmarshal([]byte(got.out), &items); err != nil {
		t.Fatalf("not json: %v\n%s", err, got.out)
	}
	return items
}

func unreadCount(t *testing.T, c *cli) int {
	t.Helper()
	var n core.InboxCount
	if err := json.Unmarshal([]byte(c.mustRun("inbox", "count", "-o", "json").out), &n); err != nil {
		t.Fatal(err)
	}
	return n.Unread
}

func TestInboxCommandsListMarkAndMute(t *testing.T) {
	c := newCLI(t)
	seedInbox(t, c, 2)
	items := inboxItems(t, c)
	if len(items) != 2 || items[0].ActorHandle != "somebody" || items[0].TaskTitle != "inbox target" {
		t.Fatalf("items = %+v", items)
	}
	table := c.mustRun("inbox", "ls").out
	if !strings.Contains(table, "mention") || !strings.Contains(table, "somebody") {
		t.Fatalf("table names neither the kind nor the actor:\n%s", table)
	}

	c.mustRun("inbox", "read", items[0].ID, "--dry-run")
	if unreadCount(t, c) != 2 {
		t.Fatal("a dry run marked an item read")
	}
	c.mustRun("inbox", "read", items[0].ID)
	if got := inboxItems(t, c, "--unread"); len(got) != 1 || got[0].ID != items[1].ID {
		t.Fatalf("unread after reading one = %+v", got)
	}
	c.mustRun("inbox", "unread", items[0].ID)
	if unreadCount(t, c) != 2 {
		t.Fatal("unread did not mark the item unread")
	}
	c.mustRun("inbox", "read-all")
	if unreadCount(t, c) != 0 {
		t.Fatal("read-all left items unread")
	}

	c.mustRun("inbox", "prefs", "--mute", "comment,STATUS")
	c.mustRun("inbox", "prefs", "--unmute", "status")
	var prefs core.InboxPrefs
	if err := json.Unmarshal([]byte(c.mustRun("inbox", "prefs", "-o", "json").out), &prefs); err != nil {
		t.Fatal(err)
	}
	if len(prefs.Muted) != 1 || prefs.Muted[0] != core.InboxComment {
		t.Fatalf("muted = %v, want comment", prefs.Muted)
	}

	cases := []struct {
		name string
		args []string
		code int
	}{
		{"unknown kind", []string{"inbox", "prefs", "--mute", "everything"}, core.ExitUsage},
		{"unknown item", []string{"inbox", "read", "01J000000000000000000A"}, core.ExitNotFound},
		{"no item", []string{"inbox", "unread"}, core.ExitUsage},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := c.run(tc.args...); got.code != tc.code {
				t.Fatalf("exited %d, want %d\n%s", got.code, tc.code, got.err)
			}
		})
	}
}

func TestInboxChannelCommands(t *testing.T) {
	c := newCLI(t)
	var issued struct {
		core.InboxChannel
		Secret string `json:"secret"`
	}
	got := c.mustRun("inbox", "channel", "add", "phone", "https://ntfy.invalid/me-tix", "--format", "ntfy", "--kinds", "mention,status", "-o", "json")
	if err := json.Unmarshal([]byte(got.out), &issued); err != nil || issued.Secret == "" || len(issued.Kinds) != 2 {
		t.Fatalf("add = %s, %v", got.out, err)
	}
	if out := c.mustRun("inbox", "channel", "ls").out; !strings.Contains(out, "phone") || strings.Contains(out, issued.Secret) {
		t.Fatalf("ls = %s", out)
	}
	c.mustRun("inbox", "channel", "edit", issued.ID, "--name", "pager", "--active=false")
	var channels []core.InboxChannel
	if err := json.Unmarshal([]byte(c.mustRun("inbox", "channel", "ls", "-o", "json").out), &channels); err != nil {
		t.Fatal(err)
	}
	if len(channels) != 1 || channels[0].Name != "pager" || channels[0].Active {
		t.Fatalf("after edit = %+v", channels)
	}
	if got := c.run("inbox", "channel", "test", issued.ID); got.code != core.ExitPrecondtion {
		t.Fatalf("testing a channel turned off exited %d\n%s", got.code, got.err)
	}
	c.mustRun("inbox", "channel", "edit", issued.ID, "--active")
	var d core.ChannelDelivery
	if err := json.Unmarshal([]byte(c.mustRun("inbox", "channel", "test", issued.ID, "-o", "json").out), &d); err != nil || !d.Test() {
		t.Fatalf("test = %+v, %v", d, err)
	}
	if out := c.mustRun("inbox", "channel", "deliveries", issued.ID).out; !strings.Contains(out, "test") {
		t.Fatalf("deliveries = %s", out)
	}
	c.mustRun("inbox", "channel", "redeliver", d.ID)
	c.mustRun("inbox", "channel", "rm", issued.ID)
	cases := []struct {
		name string
		args []string
		code int
	}{
		{"unknown format", []string{"inbox", "channel", "add", "x", "https://hooks.invalid/h", "--format", "pager"}, core.ExitUsage},
		{"nothing to change", []string{"inbox", "channel", "edit", issued.ID}, core.ExitUsage},
		{"a removed channel", []string{"inbox", "channel", "rm", issued.ID}, core.ExitNotFound},
	}
	for _, tc := range cases {
		if got := c.run(tc.args...); got.code != tc.code {
			t.Errorf("%s: exited %d, want %d\n%s", tc.name, got.code, tc.code, got.err)
		}
	}
}

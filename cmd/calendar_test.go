// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// memberToken makes a member and a token acting as them, since a feed reads as
// a member and the zero-config local actor holds no membership.
func memberToken(t *testing.T, c *cli) string {
	t.Helper()
	c.mustRun("task", "add", "seed the store")
	c.mustRun("user", "create", "bob@example.com", "--handle", "bob", "--role", "member")
	got := c.mustRun("token", "create", "bobs", "--actor", "bob", "--scope", "task:read", "-o", "json")
	var issued core.IssuedToken
	if err := json.Unmarshal([]byte(got.out), &issued); err != nil || issued.Token == "" {
		t.Fatalf("token create: %v\n%s", err, got.out)
	}
	return issued.Token
}

func TestCalendarCommandsManageTheCallersFeed(t *testing.T) {
	c := newCLI(t)
	token := memberToken(t, c)

	got := c.mustRun("calendar", "create", "--filter", "project:default", "--token", token, "-o", "json")
	var issued core.IssuedCalendarFeed
	if err := json.Unmarshal([]byte(got.out), &issued); err != nil {
		t.Fatalf("calendar create: %v\n%s", err, got.out)
	}
	if !strings.HasPrefix(issued.Token, "tix_cal_") || issued.Path != "/api/v1/calendar/feeds/"+issued.Token+".ics" {
		t.Fatalf("created %+v", issued)
	}
	if !strings.Contains(got.err, "shown once") || !strings.Contains(got.err, "local database") {
		t.Fatalf("stderr does not say the address is shown once and is a path: %q", got.err)
	}

	shown := c.mustRun("calendar", "show", "--token", token, "-o", "json")
	if strings.Contains(shown.out, issued.Token) || !strings.Contains(shown.out, `"filter": "project:default"`) {
		t.Fatalf("calendar show: %s", shown.out)
	}

	if got := c.run("calendar", "create", "--token", token); got.code != core.ExitConflict {
		t.Fatalf("a second create exited %d, want %d", got.code, core.ExitConflict)
	}
	if got := c.run("calendar", "filter", "project:nosuch", "--token", token); got.code != core.ExitNotFound {
		t.Fatalf("an unknown project in the filter exited %d, want %d", got.code, core.ExitNotFound)
	}
	c.mustRun("calendar", "filter", "--token", token)
	if shown := c.mustRun("calendar", "show", "--token", token, "-o", "json"); !strings.Contains(shown.out, `"filter": ""`) {
		t.Fatalf("filter with no expression did not restore the default: %s", shown.out)
	}

	rotated := c.mustRun("calendar", "rotate", "--token", token, "-o", "json")
	if strings.Contains(rotated.out, issued.Token) || !strings.Contains(rotated.out, "tix_cal_") {
		t.Fatalf("rotate printed %s", rotated.out)
	}
	c.mustRun("calendar", "revoke", "--token", token)
	if got := c.run("calendar", "show", "--token", token); got.code != core.ExitNotFound {
		t.Fatalf("show after revoke exited %d, want %d", got.code, core.ExitNotFound)
	}
}

func TestCalendarCreateRefusesTheLocalActor(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "seed the store")
	if got := c.run("calendar", "create"); got.code != core.ExitUsage || !strings.Contains(got.err, "membership") {
		t.Fatalf("the zero-config actor exited %d: %s", got.code, got.err)
	}
}

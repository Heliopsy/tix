// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// A local database has no web address, so the link needs --base, and every
// form the browser's Share menu copies comes out of the one command.
func TestTaskLinkOnALocalDatabase(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "Fix [the] build")

	if got := c.run("task", "link", "default-1"); got.code != core.ExitUsage || !strings.Contains(got.err, "--base") {
		t.Fatalf("a local database without --base exited %d, want %d naming --base:\n%s", got.code, core.ExitUsage, got.err)
	}
	cases := []struct{ as, want string }{
		{"url", "https://tix.example.com/tasks/default-1"},
		{"markdown", `[DEFAULT-1 Fix \[the\] build](https://tix.example.com/tasks/default-1)`},
		{"text", "DEFAULT-1 Fix [the] build"},
	}
	for _, tc := range cases {
		got := c.mustRun("task", "link", "default-1", "--base", "https://tix.example.com/", "--as", tc.as)
		if strings.TrimSpace(got.out) != tc.want {
			t.Errorf("--as %s printed %q, want %q", tc.as, got.out, tc.want)
		}
	}
	var link taskLink
	out := c.mustRun("task", "link", "default-1", "--base", "http://h:8080", "-o", "json").out
	if err := json.Unmarshal([]byte(out), &link); err != nil {
		t.Fatalf("not json: %v\n%s", err, out)
	}
	if link.URL != "http://h:8080/tasks/default-1" || link.Ref != "DEFAULT-1" || link.Text != "DEFAULT-1 Fix [the] build" ||
		!strings.HasSuffix(link.Markdown, "(http://h:8080/tasks/default-1)") {
		t.Errorf("the json carries %+v", link)
	}

	for _, args := range [][]string{
		{"task", "link", "default-1", "--base", "ftp://h"},
		{"task", "link", "default-1", "--base", "tix.example.com"},
		{"task", "link", "default-1", "--base", "https://h", "--as", "html"},
	} {
		if got := c.run(args...); got.code != core.ExitUsage {
			t.Errorf("%v exited %d, want %d", args, got.code, core.ExitUsage)
		}
	}
	if got := c.run("task", "link", "default-9", "--base", "https://h"); got.code != core.ExitNotFound {
		t.Errorf("an unknown reference exited %d, want %d", got.code, core.ExitNotFound)
	}
}

// Over a server the address is the one the context talks to.
func TestTaskLinkUsesTheContextsServer(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "renew the certificate")
	var created struct {
		Token string `json:"token"`
	}
	out := c.mustRun("token", "create", "link", "--scope", "task:read", "-o", "json").out
	if err := json.Unmarshal([]byte(out), &created); err != nil {
		t.Fatalf("token create: %v (%s)", err, out)
	}
	url := c.loginServer()
	got := c.mustRun("--server", url+"/", "--token", created.Token, "task", "link", "default-1")
	if want := strings.TrimRight(url, "/") + "/tasks/default-1"; strings.TrimSpace(got.out) != want {
		t.Fatalf("printed %q, want %q", got.out, want)
	}
}

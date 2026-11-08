// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/service"
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

// A local database with server.public_url configured has a web address, so
// the link needs no --base, and --base still wins over it.
func TestTaskLinkOnALocalDatabaseWithAPublicAddress(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "renew the certificate")
	env := append(c.environ(), "TIX_SERVER_PUBLIC_URL=https://tix.example.com/")
	runWith := func(args ...string) result {
		var out, errb bytes.Buffer
		code := Run(args, strings.NewReader(""), &out, &errb, env, c.home)
		return result{code: code, out: out.String(), err: errb.String()}
	}
	if got := runWith("task", "link", "default-1"); got.code != 0 || strings.TrimSpace(got.out) != "https://tix.example.com/tasks/default-1" {
		t.Fatalf("with server.public_url: exit %d, out %q, err %q", got.code, got.out, got.err)
	}
	if got := runWith("task", "link", "default-1", "--base", "http://h:8080"); strings.TrimSpace(got.out) != "http://h:8080/tasks/default-1" {
		t.Fatalf("--base under server.public_url printed %q", got.out)
	}
	if got := runWith("task", "show", "default-1", "-o", "json"); !strings.Contains(got.out, `"web_url": "https://tix.example.com/tasks/default-1"`) {
		t.Fatalf("task show -o json carries no web_url:\n%s", got.out)
	}
	if got := c.mustRun("task", "show", "default-1", "-o", "json"); strings.Contains(got.out, "web_url") {
		t.Fatalf("without server.public_url task show carries web_url:\n%s", got.out)
	}
	var bad bytes.Buffer
	if code := Run([]string{"task", "ls"}, strings.NewReader(""), &bad, &bad,
		append(c.environ(), "TIX_SERVER_PUBLIC_URL=https://tix.example.com/?tenant=x"), c.home); code != core.ExitUsage ||
		!strings.Contains(bad.String(), "server.public_url") {
		t.Fatalf("a public address with a query: exit %d, %s", code, bad.String())
	}
}

// Over a server that knows its public address, the link is the task's
// web_url, not the address the context happens to reach the server at.
func TestTaskLinkPrefersTheServersPublicAddress(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "renew the certificate")
	var created struct {
		Token string `json:"token"`
	}
	out := c.mustRun("token", "create", "link", "--scope", "task:read", "-o", "json").out
	if err := json.Unmarshal([]byte(out), &created); err != nil {
		t.Fatalf("token create: %v (%s)", err, out)
	}
	url := c.loginServerWith(service.WithPublicURL("https://tix.example.com"))
	got := c.mustRun("--server", url, "--token", created.Token, "task", "link", "default-1", "--as", "markdown")
	if want := "[DEFAULT-1 renew the certificate](https://tix.example.com/tasks/default-1)"; strings.TrimSpace(got.out) != want {
		t.Fatalf("printed %q, want %q", got.out, want)
	}
}

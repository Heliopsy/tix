// SPDX-License-Identifier: AGPL-3.0-or-later

package sshd_test

import (
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/sshd"
)

// tix task link runs over a real session and prints the task's web_url, the
// server's public address, in each form the Share menu copies.
func TestTaskLinkRunsOverSSHWithAPublicAddress(t *testing.T) {
	l := newHostedListenerWith(t, nil, service.WithPublicURL("https://tix.example.com/"))
	p := sshd.SeedPerson(t, l.st, "acme", "ada", "ada@example.test")
	sshd.SeedBoard(t, l.st, l.svc, p)
	client := dial(t, l.addr, "acme", p.Signer)
	if r := run(t, client, `task add "Fix [the] build" -p tob`, nil, nil); r.code != 0 {
		t.Fatalf("task add exited %d: %s", r.code, r.err)
	}

	cases := []struct{ as, want string }{
		{"url", "https://tix.example.com/tasks/tob-1"},
		{"markdown", `[TOB-1 Fix \[the\] build](https://tix.example.com/tasks/tob-1)`},
		{"text", "TOB-1 Fix [the] build"},
	}
	for _, tc := range cases {
		r := run(t, client, "task link TOB-1 --as "+tc.as, nil, nil)
		if r.code != 0 || strings.TrimSpace(r.out) != tc.want {
			t.Errorf("task link --as %s: exit %d, out %q, err %q; want %q", tc.as, r.code, r.out, r.err, tc.want)
		}
	}
	shown := run(t, client, "task show TOB-1 -o json", nil, nil)
	if !strings.Contains(shown.out, `"web_url": "https://tix.example.com/tasks/tob-1"`) {
		t.Errorf("task show over ssh carries no web_url: %s", shown.out)
	}

	base := run(t, client, "task link TOB-1 --base https://evil.example", nil, nil)
	if base.code != core.ExitUsage || !strings.Contains(base.err, "--base") || base.out != "" {
		t.Errorf("--base over ssh: exit %d, out %q, err %q; want %d naming --base", base.code, base.out, base.err, core.ExitUsage)
	}
	missing := run(t, client, "task link TOB-99", nil, nil)
	if missing.code != core.ExitNotFound {
		t.Errorf("an unknown reference over ssh exited %d, want %d", missing.code, core.ExitNotFound)
	}
}

// Without server.public_url the server knows no web address, so the command
// refuses cleanly and names the key rather than printing a guess.
func TestTaskLinkOverSSHWithoutAPublicAddressIsRefused(t *testing.T) {
	_, _, client := enrolled(t, nil)
	if r := run(t, client, "task add plain -p tob", nil, nil); r.code != 0 {
		t.Fatalf("task add exited %d: %s", r.code, r.err)
	}
	r := run(t, client, "task link TOB-1", nil, nil)
	if r.code != core.ExitUsage || !strings.Contains(r.err, "server.public_url") || !strings.Contains(r.err, "operator") || r.out != "" {
		t.Fatalf("task link without a public address: exit %d, out %q, err %q", r.code, r.out, r.err)
	}
	shown := run(t, client, "task show TOB-1 -o json", nil, nil)
	if strings.Contains(shown.out, "web_url") {
		t.Errorf("a server without a public address set web_url: %s", shown.out)
	}
}

// A tenant served on a domain of its own links under that domain, with the
// public address's scheme and path, even from an ssh session.
func TestTaskLinkOverSSHUsesTheTenantsOwnDomain(t *testing.T) {
	l := newHostedListenerWith(t, nil, service.WithPublicURL("https://tix.example.com/tix"))
	p := sshd.SeedPerson(t, l.st, "acme", "ada", "ada@example.test")
	sshd.SeedBoard(t, l.st, l.svc, p)
	if _, err := l.svc.AddDomain(sshd.SessionContext(p.Actor), core.AddDomainInput{Hostname: "tix.acme.example"}); err != nil {
		t.Fatalf("adding the domain: %v", err)
	}
	client := dial(t, l.addr, "acme", p.Signer)
	if r := run(t, client, "task add domained -p tob", nil, nil); r.code != 0 {
		t.Fatalf("task add exited %d: %s", r.code, r.err)
	}
	r := run(t, client, "task link TOB-1", nil, nil)
	if want := "https://tix.acme.example/tix/tasks/tob-1"; r.code != 0 || strings.TrimSpace(r.out) != want {
		t.Fatalf("task link on a tenant with its own domain: exit %d, out %q, err %q; want %q", r.code, r.out, r.err, want)
	}
}

// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"maps"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestTenantEditSessionFlagsAndSessionPolicy(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "seed")
	c.mustRun("tenant", "edit", "default", "--session-ttl", "8h", "--session-idle-timeout", "30m",
		"--session-remember-ttl", "off")

	var p core.SessionPolicy
	out := c.mustRun("tenant", "session-policy", "-o", "json").out
	if err := json.Unmarshal([]byte(out), &p); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if p.TTL.String() != "8h0m0s" || p.IdleTimeout.String() != "30m0s" || p.RememberTTL != 0 {
		t.Fatalf("policy = %+v", p)
	}

	c.mustRun("tenant", "edit", "default", "--session-ttl", "", "--session-idle-timeout", "",
		"--session-remember-ttl", "")
	out = c.mustRun("tenant", "session-policy", "-o", "json").out
	if err := json.Unmarshal([]byte(out), &p); err != nil {
		t.Fatal(err)
	}
	if p.TTL != core.DefaultSessionTTL || p.IdleTimeout != 0 || p.RememberTTL != core.DefaultSessionRememberTTL {
		t.Fatalf("after clearing, policy = %+v, want the server's", p)
	}

	for _, args := range [][]string{
		{"tenant", "edit", "default", "--session-ttl", "48h"},
		{"tenant", "edit", "default", "--session-ttl", "soon"},
		{"tenant", "edit", "default", "--session-remember-ttl", "90d"},
	} {
		if got := c.run(args...); got.code != core.KindInvalid.ExitCode() {
			t.Errorf("tix %s exited %d, want %d\n%s", strings.Join(args, " "), got.code,
				core.KindInvalid.ExitCode(), got.err)
		}
	}
}

func TestLoginRefusesRememberWithAnSSHKey(t *testing.T) {
	c := newCLI(t)
	got := c.run("login", "--ssh-key", "--remember")
	if got.code != core.ExitUsage || !strings.Contains(got.err, "--remember") {
		t.Fatalf("exit %d, stderr %q; want a usage error naming --remember", got.code, got.err)
	}
}

func TestServeSessionFlagsJoinTheFlagLayerOnlyWhenTyped(t *testing.T) {
	cmd := commandNamed(t, "serve")
	if err := cmd.Flags().Parse([]string{"--session-ttl", "2h", "--session-idle-timeout", "15m",
		"--session-remember-ttl", "0"}); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"session.ttl": "2h0m0s", "session.idle_timeout": "15m0s", "session.remember_ttl": "0s"}
	if got := serveConfigFlags(cmd); !maps.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

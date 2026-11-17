// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestTenantSCIMCommands(t *testing.T) {
	c := newCLI(t)
	var off core.SCIMSettings
	if err := json.Unmarshal([]byte(c.mustRun("tenant", "scim", "show", "-o", "json").out), &off); err != nil || off.Enabled {
		t.Fatalf("show before enabling: %+v, %v", off, err)
	}

	enabled := c.mustRun("tenant", "scim", "enable")
	if !strings.Contains(enabled.out, "tix_") || !strings.Contains(enabled.out, core.SCIMPath) {
		t.Errorf("enable printed no token or endpoint:\n%s", enabled.out)
	}
	if !strings.Contains(enabled.err, "shown once") {
		t.Errorf("enable did not say the token is shown once: %q", enabled.err)
	}
	if again := c.mustRun("tenant", "scim", "enable").out; strings.Contains(again, "tix_") {
		t.Errorf("enabling again printed a token:\n%s", again)
	}

	c.mustRun("tenant", "scim", "map-group", "Engineering", "--role", "admin")
	if r := c.run("tenant", "scim", "map-group", "Ops", "--role", "owner"); r.code != core.KindInvalid.ExitCode() {
		t.Errorf("an unknown role exited %d, want %d", r.code, core.KindInvalid.ExitCode())
	}
	shown := c.mustRun("tenant", "scim", "show").out
	if !strings.Contains(shown, "Engineering") || !strings.Contains(shown, "admin") {
		t.Errorf("show does not list the mapping:\n%s", shown)
	}

	var rotated core.SCIMSettings
	if err := json.Unmarshal([]byte(c.mustRun("tenant", "scim", "token", "-o", "json").out), &rotated); err != nil ||
		rotated.Token == "" || len(rotated.Tokens) != 1 {
		t.Errorf("token: %+v, %v", rotated, err)
	}
	if r := c.run("token", "create", "mixed", "--scope", "scim:provision", "--scope", "task:read"); r.code != core.KindInvalid.ExitCode() {
		t.Errorf("a mixed provisioning token exited %d", r.code)
	}

	c.mustRun("tenant", "scim", "unmap-group", "engineering")
	if r := c.run("tenant", "scim", "unmap-group", "engineering"); r.code != core.KindNotFound.ExitCode() {
		t.Errorf("unmapping twice exited %d, want %d", r.code, core.KindNotFound.ExitCode())
	}
	var disabled core.SCIMSettings
	if err := json.Unmarshal([]byte(c.mustRun("tenant", "scim", "disable", "-o", "json").out), &disabled); err != nil ||
		disabled.Enabled || len(disabled.Tokens) != 1 {
		t.Errorf("disable: %+v, %v", disabled, err)
	}
}

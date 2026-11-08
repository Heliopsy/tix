// SPDX-License-Identifier: AGPL-3.0-or-later

package connect

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/heliopsy/tix/internal/config"
	"github.com/heliopsy/tix/internal/core"
)

func TestAHostedConnectionIsTheSessionsActorInItsTenant(t *testing.T) {
	actor := &core.Actor{ID: "a1", TenantID: "t1", Handle: "ada"}
	via := core.Via{Kind: core.ViaSSH, KeyFingerprint: "SHA256:abc"}
	conn := Hosted(nil, actor, via, "acme")
	if conn.Actor != actor || conn.Via != via {
		t.Fatalf("the connection does not carry the session's actor and channel: %+v", conn)
	}
	if conn.Info.TenantID != "t1" || conn.Info.Target.Tenant != "acme" ||
		conn.Info.Target.Mode != ModeSSH || conn.Info.Target.Origin != OriginSSH {
		t.Fatalf("the connection does not name the key's tenant over ssh: %+v", conn.Info)
	}
	if got := conn.Info.Target.Describe(); got != "ssh session as acme" {
		t.Errorf("Describe = %q", got)
	}
	if conn.Info.Target.Implicit() {
		t.Error("an ssh session was taken for the zero-configuration database")
	}
}

func TestOnlyAMissingDefaultDatabaseNeedsOptingInto(t *testing.T) {
	dir := t.TempDir()
	missing := Target{Mode: ModeLocal, Origin: OriginDefault, Path: filepath.Join(dir, "tix.db")}
	present := Target{Mode: ModeLocal, Origin: OriginDefault, Path: filepath.Join(dir, "there.db")}
	if err := os.WriteFile(present.Path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	named := missing
	named.Origin = OriginFlag
	cfg := config.Defaulted()
	optedIn := config.Defaulted()
	optedIn.Config.Target = config.TargetLocal

	for _, tc := range []struct {
		name string
		cfg  *config.Resolved
		t    Target
		want bool
	}{
		{"nothing there and nothing asked for", cfg, missing, true},
		{"a database already there", cfg, present, false},
		{"a target somebody named", cfg, named, false},
		{"the target key set to local", optedIn, missing, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := NeedsOptIn(tc.cfg, tc.t); got != tc.want {
				t.Errorf("NeedsOptIn = %v, want %v", got, tc.want)
			}
		})
	}
}

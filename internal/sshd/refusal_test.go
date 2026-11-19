// SPDX-License-Identifier: AGPL-3.0-or-later

package sshd

import (
	"testing"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
)

func TestARefusedKeyNotesWhy(t *testing.T) {
	e, _, st := newEnrolled(t)
	enrol(t, st, "acme", "alice", core.RoleMember, "SHA256:alice")
	enrol(t, st, "beta", "alice", core.RoleMember, "SHA256:alice")
	enrol(t, st, "gamma", "bob", core.RoleMember, "SHA256:bob")

	cases := []struct{ name, user, fingerprint, want string }{
		{"unknown key", NeutralUser, "SHA256:stranger", auth.RefusedSSHUnknownKey},
		{"no tenant of that name", "nowhere", "SHA256:bob", auth.RefusedSSHWrongTenant},
		{"not enrolled in that tenant", "acme", "SHA256:bob", auth.RefusedSSHWrongTenant},
		{"ambiguous key", NeutralUser, "SHA256:alice", auth.RefusedSSHAmbiguousKey},
		{"accepted", "acme", "SHA256:alice", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, note := auth.WithRefusalNote(asUser(tc.user))
			_, err := e.ActorByFingerprint(ctx, tc.fingerprint)
			got, _ := note.Get()
			if got.Reason != tc.want || (err == nil) != (tc.want == "") {
				t.Errorf("noted %q with error %v, want %q", got.Reason, err, tc.want)
			}
		})
	}
}

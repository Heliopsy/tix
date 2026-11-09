// SPDX-License-Identifier: AGPL-3.0-or-later

package sshd

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// requireMFAPolicy turns the tenant's second factor policy on with a grace,
// and leaves every per-surface flag off, so what the test sees is the
// policy's doing and not the ssh flag's.
func requireMFAPolicy(t *testing.T, st store.Store, tenantID string, grace time.Duration) {
	t.Helper()
	ctx := context.Background()
	if err := st.Unscoped(ctx, func(u store.UnscopedTx) error {
		tenant, err := u.GetTenantByID(ctx, tenantID)
		if err != nil {
			return err
		}
		tenant.MFAPolicy = core.MFAPolicyRequired
		tenant.MFAGrace = core.Duration(grace)
		at := time.Now()
		tenant.MFARequiredAt = &at
		return u.UpdateTenant(ctx, tenant)
	}); err != nil {
		t.Fatalf("requiring a second factor: %v", err)
	}
}

func TestThePolicyOffersAnUnenrolledKeyOnlyEnrolment(t *testing.T) {
	srv, watched, st := factorListener(t, time.Hour)
	alice := seedPerson(t, st, "acme", "alice", "alice@example.com")
	seedBoard(t, st, watched.Service, alice)
	requireMFAPolicy(t, st, alice.tenantID, 0)

	sess := openPromptSession(t, srv.Addr(), "acme", alice.signer)
	sess.waitFor(t, "otpauth://totp/")
	if read, _ := watched.boardReadBefore("BeginTOTPEnrolment"); read || strings.Contains(sess.text(), boardMarker) {
		t.Fatalf("the board was read or drawn before enrolment: %q", sess.text())
	}
	sess.answer(t, totpNow(t, secretFromURI(t, sess.text())))
	sess.waitFor(t, "recovery codes")
	sess.answer(t, "")
	sess.waitFor(t, boardMarker)
}

func TestThePolicyGraceOpensTheBoard(t *testing.T) {
	srv, watched, st := factorListener(t, time.Hour)
	alice := seedPerson(t, st, "acme", "alice", "alice@example.com")
	seedBoard(t, st, watched.Service, alice)
	requireMFAPolicy(t, st, alice.tenantID, 7*24*time.Hour)

	sess := openPromptSession(t, srv.Addr(), "acme", alice.signer)
	sess.waitFor(t, boardMarker)
	if strings.Contains(sess.text(), "otpauth://") {
		t.Fatalf("enrolment was demanded inside the grace: %q", sess.text())
	}
}

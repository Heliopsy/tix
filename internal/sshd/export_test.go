// SPDX-License-Identifier: AGPL-3.0-or-later

package sshd

import (
	"context"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	gossh "golang.org/x/crypto/ssh"
)

// The seeding helpers the external command tests share with this package's
// own. They live in package sshd_test because they drive the real command
// tree, which imports this package.

// Person is an enrolled human, as seedPerson builds one.
type Person struct {
	TenantID string
	Actor    *core.Actor
	Signer   gossh.Signer
}

// SeedPerson enrols a new admin with a fresh key in tenantKey.
func SeedPerson(t *testing.T, st store.Store, tenantKey, handle, email string) Person {
	p := seedPerson(t, st, tenantKey, handle, email)
	return Person{TenantID: p.tenantID, Actor: p.actor, Signer: p.signer}
}

// SeedBoard gives the person's tenant its workflow and the project "tob".
func SeedBoard(t *testing.T, st store.Store, svc core.Service, p Person) {
	seedBoard(t, st, svc, person{tenantID: p.TenantID, actor: p.Actor, signer: p.Signer})
}

// RequireFactor turns a tenant's second factor requirement on for surface.
var RequireFactor = requireFactor

// EnrolFactor enrols the person's second factor, on a service running on clk,
// and returns its secret.
func EnrolFactor(t *testing.T, svc core.Service, clk clock.Clock, p Person) string {
	secret, _ := enrolFactor(t, svc, clk, person{tenantID: p.TenantID, actor: p.Actor, signer: p.Signer})
	return secret
}

// TOTPNow is the code an authenticator holding secret shows at clk's now.
var TOTPNow = totpNow

// SessionContext is a context acting as actor.
var SessionContext = sessionContext

// FactorTestKey seals second factors in the command tests.
const FactorTestKey = factorTestKey

// NewTestSigner is a fresh ed25519 key nobody enrolled.
func NewTestSigner(t *testing.T) gossh.Signer {
	s, _ := newSigner(t)
	return s
}

// RevokeKeysOf revokes every key the person holds, as disabling them does.
func RevokeKeysOf(t *testing.T, st store.Store, p Person) {
	t.Helper()
	ctx := context.Background()
	if err := st.Update(ctx, core.TenantScope{TenantID: p.TenantID}, func(tx store.Tx) error {
		keys, err := tx.ListSSHKeys(ctx, p.Actor.ID)
		if err != nil {
			return err
		}
		for _, k := range keys {
			if err := tx.RevokeSSHKey(ctx, k.ID, time.Now()); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("revoking keys: %v", err)
	}
}

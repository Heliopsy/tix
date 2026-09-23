// SPDX-License-Identifier: AGPL-3.0-or-later

package sshd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

// sandboxPrefix marks the tenants this listener owns. Nothing outside the
// prefix is ever counted, touched or reaped, so pointing the listener at a
// database that holds other tenants cannot destroy them.
const sandboxPrefix = "sandbox-"

// visitorHandle names the single actor inside a sandbox.
const visitorHandle = "visitor"

// visitorScopes is what a visitor may do inside their own tenant. It is
// generous on purpose: alone in a sandbox there is nobody else's work to
// spoil, so the demo should reach the screens a member cannot. What is held
// back either reaches outside the sandbox or breaks it in a way the visitor
// could not understand: administering the tenant itself, minting credentials,
// creating users, registering webhooks that would make this listener issue
// outbound requests, and bulk import.
var visitorScopes = []core.Scope{
	core.ScopeTaskRead, core.ScopeTaskWrite, core.ScopeTaskTransition,
	core.ScopeTaskClaim, core.ScopeTaskDelete,
	core.ScopeProjectRead, core.ScopeProjectWrite,
	core.ScopeWorkflowRead, core.ScopeWorkflowWrite,
	core.ScopeCommentWrite, core.ScopeArtifactWrite,
	core.ScopeEventSubscribe, core.ScopeAuditRead, core.ScopeExport,
}

// provisioner resolves a fingerprint to the sandbox it owns, creating and
// seeding one the first time a key is seen. It satisfies auth.PublicKeyLookup,
// which is the seam key enrolment will reuse with a lookup over registered
// keys instead of one that provisions.
type provisioner struct {
	store      store.Store
	service    core.Service
	clk        clock.Clock
	maxTenants int
	leaseTTL   time.Duration
	tenantTTL  time.Duration
}

var _ auth.PublicKeyLookup = (*provisioner)(nil)

// tenantKeyFor derives a sandbox's tenant key from a fingerprint. It is a pure
// function of the key the client proved, which is what makes a reconnection a
// lookup rather than a fresh seed.
func tenantKeyFor(fingerprint string) string {
	sum := sha256.Sum256([]byte(fingerprint))
	return sandboxPrefix + hex.EncodeToString(sum[:])[:32]
}

// isSandbox reports whether a tenant key belongs to this listener.
func isSandbox(key string) bool { return strings.HasPrefix(key, sandboxPrefix) }

// ActorByFingerprint returns the visitor actor for a fingerprint's sandbox,
// provisioning the sandbox when the fingerprint is new.
func (p *provisioner) ActorByFingerprint(ctx context.Context, fingerprint string) (*core.Actor, error) {
	key := tenantKeyFor(fingerprint)
	tenant, err := p.existing(ctx, key)
	if err != nil {
		return nil, err
	}
	if tenant != nil {
		return p.visitor(ctx, tenant.ID)
	}
	tenant, err = p.create(ctx, key, fingerprint)
	if err != nil {
		return nil, err
	}
	// The actor comes before the seed because the seeded rows are attributed
	// to it, and a task whose creator does not exist is a broken row.
	actor, err := p.visitor(ctx, tenant.ID)
	if err != nil {
		return nil, err
	}
	if err := p.seed(ctx, tenant.ID, actor.ID); err != nil {
		return nil, err
	}
	return actor, nil
}

// existing returns the sandbox behind a tenant key, touching its last-seen
// time, or nil when the fingerprint has never connected or has been reaped.
func (p *provisioner) existing(ctx context.Context, key string) (*core.Tenant, error) {
	var found *core.Tenant
	err := p.store.Unscoped(ctx, func(u store.UnscopedTx) error {
		t, err := u.GetTenantByKey(ctx, key)
		if core.IsKind(err, core.KindNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if t.DeletedAt != nil {
			return nil
		}
		// The write is the touch: UpdateTenant stamps updated_at, which is the
		// last-seen time the reaper selects on, so the sandbox of someone who
		// keeps coming back is never the one that expires.
		if err := u.UpdateTenant(ctx, t); err != nil {
			return err
		}
		found = t
		return nil
	})
	return found, err
}

// create registers a sandbox, refusing once the listener is full.
func (p *provisioner) create(ctx context.Context, key, fingerprint string) (*core.Tenant, error) {
	live, err := p.count(ctx)
	if err != nil {
		return nil, err
	}
	if live >= p.maxTenants {
		return nil, core.Precondition(
			"this demo is full: %d sandboxes are in use and none will be deleted to make room. "+
				"Each one is somebody's board, and they expire on their own after %s unvisited, so try again later",
			live, p.tenantTTL)
	}
	tenant := &core.Tenant{Key: key, Name: "Demo sandbox " + fingerprint}
	if err := p.store.Unscoped(ctx, func(u store.UnscopedTx) error {
		return u.CreateTenant(ctx, tenant)
	}); err != nil {
		return nil, err
	}
	return tenant, nil
}

// count returns how many sandboxes are live.
func (p *provisioner) count(ctx context.Context) (int, error) {
	live := 0
	err := walkTenants(ctx, p.store, func(t core.Tenant) {
		if isSandbox(t.Key) && t.DeletedAt == nil {
			live++
		}
	})
	return live, err
}

// visitor returns the sandbox's actor, creating it on first connection.
func (p *provisioner) visitor(ctx context.Context, tenantID string) (*core.Actor, error) {
	scope := core.TenantScope{TenantID: tenantID}
	var found *core.Actor
	if err := p.store.View(ctx, scope, func(tx store.Tx) error {
		a, err := tx.GetActorByHandle(ctx, visitorHandle)
		if core.IsKind(err, core.KindNotFound) {
			return nil
		}
		found = a
		return err
	}); err != nil {
		return nil, err
	}
	if found == nil {
		created := &core.Actor{
			TenantID:    tenantID,
			Kind:        core.ActorUser,
			Handle:      visitorHandle,
			DisplayName: "Visitor",
		}
		if err := p.store.Update(ctx, scope, func(tx store.Tx) error {
			return tx.CreateActor(ctx, created)
		}); err != nil {
			return nil, err
		}
		found = created
	}
	found.TenantID = tenantID
	// The scopes are set here and nowhere else. Leaving Role empty keeps the
	// grant explicit, so widening a built-in role never silently widens what a
	// stranger on the public listener can do.
	found.Role = ""
	found.Scopes = visitorScopes
	return found, nil
}

// walkTenants visits every tenant, following the keyset pages so a listener
// with more sandboxes than one page holds still sees all of them.
func walkTenants(ctx context.Context, st store.Store, visit func(core.Tenant)) error {
	page := core.Page{Limit: core.MaxPageLimit, Sort: "created_at", Direction: core.Ascending}
	for {
		var batch []core.Tenant
		if err := st.Unscoped(ctx, func(u store.UnscopedTx) error {
			var err error
			batch, err = u.ListTenants(ctx, page)
			return err
		}); err != nil {
			return err
		}
		for _, t := range batch {
			visit(t)
		}
		if len(batch) < page.Limit {
			return nil
		}
		last := batch[len(batch)-1]
		page.Cursor = core.Cursor{
			SortValue: sqlb.TimeText(last.CreatedAt),
			ID:        last.ID,
			Sort:      page.Sort,
			Direction: page.Direction,
		}.Encode()
	}
}

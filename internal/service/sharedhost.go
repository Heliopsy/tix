// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// The methods in this file serve the transport's tenant resolution, which runs
// once per request before any credential is examined. None is part of
// core.Service: they take no actor, answer only what the transport needs to
// pick a tenant, and each reads through ViewUnscoped for the reason
// ResolveDomain gives.

// ResolveHost maps a hostname to the tenant whose domain claims it, and
// reports whether that domain is marked shared. A host no domain claims is
// not found.
func (l *Local) ResolveHost(ctx context.Context, hostname string) (*core.Tenant, bool, error) {
	host, err := normalizeHostname(hostname)
	if err != nil {
		return nil, false, err
	}
	var (
		out    *core.Tenant
		shared bool
	)
	err = l.store.ViewUnscoped(ctx, func(u store.UnscopedTx) error {
		out, shared, err = u.ResolveHost(ctx, host)
		return err
	})
	return out, shared, err
}

// TenantForKey returns the live tenant a key names, for the tenant header.
func (l *Local) TenantForKey(ctx context.Context, key string) (*core.Tenant, error) {
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "" {
		return nil, core.Invalid("tenant key is required")
	}
	var out *core.Tenant
	err := l.store.ViewUnscoped(ctx, func(u store.UnscopedTx) error {
		t, err := u.GetTenantByKey(ctx, key)
		if err == nil && t.DeletedAt != nil {
			err = core.NotFound("tenant %q", key)
		}
		out = t
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// TenantAddress returns a tenant's own address, its first domain not marked
// shared, or "" when it is served on a shared host alone.
func (l *Local) TenantAddress(ctx context.Context, tenantID string) (string, error) {
	var out string
	err := l.store.ViewUnscoped(ctx, func(u store.UnscopedTx) error {
		var err error
		out, err = u.TenantAddress(ctx, tenantID)
		return err
	})
	return out, err
}

// CredentialTenant returns the tenant a session or API token belongs to, by
// the hash of its secret, or "" when none matches. It grants nothing: the
// credential is verified inside the tenant it names like on any other host.
func (l *Local) CredentialTenant(ctx context.Context, tokenHash string) (string, error) {
	if tokenHash == "" {
		return "", nil
	}
	var out string
	err := l.store.ViewUnscoped(ctx, func(u store.UnscopedTx) error {
		id, err := u.CredentialTenant(ctx, tokenHash)
		if core.IsKind(err, core.KindNotFound) {
			return nil
		}
		out = id
		return err
	})
	return out, err
}

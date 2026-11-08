// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"strings"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// normalizeHostname lowercases a host and drops its port, so that
// "ACME.Example.com:8443" and "acme.example.com" address the same tenant.
func normalizeHostname(host string) (string, error) {
	h := strings.TrimSpace(host)
	h = strings.TrimSuffix(h, ".")
	if strings.HasPrefix(h, "[") {
		if end := strings.Index(h, "]"); end > 0 {
			h = h[1:end]
		}
	} else if i := strings.LastIndex(h, ":"); i >= 0 && !strings.Contains(h[i+1:], ":") {
		h = h[:i]
	}
	h = strings.ToLower(strings.TrimSpace(h))
	if h == "" {
		return "", core.Invalid("hostname is required")
	}
	if strings.ContainsAny(h, " /\\") {
		return "", core.Invalid("hostname %q is not a valid host", host)
	}
	return h, nil
}

// AddDomain maps a hostname to the current tenant.
func (l *Local) AddDomain(ctx context.Context, in core.AddDomainInput) (*core.Domain, error) {
	actor, err := l.authorize(ctx, authz.ActionTenantAdmin, authz.Resource{})
	if err != nil {
		return nil, err
	}
	hostname, err := normalizeHostname(in.Hostname)
	if err != nil {
		return nil, err
	}
	mode := in.CertMode
	if mode == "" {
		mode = core.CertNone
	}
	if !mode.Valid() {
		return nil, core.Invalid("cert mode %q must be %q or %q", mode, core.CertNone, core.CertFile)
	}
	if mode == core.CertFile && (in.CertPath == "" || in.KeyPath == "") {
		return nil, core.Invalid("cert mode %q requires both a certificate and a key path", core.CertFile)
	}

	var out *core.Domain
	err = l.write(ctx, actor, func(m *mutation) error {
		d := &core.Domain{
			Hostname: hostname,
			CertMode: mode,
			CertPath: in.CertPath,
			KeyPath:  in.KeyPath,
		}
		if err := m.tx.AddDomain(ctx, d); err != nil {
			return err
		}
		out = d
		return m.Record(auditDomainAdd, eventDomainAdded, "domain", d.ID, "", nil, d,
			map[string]any{"hostname": d.Hostname})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ListDomains returns the current tenant's hostnames.
func (l *Local) ListDomains(ctx context.Context) ([]core.Domain, error) {
	actor, err := l.authorize(ctx, authz.ActionTenantAdmin, authz.Resource{})
	if err != nil {
		return nil, err
	}
	out := []core.Domain{}
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		found, err := tx.ListDomains(ctx)
		out = found
		return err
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// RemoveDomain detaches a hostname from the current tenant.
func (l *Local) RemoveDomain(ctx context.Context, hostname string) error {
	actor, err := l.authorize(ctx, authz.ActionTenantAdmin, authz.Resource{})
	if err != nil {
		return err
	}
	host, err := normalizeHostname(hostname)
	if err != nil {
		return err
	}
	return l.write(ctx, actor, func(m *mutation) error {
		domains, err := m.tx.ListDomains(ctx)
		if err != nil {
			return err
		}
		var before *core.Domain
		for i := range domains {
			if domains[i].Hostname == host {
				before = &domains[i]
				break
			}
		}
		if before == nil {
			return core.NotFound("domain %q", host)
		}
		if err := m.tx.RemoveDomain(ctx, host); err != nil {
			return err
		}
		return m.Record(auditDomainRemove, eventDomainRemoved, "domain", before.ID, "", before, nil,
			map[string]any{"hostname": host})
	})
}

// ResolveDomain maps a hostname to its tenant. It runs before authentication,
// so it takes no actor and reads across tenants.
//
// It reads through ViewUnscoped rather than Unscoped because this runs on every
// request: on SQLite an unscoped write transaction holds the one writer
// connection, so host resolution behind an open write blocked until the
// request's own deadline.
func (l *Local) ResolveDomain(ctx context.Context, hostname string) (*core.Tenant, error) {
	host, err := normalizeHostname(hostname)
	if err != nil {
		return nil, err
	}
	var out *core.Tenant
	if err := l.store.ViewUnscoped(ctx, func(u store.UnscopedTx) error {
		t, err := u.ResolveDomain(ctx, host)
		out = t
		return err
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateDomain changes a domain of any tenant. Only a global admin may: a
// shared host serves tenants other than the one owning the domain, so the
// tenant's own administrator is not the one to decide it. Setting the value
// the domain already holds changes nothing and records nothing.
func (l *Local) UpdateDomain(ctx context.Context, hostname string, in core.UpdateDomainInput) (*core.Domain, error) {
	if _, err := l.authorizeGlobal(ctx); err != nil {
		return nil, err
	}
	host, err := normalizeHostname(hostname)
	if err != nil {
		return nil, err
	}
	owner, err := l.ResolveDomain(ctx, host)
	if err != nil {
		return nil, err
	}
	inner, err := l.inTenant(ctx, owner.ID)
	if err != nil {
		return nil, err
	}
	actor, err := core.RequireActor(inner)
	if err != nil {
		return nil, err
	}
	var out *core.Domain
	err = l.write(inner, actor, func(m *mutation) error {
		before, err := domainNamed(inner, m.tx, host)
		if err != nil {
			return err
		}
		after := *before
		out = &after
		if in.Shared == nil || *in.Shared == before.Shared {
			return nil
		}
		after.Shared = *in.Shared
		if err := m.tx.SetDomainShared(inner, host, after.Shared); err != nil {
			return err
		}
		return m.Record(auditDomainUpdate, eventDomainUpdated, "domain", before.ID, "",
			map[string]any{"hostname": host, "shared": before.Shared},
			map[string]any{"hostname": host, "shared": after.Shared},
			map[string]any{"hostname": host, "shared": after.Shared})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ListTenantDomains lists the domains of any tenant, for a global admin.
func (l *Local) ListTenantDomains(ctx context.Context, tenant string) ([]core.Domain, error) {
	inner, err := l.inTenant(ctx, tenant)
	if err != nil {
		return nil, err
	}
	return l.ListDomains(inner)
}

// domainNamed finds one of the scoped tenant's domains by hostname.
func domainNamed(ctx context.Context, tx store.Tx, host string) (*core.Domain, error) {
	domains, err := tx.ListDomains(ctx)
	if err != nil {
		return nil, err
	}
	for i := range domains {
		if domains[i].Hostname == host {
			return &domains[i], nil
		}
	}
	return nil, core.NotFound("domain %q", host)
}

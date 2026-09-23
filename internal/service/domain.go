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
func (l *Local) ResolveDomain(ctx context.Context, hostname string) (*core.Tenant, error) {
	host, err := normalizeHostname(hostname)
	if err != nil {
		return nil, err
	}
	var out *core.Tenant
	if err := l.store.Unscoped(ctx, func(u store.UnscopedTx) error {
		t, err := u.ResolveDomain(ctx, host)
		out = t
		return err
	}); err != nil {
		return nil, err
	}
	return out, nil
}

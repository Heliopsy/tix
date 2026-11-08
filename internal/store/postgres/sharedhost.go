// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/heliopsy/tix/internal/core"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

// extraScan scans a row into a scanner's destinations followed by extra ones,
// so a row scanner can be reused for a query selecting more columns.
type extraScan struct {
	s     scanner
	extra []any
}

func (e extraScan) Scan(dest ...any) error { return e.s.Scan(append(dest, e.extra...)...) }

// ResolveHost maps a hostname to the tenant that owns it and reports whether
// the domain is marked shared.
func (t *tx) ResolveHost(ctx context.Context, hostname string) (*core.Tenant, bool, error) {
	if err := t.domainAuthLookup(ctx, true); err != nil {
		return nil, false, err
	}
	defer func() { _ = t.domainAuthLookup(ctx, false) }()
	q, args := sqlb.HostByHostnameQuery(dialect, tenantColumns, hostname)
	var shared bool
	out, err := scanTenant(extraScan{s: t.ex.QueryRowContext(ctx, q, args...), extra: []any{&shared}})
	if err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, false, core.NotFound("no tenant serves hostname %q", hostname)
		}
		return nil, false, err
	}
	return &out, shared, nil
}

// TenantAddress returns a tenant's own address, or "" when it has none.
func (t *tx) TenantAddress(ctx context.Context, tenantID string) (string, error) {
	if err := t.domainAuthLookup(ctx, true); err != nil {
		return "", err
	}
	defer func() { _ = t.domainAuthLookup(ctx, false) }()
	q, args := sqlb.TenantAddressQuery(dialect, tenantID)
	var host string
	if err := t.ex.QueryRowContext(ctx, q, args...).Scan(&host); err != nil {
		return "", mapErr(err, "reading the address of tenant %q", tenantID)
	}
	return host, nil
}

// CredentialTenant returns the tenant a session or API token hash belongs to.
func (t *tx) CredentialTenant(ctx context.Context, tokenHash string) (string, error) {
	if err := t.credentialLookup(ctx, true); err != nil {
		return "", err
	}
	defer func() { _ = t.credentialLookup(ctx, false) }()
	q, args := sqlb.CredentialTenantQuery(dialect, tokenHash)
	var tenantID string
	if err := t.ex.QueryRowContext(ctx, q, args...).Scan(&tenantID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", core.NotFound("credential not found")
		}
		return "", mapErr(err, "finding the tenant of a credential")
	}
	return tenantID, nil
}

// SetDomainShared marks or unmarks one of this tenant's domains as shared.
func (t *tx) SetDomainShared(ctx context.Context, hostname string, shared bool) error {
	b := t.builder("tenant_domains").
		Where("hostname = ?", hostname).
		Set("shared", shared)
	n, err := t.execUpdate(ctx, b, "marking domain %q shared", hostname)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("domain %q", hostname)
	}
	return nil
}

// credentialLookup raises or lowers the flag credentialLookupPolicy reads. It
// is transaction-local, so it cannot outlive the lookup that raised it, and
// this is deliberately the only place in the tree that writes it.
func (t *tx) credentialLookup(ctx context.Context, on bool) error {
	value := ""
	if on {
		value = credentialLookupOn
	}
	if _, err := t.ex.ExecContext(ctx,
		`SELECT set_config($1, $2, true)`, credentialLookupSetting, value); err != nil {
		return mapErr(err, "admitting the credential lookup")
	}
	return nil
}

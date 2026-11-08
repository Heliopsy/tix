// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

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
	q, args := sqlb.HostByHostnameQuery(dialect, tenantColumns, hostname)
	var shared int64
	out, err := scanTenant(extraScan{s: t.ex.QueryRowContext(ctx, q, args...), extra: []any{&shared}})
	if err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, false, core.NotFound("no tenant serves hostname %q", hostname)
		}
		return nil, false, err
	}
	return &out, shared != 0, nil
}

// TenantAddress returns a tenant's own address, or "" when it has none.
func (t *tx) TenantAddress(ctx context.Context, tenantID string) (string, error) {
	q, args := sqlb.TenantAddressQuery(dialect, tenantID)
	var host string
	if err := t.ex.QueryRowContext(ctx, q, args...).Scan(&host); err != nil {
		return "", mapErr(err, "reading the address of tenant %q", tenantID)
	}
	return host, nil
}

// CredentialTenant returns the tenant a session or API token hash belongs to.
func (t *tx) CredentialTenant(ctx context.Context, tokenHash string) (string, error) {
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
		Set("shared", sqlb.BoolInt(shared))
	n, err := t.execUpdate(ctx, b, "marking domain %q shared", hostname)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("domain %q", hostname)
	}
	return nil
}

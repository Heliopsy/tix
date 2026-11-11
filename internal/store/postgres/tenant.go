// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"database/sql"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/id"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

// theme is read back here as well as written. It was written and never
// selected, so every tenant read on this engine answered with no theme while
// the column held one.
var tenantColumns = []string{
	"id", "key", "name", "theme",
	"require_totp_web", "require_totp_cli", "require_totp_ssh",
	"lease_default", "lease_max", "trash_off",
	"time_format", "timezone",
	"mfa_policy", "mfa_grace", "mfa_required_at",
	"artifact_max_bytes", "artifact_types",
	"session_ttl", "session_idle_timeout", "session_remember_ttl",
	"created_at", "updated_at", "deleted_at",
}

func scanTenant(s scanner) (core.Tenant, error) {
	var (
		t         core.Tenant
		created   sql.NullTime
		updated   sql.NullTime
		deletedAt sql.NullTime
		leaseDef  sql.NullString
		leaseMax  sql.NullString
		mfaGrace  string
		artTypes  string
		sessTTL   string
		sessIdle  string
		sessKeep  string
		mfaAt     sql.NullTime
	)
	if err := s.Scan(&t.ID, &t.Key, &t.Name, &t.Theme,
		&t.RequireTOTPWeb, &t.RequireTOTPCLI, &t.RequireTOTPSSH,
		&leaseDef, &leaseMax, &t.TrashOff,
		&t.TimeFormat, &t.Timezone,
		&t.MFAPolicy, &mfaGrace, &mfaAt,
		&t.ArtifactMaxBytes, &artTypes,
		&sessTTL, &sessIdle, &sessKeep,
		&created, &updated, &deletedAt); err != nil {
		return core.Tenant{}, mapErr(err, "scanning tenant")
	}
	var err error
	if t.LeaseDefault, err = sqlb.ScanDuration(sqlb.Text(leaseDef), "tenant lease default"); err != nil {
		return core.Tenant{}, err
	}
	if t.LeaseMax, err = sqlb.ScanDuration(sqlb.Text(leaseMax), "tenant lease maximum"); err != nil {
		return core.Tenant{}, err
	}
	t.ArtifactTypes = splitArtifactTypes(artTypes)
	if err = scanSessionSettings(&t, sessTTL, sessIdle, sessKeep); err != nil {
		return core.Tenant{}, err
	}
	if t.MFAGrace, err = sqlb.ScanDuration(mfaGrace, "tenant second factor grace"); err != nil {
		return core.Tenant{}, err
	}
	t.MFARequiredAt = scanNullTime(mfaAt)
	t.CreatedAt = scanTime(created)
	t.UpdatedAt = scanTime(updated)
	t.DeletedAt = scanNullTime(deletedAt)
	return t, nil
}

// GetTenant returns the tenant this transaction is scoped to.
func (t *tx) GetTenant(ctx context.Context) (*core.Tenant, error) {
	return t.tenantWhere(ctx, "id = ?", t.scope.TenantID, "tenant %q", t.scope.TenantID)
}

// GetTenantByID returns a tenant by identifier, ignoring the scope.
func (t *tx) GetTenantByID(ctx context.Context, tenantID string) (*core.Tenant, error) {
	return t.tenantWhere(ctx, "id = ?", tenantID, "tenant %q", tenantID)
}

// GetTenantByKey returns a tenant by its unique key.
func (t *tx) GetTenantByKey(ctx context.Context, key string) (*core.Tenant, error) {
	return t.tenantWhere(ctx, "key = ?", key, "tenant with key %q", key)
}

func (t *tx) tenantWhere(ctx context.Context, cond string, arg any, what string, whatArgs ...any) (*core.Tenant, error) {
	b := t.builder("tenants").Select(tenantColumns...).Where(cond, arg).Limit(1)
	q, args := b.SelectQuery()
	out, err := scanTenant(t.ex.QueryRowContext(ctx, q, args...))
	if err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, core.NotFound(what, whatArgs...)
		}
		return nil, err
	}
	return &out, nil
}

// ListTenants returns every tenant, keyset paginated.
func (t *tx) ListTenants(ctx context.Context, page core.Page) ([]core.Tenant, error) {
	spec, err := resolvePage(page, "created_at", map[string]string{
		"created_at": "created_at",
		"key":        "key",
		"name":       "name",
	})
	if err != nil {
		return nil, err
	}
	b := spec.apply(t.builder("tenants").Select(tenantColumns...), "id")
	rows, err := t.query(ctx, b, "listing tenants")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []core.Tenant{}
	for rows.Next() {
		v, err := scanTenant(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, mapRowsErr(rows, "listing tenants")
}

// CreateTenant inserts a tenant.
func (t *tx) CreateTenant(ctx context.Context, in *core.Tenant) error {
	if in.ID == "" {
		in.ID = id.New()
	}
	if in.CreatedAt.IsZero() {
		in.CreatedAt = t.store.clock.Now()
	}
	in.UpdatedAt = t.store.clock.Now()

	ins := t.insert("tenants").
		Set("id", in.ID).
		Set("key", in.Key).
		Set("name", in.Name).
		Set("theme", in.Theme).
		Set("require_totp_web", in.RequireTOTPWeb).
		Set("require_totp_cli", in.RequireTOTPCLI).
		Set("require_totp_ssh", in.RequireTOTPSSH).
		Set("lease_default", sqlb.DurationText(in.LeaseDefault)).
		Set("lease_max", sqlb.DurationText(in.LeaseMax)).
		Set("trash_off", in.TrashOff).
		Set("time_format", in.TimeFormat).
		Set("timezone", in.Timezone).
		Set("mfa_policy", string(mfaPolicyOf(in))).
		Set("mfa_grace", sqlb.DurationText(in.MFAGrace)).
		Set("mfa_required_at", nullTimeArg(in.MFARequiredAt)).
		Set("artifact_max_bytes", in.ArtifactMaxBytes).
		Set("artifact_types", strings.Join(in.ArtifactTypes, ",")).
		Set("session_ttl", sqlb.DurationText(in.SessionTTL)).
		Set("session_idle_timeout", sqlb.DurationText(in.SessionIdleTimeout)).
		Set("session_remember_ttl", core.SessionRememberText(in.SessionRememberTTL, in.SessionRememberOff)).
		Set("created_at", timeArg(in.CreatedAt)).
		Set("updated_at", timeArg(in.UpdatedAt)).
		Set("deleted_at", nullTimeArg(in.DeletedAt))
	_, err := t.execInsert(ctx, ins, "creating tenant %q", in.Key)
	return err
}

// UpdateTenant writes a tenant's mutable fields.
func (t *tx) UpdateTenant(ctx context.Context, in *core.Tenant) error {
	in.UpdatedAt = t.store.clock.Now()
	b := t.builder("tenants").
		Where("id = ?", in.ID).
		Set("key", in.Key).
		Set("name", in.Name).
		Set("theme", in.Theme).
		Set("require_totp_web", in.RequireTOTPWeb).
		Set("require_totp_cli", in.RequireTOTPCLI).
		Set("require_totp_ssh", in.RequireTOTPSSH).
		Set("lease_default", sqlb.DurationText(in.LeaseDefault)).
		Set("lease_max", sqlb.DurationText(in.LeaseMax)).
		Set("trash_off", in.TrashOff).
		Set("time_format", in.TimeFormat).
		Set("timezone", in.Timezone).
		Set("mfa_policy", string(mfaPolicyOf(in))).
		Set("mfa_grace", sqlb.DurationText(in.MFAGrace)).
		Set("mfa_required_at", nullTimeArg(in.MFARequiredAt)).
		Set("artifact_max_bytes", in.ArtifactMaxBytes).
		Set("artifact_types", strings.Join(in.ArtifactTypes, ",")).
		Set("session_ttl", sqlb.DurationText(in.SessionTTL)).
		Set("session_idle_timeout", sqlb.DurationText(in.SessionIdleTimeout)).
		Set("session_remember_ttl", core.SessionRememberText(in.SessionRememberTTL, in.SessionRememberOff)).
		Set("updated_at", timeArg(in.UpdatedAt)).
		Set("deleted_at", nullTimeArg(in.DeletedAt))
	n, err := t.execUpdate(ctx, b, "updating tenant %q", in.ID)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("tenant %q", in.ID)
	}
	return nil
}

// DeleteTenant removes a tenant and everything cascading from it.
func (t *tx) DeleteTenant(ctx context.Context, tenantID string) error {
	b := t.builder("tenants").Where("id = ?", tenantID)
	n, err := t.execDelete(ctx, b, "deleting tenant %q", tenantID)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("tenant %q", tenantID)
	}
	return nil
}

// ResolveDomain maps a hostname to the tenant that owns it.
//
// It runs before a tenant is known, and tenant_domains carries FORCE ROW LEVEL
// SECURITY with a policy admitting only the transaction's tenant, so an
// unscoped read would return no rows rather than failing loudly. Raising the
// domain auth flag adds the SELECT-only policy that admits this one read, and
// it is lowered again before the caller regains control.
func (t *tx) ResolveDomain(ctx context.Context, hostname string) (*core.Tenant, error) {
	if err := t.domainAuthLookup(ctx, true); err != nil {
		return nil, err
	}
	defer func() { _ = t.domainAuthLookup(ctx, false) }()

	q, args := sqlb.DomainByHostnameQuery(dialect, tenantColumns, hostname)
	out, err := scanTenant(t.ex.QueryRowContext(ctx, q, args...))
	if err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, core.NotFound("no tenant serves hostname %q", hostname)
		}
		return nil, err
	}
	return &out, nil
}

// domainAuthLookup raises or lowers the flag domainAuthPolicy reads. It is
// transaction-local, so it cannot outlive the lookup that raised it, and this
// is deliberately the only place in the tree that writes it.
func (t *tx) domainAuthLookup(ctx context.Context, on bool) error {
	value := ""
	if on {
		value = domainAuthOn
	}
	if _, err := t.ex.ExecContext(ctx,
		`SELECT set_config($1, $2, true)`, domainAuthSetting, value); err != nil {
		return mapErr(err, "admitting the hostname lookup")
	}
	return nil
}

var domainColumns = []string{
	"id", "tenant_id", "hostname", "verified_at", "cert_mode", "cert_path", "key_path", "created_at", "shared",
}

func scanDomain(s scanner) (core.Domain, error) {
	var (
		d        core.Domain
		verified sql.NullTime
		shared   bool
		created  sql.NullTime
	)
	if err := s.Scan(&d.ID, &d.TenantID, &d.Hostname, &verified,
		&d.CertMode, &d.CertPath, &d.KeyPath, &created, &shared); err != nil {
		return core.Domain{}, mapErr(err, "scanning domain")
	}
	d.Shared = shared
	d.VerifiedAt = scanNullTime(verified)
	d.CreatedAt = scanTime(created)
	return d, nil
}

// AddDomain attaches a hostname to this tenant.
func (t *tx) AddDomain(ctx context.Context, d *core.Domain) error {
	if d.ID == "" {
		d.ID = id.New()
	}
	d.TenantID = t.scope.TenantID
	if d.CreatedAt.IsZero() {
		d.CreatedAt = t.store.clock.Now()
	}
	if d.CertMode == "" {
		d.CertMode = core.CertNone
	}
	ins := t.insert("tenant_domains").
		Set("id", d.ID).
		Set("hostname", d.Hostname).
		Set("verified_at", nullTimeArg(d.VerifiedAt)).
		Set("cert_mode", string(d.CertMode)).
		Set("cert_path", d.CertPath).
		Set("key_path", d.KeyPath).
		Set("created_at", timeArg(d.CreatedAt))
	_, err := t.execInsert(ctx, ins, "adding domain %q", d.Hostname)
	return err
}

// ListDomains returns this tenant's hostnames.
func (t *tx) ListDomains(ctx context.Context) ([]core.Domain, error) {
	b := t.builder("tenant_domains").
		Select(domainColumns...).
		OrderBy("hostname", core.Ascending)
	rows, err := t.query(ctx, b, "listing domains")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []core.Domain{}
	for rows.Next() {
		v, err := scanDomain(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, mapRowsErr(rows, "listing domains")
}

// RemoveDomain detaches a hostname from this tenant.
func (t *tx) RemoveDomain(ctx context.Context, hostname string) error {
	b := t.builder("tenant_domains").
		Where("hostname = ?", hostname)
	n, err := t.execDelete(ctx, b, "removing domain %q", hostname)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("domain %q", hostname)
	}
	return nil
}

func scanMembership(s scanner) (core.Membership, error) {
	var (
		m       core.Membership
		created sql.NullTime
	)
	if err := s.Scan(&m.TenantID, &m.ActorID, &m.Role, &created); err != nil {
		return core.Membership{}, mapErr(err, "scanning membership")
	}
	m.CreatedAt = scanTime(created)
	return m, nil
}

// AddMember grants an actor a role in this tenant.
func (t *tx) AddMember(ctx context.Context, m *core.Membership) error {
	m.TenantID = t.scope.TenantID
	if m.CreatedAt.IsZero() {
		m.CreatedAt = t.store.clock.Now()
	}
	ins := t.insert("tenant_members").
		Set("actor_id", m.ActorID).
		Set("role", string(m.Role)).
		Set("created_at", timeArg(m.CreatedAt))
	_, err := t.execInsert(ctx, ins, "adding member %q", m.ActorID)
	return err
}

// GetMember returns an actor's membership in this tenant.
func (t *tx) GetMember(ctx context.Context, actorID string) (*core.Membership, error) {
	b := t.builder("tenant_members").
		Select("tenant_id", "actor_id", "role", "created_at").
		Where("actor_id = ?", actorID).
		Limit(1)
	q, args := b.SelectQuery()
	m, err := scanMembership(t.ex.QueryRowContext(ctx, q, args...))
	if err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, core.NotFound("actor %q is not a member of this tenant", actorID)
		}
		return nil, err
	}
	return &m, nil
}

// ListMembers returns every membership in this tenant.
func (t *tx) ListMembers(ctx context.Context) ([]core.Membership, error) {
	b := t.builder("tenant_members").
		Select("tenant_id", "actor_id", "role", "created_at").
		OrderBy("actor_id", core.Ascending)
	rows, err := t.query(ctx, b, "listing members")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []core.Membership{}
	for rows.Next() {
		v, err := scanMembership(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, mapRowsErr(rows, "listing members")
}

// RemoveMember revokes an actor's membership in this tenant.
func (t *tx) RemoveMember(ctx context.Context, actorID string) error {
	b := t.builder("tenant_members").Where("actor_id = ?", actorID)
	n, err := t.execDelete(ctx, b, "removing member %q", actorID)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("actor %q is not a member of this tenant", actorID)
	}
	return nil
}

// mfaPolicyOf is the policy a tenant row stores; a tenant built without one
// is off, which is what the column defaults to.
func mfaPolicyOf(t *core.Tenant) core.MFAPolicy {
	if t.MFAPolicy == "" {
		return core.MFAPolicyOff
	}
	return t.MFAPolicy
}

// splitArtifactTypes reads the stored comma separated list; empty inherits.
func splitArtifactTypes(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

// scanSessionSettings reads a tenant's stored session settings; empty inherits.
func scanSessionSettings(t *core.Tenant, ttl, idle, remember string) error {
	var err error
	if t.SessionTTL, err = sqlb.ScanDuration(ttl, "tenant session lifetime"); err != nil {
		return err
	}
	if t.SessionIdleTimeout, err = sqlb.ScanDuration(idle, "tenant session idle timeout"); err != nil {
		return err
	}
	if t.SessionRememberTTL, t.SessionRememberOff, err = core.ParseSessionRemember(remember); err != nil {
		return core.Internal("parsing stored tenant keep me signed in %q", remember).Wrap(err)
	}
	return nil
}

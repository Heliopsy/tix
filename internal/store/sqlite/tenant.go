package sqlite

import (
	"context"
	"database/sql"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/id"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

var tenantColumns = []string{"id", "key", "name", "created_at", "updated_at", "deleted_at"}

func scanTenant(s scanner) (core.Tenant, error) {
	var (
		t         core.Tenant
		created   sql.NullString
		updated   sql.NullString
		deletedAt sql.NullString
	)
	if err := s.Scan(&t.ID, &t.Key, &t.Name, &created, &updated, &deletedAt); err != nil {
		return core.Tenant{}, mapErr(err, "scanning tenant")
	}
	var err error
	if t.CreatedAt, err = sqlb.ScanTime(created); err != nil {
		return core.Tenant{}, err
	}
	if t.UpdatedAt, err = sqlb.ScanTime(updated); err != nil {
		return core.Tenant{}, err
	}
	if t.DeletedAt, err = sqlb.ScanNullTime(deletedAt); err != nil {
		return core.Tenant{}, err
	}
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
	row := t.ex.QueryRowContext(ctx, q, args...)
	out, err := scanTenant(row)
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
		Set("created_at", sqlb.TimeText(in.CreatedAt)).
		Set("updated_at", sqlb.TimeText(in.UpdatedAt)).
		Set("deleted_at", sqlb.NullTimeText(in.DeletedAt))
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
		Set("updated_at", sqlb.TimeText(in.UpdatedAt)).
		Set("deleted_at", sqlb.NullTimeText(in.DeletedAt))
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
func (t *tx) ResolveDomain(ctx context.Context, hostname string) (*core.Tenant, error) {
	cols := make([]string, len(tenantColumns))
	for i, c := range tenantColumns {
		cols[i] = "tenants." + c
	}
	b := t.builder("tenant_domains").
		Select(cols...).
		Join("JOIN tenants ON tenants.id = tenant_domains.tenant_id").
		Where("tenant_domains.hostname = ?", hostname).
		Where("tenants.deleted_at IS NULL").
		Limit(1)
	q, args := b.SelectQuery()
	out, err := scanTenant(t.ex.QueryRowContext(ctx, q, args...))
	if err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, core.NotFound("no tenant serves hostname %q", hostname)
		}
		return nil, err
	}
	return &out, nil
}

var domainColumns = []string{
	"id", "tenant_id", "hostname", "verified_at", "cert_mode", "cert_path", "key_path", "created_at",
}

func scanDomain(s scanner) (core.Domain, error) {
	var (
		d        core.Domain
		verified sql.NullString
		created  sql.NullString
	)
	if err := s.Scan(&d.ID, &d.TenantID, &d.Hostname, &verified,
		&d.CertMode, &d.CertPath, &d.KeyPath, &created); err != nil {
		return core.Domain{}, mapErr(err, "scanning domain")
	}
	var err error
	if d.VerifiedAt, err = sqlb.ScanNullTime(verified); err != nil {
		return core.Domain{}, err
	}
	if d.CreatedAt, err = sqlb.ScanTime(created); err != nil {
		return core.Domain{}, err
	}
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
		Set("tenant_id", d.TenantID).
		Set("hostname", d.Hostname).
		Set("verified_at", sqlb.NullTimeText(d.VerifiedAt)).
		Set("cert_mode", string(d.CertMode)).
		Set("cert_path", d.CertPath).
		Set("key_path", d.KeyPath).
		Set("created_at", sqlb.TimeText(d.CreatedAt))
	_, err := t.execInsert(ctx, ins, "adding domain %q", d.Hostname)
	return err
}

// ListDomains returns this tenant's hostnames.
func (t *tx) ListDomains(ctx context.Context) ([]core.Domain, error) {
	b := t.builder("tenant_domains").
		Select(domainColumns...).
		Where("tenant_id = ?", t.scope.TenantID).
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
		Where("tenant_id = ?", t.scope.TenantID).
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
		created sql.NullString
	)
	if err := s.Scan(&m.TenantID, &m.ActorID, &m.Role, &created); err != nil {
		return core.Membership{}, mapErr(err, "scanning membership")
	}
	var err error
	if m.CreatedAt, err = sqlb.ScanTime(created); err != nil {
		return core.Membership{}, err
	}
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
		Set("created_at", sqlb.TimeText(m.CreatedAt))
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

func mapRowsErr(rows *sql.Rows, what string) error {
	if err := rows.Err(); err != nil {
		return mapErr(err, "%s", what)
	}
	return nil
}

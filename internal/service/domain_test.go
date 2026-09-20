package service

import (
	"context"
	"testing"

	"github.com/thereisnotime/tix/internal/core"
)

func TestNormalizeHostname(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "acme.example.com", want: "acme.example.com"},
		{in: "ACME.Example.com:8443", want: "acme.example.com"},
		{in: "  Acme.Example.com.  ", want: "acme.example.com"},
		{in: "[2001:db8::1]:8443", want: "2001:db8::1"},
		{in: "localhost:80", want: "localhost"},
		{in: "", wantErr: true},
		{in: ":443", wantErr: true},
		{in: "acme.example.com/path", wantErr: true},
	}
	for _, tc := range tests {
		got, err := normalizeHostname(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("normalizeHostname(%q) = %q, want an error", tc.in, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("normalizeHostname(%q) = %q, %v, want %q", tc.in, got, err, tc.want)
		}
	}
}

func TestAddDomainNormalizesAndRecords(t *testing.T) {
	l, _, scope, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	events, audits := countRows(t, l, scope)

	d, err := l.AddDomain(ctx, core.AddDomainInput{Hostname: "ACME.Example.com:8443"})
	if err != nil {
		t.Fatalf("AddDomain: %v", err)
	}
	if d.Hostname != "acme.example.com" {
		t.Errorf("hostname = %q, want the normalized form", d.Hostname)
	}
	if d.CertMode != core.CertNone {
		t.Errorf("cert mode = %q, want %q", d.CertMode, core.CertNone)
	}
	if d.Verified() {
		t.Error("a new domain must start unverified")
	}

	afterEvents, afterAudits := countRows(t, l, scope)
	if afterEvents != events+1 || afterAudits != audits+1 {
		t.Error("AddDomain did not write both an audit entry and an event")
	}
}

func TestAddDomainRejections(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)

	if _, err := l.AddDomain(ctx, core.AddDomainInput{Hostname: "acme.example.com"}); err != nil {
		t.Fatalf("AddDomain: %v", err)
	}
	tests := []struct {
		name string
		in   core.AddDomainInput
		kind core.Kind
	}{
		{"already claimed", core.AddDomainInput{Hostname: "ACME.example.com"}, core.KindConflict},
		{"empty", core.AddDomainInput{Hostname: " "}, core.KindInvalid},
		{"unknown cert mode", core.AddDomainInput{Hostname: "b.example.com", CertMode: core.CertMode("acme")}, core.KindInvalid},
		{"file mode without paths", core.AddDomainInput{Hostname: "c.example.com", CertMode: core.CertFile}, core.KindInvalid},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := l.AddDomain(ctx, tc.in); !core.IsKind(err, tc.kind) {
				t.Errorf("AddDomain = %v, want %s", err, tc.kind)
			}
		})
	}
}

func TestAddDomainWithCertificateFiles(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)

	d, err := l.AddDomain(ctx, core.AddDomainInput{
		Hostname: "tls.example.com",
		CertMode: core.CertFile,
		CertPath: "/etc/tix/tls.crt",
		KeyPath:  "/etc/tix/tls.key",
	})
	if err != nil {
		t.Fatalf("AddDomain: %v", err)
	}
	if d.CertPath != "/etc/tix/tls.crt" || d.KeyPath != "/etc/tix/tls.key" {
		t.Errorf("certificate paths not stored: %+v", d)
	}
}

func TestListAndRemoveDomain(t *testing.T) {
	l, _, scope, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)

	for _, host := range []string{"b.example.com", "a.example.com"} {
		if _, err := l.AddDomain(ctx, core.AddDomainInput{Hostname: host}); err != nil {
			t.Fatalf("AddDomain(%q): %v", host, err)
		}
	}
	got, err := l.ListDomains(ctx)
	if err != nil {
		t.Fatalf("ListDomains: %v", err)
	}
	if len(got) != 2 || got[0].Hostname != "a.example.com" {
		t.Errorf("ListDomains = %+v", got)
	}

	events, audits := countRows(t, l, scope)
	if err := l.RemoveDomain(ctx, "A.Example.com"); err != nil {
		t.Fatalf("RemoveDomain: %v", err)
	}
	afterEvents, afterAudits := countRows(t, l, scope)
	if afterEvents != events+1 || afterAudits != audits+1 {
		t.Error("RemoveDomain did not write both an audit entry and an event")
	}
	if err := l.RemoveDomain(ctx, "a.example.com"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("removing a domain twice = %v, want not found", err)
	}
	if err := l.RemoveDomain(ctx, " "); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("removing an empty hostname = %v, want invalid", err)
	}
}

// Resolution runs before authentication, so it takes no actor at all.
func TestResolveDomainNeedsNoActor(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)

	if _, err := l.AddDomain(ctx, core.AddDomainInput{Hostname: "acme.example.com"}); err != nil {
		t.Fatalf("AddDomain: %v", err)
	}

	anonymous := context.Background()
	got, err := l.ResolveDomain(anonymous, "ACME.Example.com:8443")
	if err != nil {
		t.Fatalf("ResolveDomain: %v", err)
	}
	if got.ID != actor.TenantID {
		t.Errorf("resolved tenant = %q, want %q", got.ID, actor.TenantID)
	}
	if _, err := l.ResolveDomain(anonymous, "unknown.example.com"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("ResolveDomain for an unmapped host = %v, want not found", err)
	}
	if _, err := l.ResolveDomain(anonymous, ""); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("ResolveDomain with no host = %v, want invalid", err)
	}
}

func TestDomainAdministrationRequiresTenantAdmin(t *testing.T) {
	l, _, _, actor := newLocal(t)
	member := &core.Actor{ID: "m1", TenantID: actor.TenantID, Kind: core.ActorUser, Role: core.RoleMember}
	ctx := core.WithActor(context.Background(), member)

	if _, err := l.AddDomain(ctx, core.AddDomainInput{Hostname: "a.example.com"}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("member adding a domain = %v, want forbidden", err)
	}
	if _, err := l.ListDomains(ctx); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("member listing domains = %v, want forbidden", err)
	}
	if err := l.RemoveDomain(ctx, "a.example.com"); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("member removing a domain = %v, want forbidden", err)
	}
}

// A hostname maps to at most one tenant across the whole deployment.
func TestDomainsDoNotLeakAcrossTenants(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)

	other, err := l.CreateTenant(ctx, core.CreateTenantInput{Key: "beta", Name: "Beta"})
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	otherCtx := core.WithActor(context.Background(), core.SystemActor(other.ID))
	if _, err := l.AddDomain(otherCtx, core.AddDomainInput{Hostname: "beta.example.com"}); err != nil {
		t.Fatalf("AddDomain: %v", err)
	}

	got, err := l.ListDomains(ctx)
	if err != nil {
		t.Fatalf("ListDomains: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("ListDomains leaked %+v from another tenant", got)
	}
	if err := l.RemoveDomain(ctx, "beta.example.com"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("removing another tenant's domain = %v, want not found", err)
	}
	if _, err := l.AddDomain(ctx, core.AddDomainInput{Hostname: "beta.example.com"}); !core.IsKind(err, core.KindConflict) {
		t.Errorf("claiming another tenant's hostname = %v, want conflict", err)
	}
}

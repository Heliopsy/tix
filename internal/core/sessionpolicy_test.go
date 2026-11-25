// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"testing"
	"time"
)

func TestEffectiveSessionPolicyOnlyTightens(t *testing.T) {
	h := func(n int) Duration { return Duration(time.Duration(n) * time.Hour) }
	server := SessionPolicy{TTL: h(12), IdleTimeout: 0, RememberTTL: h(720)}
	tests := []struct {
		name   string
		server SessionPolicy
		tenant Tenant
		want   SessionPolicy
	}{
		{"inherits", server, Tenant{}, SessionPolicy{TTL: h(12), RememberTTL: h(720)}},
		{"shorter ttl", server, Tenant{SessionTTL: h(8)}, SessionPolicy{TTL: h(8), RememberTTL: h(720)}},
		{"longer ttl clamped", server, Tenant{SessionTTL: h(24)}, SessionPolicy{TTL: h(12), RememberTTL: h(720)}},
		{"idle where the server has none", server, Tenant{SessionIdleTimeout: h(1)},
			SessionPolicy{TTL: h(12), IdleTimeout: h(1), RememberTTL: h(720)}},
		{"longer idle clamped", SessionPolicy{TTL: h(12), IdleTimeout: h(1), RememberTTL: h(720)},
			Tenant{SessionIdleTimeout: h(2)}, SessionPolicy{TTL: h(12), IdleTimeout: h(1), RememberTTL: h(720)}},
		{"remember off", server, Tenant{SessionRememberOff: true}, SessionPolicy{TTL: h(12)}},
		{"remember shorter", server, Tenant{SessionRememberTTL: h(168)}, SessionPolicy{TTL: h(12), RememberTTL: h(168)}},
		{"remember off on the server", SessionPolicy{TTL: h(12)}, Tenant{SessionRememberTTL: h(168)},
			SessionPolicy{TTL: h(12)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EffectiveSessionPolicy(tt.server, tt.tenant)
			if got.TTL != tt.want.TTL || got.IdleTimeout != tt.want.IdleTimeout || got.RememberTTL != tt.want.RememberTTL {
				t.Errorf("got ttl %s idle %s remember %s, want %s %s %s", got.TTL, got.IdleTimeout, got.RememberTTL,
					tt.want.TTL, tt.want.IdleTimeout, tt.want.RememberTTL)
			}
			if got.ServerTTL != tt.server.TTL {
				t.Errorf("server ttl = %s, want %s", got.ServerTTL, tt.server.TTL)
			}
		})
	}
}

func TestRememberIsOfferedOnlyWhenItGrantsMore(t *testing.T) {
	p := SessionPolicy{TTL: Duration(12 * time.Hour), RememberTTL: Duration(6 * time.Hour)}
	if p.RememberOffered() {
		t.Error("a remember lifetime shorter than the ordinary one is offered")
	}
	if d, kept := p.Lifetime(true); kept || d != p.TTL {
		t.Errorf("Lifetime(true) = %s, %v; want the ordinary ttl, not remembered", d, kept)
	}
	p.RememberTTL = DefaultSessionRememberTTL
	if d, kept := p.Lifetime(true); !kept || d != DefaultSessionRememberTTL {
		t.Errorf("Lifetime(true) = %s, %v; want the remember ttl", d, kept)
	}
	if d, kept := p.Lifetime(false); kept || d != p.TTL {
		t.Errorf("Lifetime(false) = %s, %v; want the ordinary ttl", d, kept)
	}
}

func TestParseSessionRemember(t *testing.T) {
	tests := []struct {
		in      string
		d       Duration
		off, ok bool
	}{
		{"", 0, false, true},
		{"off", 0, true, true},
		{"OFF", 0, true, true},
		{"0", 0, true, true},
		{"7d", Duration(7 * 24 * time.Hour), false, true},
		{"soon", 0, false, false},
	}
	for _, tt := range tests {
		d, off, err := ParseSessionRemember(tt.in)
		if (err == nil) != tt.ok || d != tt.d || off != tt.off {
			t.Errorf("ParseSessionRemember(%q) = %s, %v, %v", tt.in, d, off, err)
		}
		if tt.ok {
			if back, backOff, _ := ParseSessionRemember(SessionRememberText(d, off)); back != d || backOff != off {
				t.Errorf("%q does not survive SessionRememberText", tt.in)
			}
		}
	}
}

func TestUpdateTenantInputRefusesASessionSettingUnderAMinute(t *testing.T) {
	short := Duration(30 * time.Second)
	if err := (UpdateTenantInput{SessionTTL: &short}).Validate(); !IsKind(err, KindInvalid) {
		t.Errorf("a 30s session lifetime: err = %v, want invalid", err)
	}
	neg := Duration(-time.Hour)
	if err := (UpdateTenantInput{SessionIdleTimeout: &neg}).Validate(); !IsKind(err, KindInvalid) {
		t.Errorf("a negative idle timeout: err = %v, want invalid", err)
	}
	zero := Duration(0)
	if err := (UpdateTenantInput{SessionTTL: &zero}).Validate(); err != nil {
		t.Errorf("zero, which inherits, was refused: %v", err)
	}
}

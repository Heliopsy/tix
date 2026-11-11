// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

func TestTheTenantScreenStatesItsSessionSettings(t *testing.T) {
	tenant := core.Tenant{Key: "acme", SessionTTL: core.Duration(8 * time.Hour), SessionRememberOff: true}
	var text []string
	for _, l := range tenantAttributes(TenantAdminState{Tenant: &tenant, Loaded: true}) {
		text = append(text, l.Text)
	}
	all := strings.Join(text, "\n")
	for _, want := range []string{"8h", "off", "server's"} {
		if !strings.Contains(all, want) {
			t.Errorf("tenant rows lack %q:\n%s", want, all)
		}
	}
	form := TenantEditForm(tenant)
	opts := strings.Join(form.Fields[0].Options, ",")
	for _, a := range []string{attrSessionTTL, attrSessionIdle, attrSessionRemember} {
		if !strings.Contains(opts, a) {
			t.Errorf("the edit form does not offer %q", a)
		}
	}
}

func TestSessionSettingsTypedAtThePromptReachTheService(t *testing.T) {
	m, svc := adminModel(t)
	for _, tc := range []struct {
		kind promptKind
		text string
	}{
		{promptTenantSessionTTL, "8h"},
		{promptTenantSessionIdle, "30m"},
		{promptTenantSessionRemember, "off"},
		{promptTenantSessionTTL, ""},
	} {
		runCmd(t, m, m.updateSessionSetting(tc.kind, tc.text))
	}
	got := svc.tenantsEdited
	if len(got) != 4 {
		t.Fatalf("edits sent = %d, want 4", len(got))
	}
	if got[0].SessionTTL == nil || got[0].SessionTTL.D() != 8*time.Hour {
		t.Errorf("ttl sent = %v", got[0].SessionTTL)
	}
	if got[1].SessionIdleTimeout == nil || got[1].SessionIdleTimeout.D() != 30*time.Minute {
		t.Errorf("idle sent = %v", got[1].SessionIdleTimeout)
	}
	if got[2].SessionRememberTTL == nil || *got[2].SessionRememberTTL != "off" {
		t.Errorf("remember sent = %v", got[2].SessionRememberTTL)
	}
	if got[3].SessionTTL == nil || *got[3].SessionTTL != 0 {
		t.Errorf("an empty ttl sent %v, want zero to inherit", got[3].SessionTTL)
	}

	before := len(svc.tenantsEdited)
	runCmd(t, m, m.updateSessionSetting(promptTenantSessionIdle, "soon"))
	if len(svc.tenantsEdited) != before {
		t.Error("a duration that does not parse was sent")
	}
}

func TestEditingASessionSettingOpensThePromptSeededWithItsValue(t *testing.T) {
	m, _ := adminModel(t)
	d := core.Duration(45 * time.Minute)
	m.admin.tenant.SessionIdleTimeout = d
	form := TenantEditForm(*m.admin.tenant)
	form.Fields[0].Value = attrSessionIdle
	next, _ := m.applyTenantEdit(form)
	if next.prompt != promptTenantSessionIdle || next.input.Value() != "45m" {
		t.Errorf("prompt = %v %q, want the idle timeout seeded with 45m", next.prompt, next.input.Value())
	}
	for _, a := range []struct {
		attr string
		kind promptKind
	}{{attrSessionTTL, promptTenantSessionTTL}, {attrSessionRemember, promptTenantSessionRemember}} {
		form.Fields[0].Value = a.attr
		if next, _ := m.applyTenantEdit(form); next.prompt != a.kind {
			t.Errorf("%s opened prompt %v", a.attr, next.prompt)
		}
	}
}

// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// timePerson creates a user on the fixture tenant and returns a context that
// speaks for them.
func timePerson(t *testing.T, l *Local, admin *core.Actor, email string) context.Context {
	t.Helper()
	user, err := l.CreateUser(authContext(admin), core.CreateUserInput{
		Email: email, Password: testPassword, Role: core.RoleMember,
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	return authContext(&core.Actor{ID: user.ID, TenantID: admin.TenantID, Kind: core.ActorUser,
		Handle: "p", Role: core.RoleMember})
}

// The three levels resolve field by field: the person over the tenant over
// the deployment, with an empty value at any level inheriting from below.
func TestTimeSettingsResolveGlobalThenTenantThenUser(t *testing.T) {
	cases := []struct {
		name               string
		tenant, user       core.TimePrefs
		wantFormat, wantTZ string
		fromFormat, fromTZ string
	}{
		{"global only", core.TimePrefs{}, core.TimePrefs{}, "iso", "UTC", "global", "global"},
		{"tenant overrides global", core.TimePrefs{TimeFormat: "dotted", Timezone: "Europe/Sofia"}, core.TimePrefs{},
			"dotted", "Europe/Sofia", "tenant", "tenant"},
		{"user overrides tenant", core.TimePrefs{TimeFormat: "dotted", Timezone: "Europe/Sofia"},
			core.TimePrefs{TimeFormat: "us", Timezone: "Asia/Tokyo"}, "us", "Asia/Tokyo", "user", "user"},
		{"user inherits the format only", core.TimePrefs{TimeFormat: "dotted"},
			core.TimePrefs{Timezone: "Asia/Tokyo"}, "dotted", "Asia/Tokyo", "tenant", "user"},
		{"tenant inherits the zone only", core.TimePrefs{TimeFormat: "long"}, core.TimePrefs{},
			"long", "UTC", "tenant", "global"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l, _, _, admin := newLocalWith(t, WithTimeDefaults(core.TimePrefs{TimeFormat: "iso", Timezone: "UTC"}))
			adminCtx := authContext(admin)
			if _, err := l.UpdateTenant(adminCtx, "", core.UpdateTenantInput{
				TimeFormat: strp(c.tenant.TimeFormat), Timezone: strp(c.tenant.Timezone)}); err != nil {
				t.Fatalf("UpdateTenant: %v", err)
			}
			ctx := timePerson(t, l, admin, "p@example.com")
			if _, err := l.SetTimePrefs(ctx, core.TimePrefsInput{
				TimeFormat: strp(c.user.TimeFormat), Timezone: strp(c.user.Timezone)}); err != nil {
				t.Fatalf("SetTimePrefs: %v", err)
			}
			got, err := l.GetTimeSettings(ctx)
			if err != nil {
				t.Fatalf("GetTimeSettings: %v", err)
			}
			if got.Effective.TimeFormat != c.wantFormat || got.Effective.Timezone != c.wantTZ {
				t.Errorf("effective = %+v, want %s in %s", got.Effective, c.wantFormat, c.wantTZ)
			}
			if got.FormatSource != c.fromFormat || got.TimezoneSource != c.fromTZ {
				t.Errorf("sources = %s, %s; want %s, %s", got.FormatSource, got.TimezoneSource, c.fromFormat, c.fromTZ)
			}
			if !got.Personal {
				t.Errorf("a user reads as not personal")
			}
		})
	}
}

// Setting a field back to "" returns it to the level below, and leaving a
// field out of the input leaves it alone.
func TestSetTimePrefsInheritsOnEmptyAndLeavesAnAbsentField(t *testing.T) {
	l, _, _, admin := newLocalWith(t, WithTimeDefaults(core.TimePrefs{TimeFormat: "iso", Timezone: "UTC"}))
	ctx := timePerson(t, l, admin, "p@example.com")
	if _, err := l.SetTimePrefs(ctx, core.TimePrefsInput{TimeFormat: strp(" Dotted "), Timezone: strp("Asia/Tokyo")}); err != nil {
		t.Fatalf("SetTimePrefs: %v", err)
	}
	got, err := l.SetTimePrefs(ctx, core.TimePrefsInput{TimeFormat: strp("")})
	if err != nil {
		t.Fatalf("SetTimePrefs: %v", err)
	}
	if got.User.TimeFormat != "" || got.User.Timezone != "Asia/Tokyo" {
		t.Errorf("user = %+v, want the format inherited and the zone kept", got.User)
	}
	if got.Effective.TimeFormat != "iso" || got.FormatSource != core.TimeSourceGlobal {
		t.Errorf("effective format = %s from %s, want iso from global", got.Effective.TimeFormat, got.FormatSource)
	}
}

func TestTimePrefsRefuseWhatCannotBeRendered(t *testing.T) {
	l, _, _, admin := newLocal(t)
	ctx := timePerson(t, l, admin, "p@example.com")
	for _, in := range []core.TimePrefsInput{
		{TimeFormat: strp("klingon")},
		{Timezone: strp("Mars/Olympus")},
		{Timezone: strp("local")},
	} {
		if _, err := l.SetTimePrefs(ctx, in); !core.IsKind(err, core.KindInvalid) {
			t.Errorf("SetTimePrefs(%+v) = %v, want invalid", in, err)
		}
		if _, err := l.UpdateTenant(authContext(admin), "", core.UpdateTenantInput{TimeFormat: in.TimeFormat, Timezone: in.Timezone}); !core.IsKind(err, core.KindInvalid) {
			t.Errorf("UpdateTenant(%+v) = %v, want invalid", in, err)
		}
	}
}

// An agent has no users row: it reads the tenant's choice and cannot hold one
// of its own.
func TestAnAgentReadsTheTenantsDatesAndCannotSetItsOwn(t *testing.T) {
	l, _, _, admin := newLocal(t)
	if _, err := l.UpdateTenant(authContext(admin), "", core.UpdateTenantInput{TimeFormat: strp("dotted")}); err != nil {
		t.Fatalf("UpdateTenant: %v", err)
	}
	got, err := l.GetTimeSettings(authContext(admin))
	if err != nil {
		t.Fatalf("GetTimeSettings: %v", err)
	}
	if got.Personal || got.Effective.TimeFormat != "dotted" || got.FormatSource != core.TimeSourceTenant {
		t.Errorf("settings = %+v, want the tenant's dotted and not personal", got)
	}
	if _, err := l.SetTimePrefs(authContext(admin), core.TimePrefsInput{TimeFormat: strp("us")}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("SetTimePrefs as a non-person = %v, want invalid", err)
	}
}

// A person's choice is theirs alone: another member of the same tenant still
// reads the tenant's.
func TestOnePersonsChoiceDoesNotReachAnother(t *testing.T) {
	l, _, _, admin := newLocal(t)
	ada := timePerson(t, l, admin, "ada@example.com")
	bob := timePerson(t, l, admin, "bob@example.com")
	if _, err := l.SetTimePrefs(ada, core.TimePrefsInput{TimeFormat: strp("us")}); err != nil {
		t.Fatalf("SetTimePrefs: %v", err)
	}
	got, err := l.GetTimeSettings(bob)
	if err != nil {
		t.Fatalf("GetTimeSettings: %v", err)
	}
	if got.User.TimeFormat != "" || got.FormatSource == core.TimeSourceUser {
		t.Errorf("bob reads %+v, want nothing of ada's", got)
	}
}

// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"context"
	"errors"
	"maps"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/config"
	"github.com/heliopsy/tix/internal/connect"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/tui"
)

func TestFlagRebindingsWinOnlyForTheActionTheyName(t *testing.T) {
	for _, tc := range []struct {
		name              string
		configured, flags map[string]string
		want              map[string]string
	}{
		{"no configuration", nil, map[string]string{"Quit": "Q"}, map[string]string{"Quit": "Q"}},
		{"no flags", map[string]string{"Quit": "Q"}, nil, map[string]string{"Quit": "Q"}},
		{"both", map[string]string{"Quit": "Q", "Help": "H"}, map[string]string{"Quit": "x"},
			map[string]string{"Quit": "x", "Help": "H"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := mergeOverrides(tc.configured, tc.flags); !maps.Equal(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// timeSettingsService answers GetTimeSettings and nothing else.
type timeSettingsService struct {
	core.Service
	settings *core.TimeSettings
	err      error
}

func (s timeSettingsService) GetTimeSettings(context.Context) (*core.TimeSettings, error) {
	return s.settings, s.err
}

func TestTheAccountFillsOnlyTheTimePreferencesThisMachineLeftUnset(t *testing.T) {
	settings := &core.TimeSettings{
		Effective:    core.TimePrefs{TimeFormat: "iso", Timezone: "Europe/Sofia"},
		FormatSource: "user", TimezoneSource: "tenant",
	}
	unset := tui.Preferences{TimeFormat: string(config.LayerDefault), Timezone: string(config.LayerDefault)}

	prefs, sources := accountTimePrefs(context.Background(), timeSettingsService{settings: settings},
		tui.Preferences{TimeFormat: "relative", Timezone: "UTC"}, unset)
	if prefs.TimeFormat != "iso" || prefs.Timezone != "Europe/Sofia" {
		t.Errorf("unset preferences were not filled from the account: %+v", prefs)
	}
	if sources.TimeFormat != tui.AccountLayer+"user" || sources.Timezone != tui.AccountLayer+"tenant" {
		t.Errorf("the sources do not name the account's levels: %+v", sources)
	}

	pinned := tui.Preferences{TimeFormat: "file", Timezone: "env"}
	prefs, sources = accountTimePrefs(context.Background(), timeSettingsService{settings: settings},
		tui.Preferences{TimeFormat: "relative", Timezone: "UTC"}, pinned)
	if prefs.TimeFormat != "relative" || prefs.Timezone != "UTC" || sources != pinned {
		t.Errorf("a preference this machine set was overridden: %+v from %+v", prefs, sources)
	}

	prefs, sources = accountTimePrefs(context.Background(), timeSettingsService{err: errors.New("offline")},
		tui.Preferences{TimeFormat: "relative"}, unset)
	if prefs.TimeFormat != "relative" || sources != unset {
		t.Errorf("a refused read changed the preferences: %+v from %+v", prefs, sources)
	}
}

func TestTheSessionInfoNamesTheTargetTenantAndBuild(t *testing.T) {
	c := newCLI(t)
	g := &globals{environ: c.environ(), dir: c.home}
	resolved, err := g.resolve()
	if err != nil {
		t.Fatalf("resolving: %v", err)
	}
	info := g.sessionInfo(resolved)
	if info.Target == "" || !strings.HasPrefix(info.Version, "tix ") || info.ConfigFile == "" {
		t.Errorf("the session info is missing a row: %+v", info)
	}
}

func TestALocalDatabaseReachesEveryTenant(t *testing.T) {
	g := &globals{}
	local := &connect.Conn{Info: connect.Info{Target: connect.Target{Mode: connect.ModeLocal}}}
	if g.tenantReachable(local) != nil {
		t.Error("a local database was given a reachability filter; it reaches every tenant")
	}
	if got := currentTenantOf(&connect.Conn{}); got != "" {
		t.Errorf("a connection with no actor names tenant %q", got)
	}
	if got := currentTenantOf(&connect.Conn{Actor: &core.Actor{TenantKey: "acme"}}); got != "acme" {
		t.Errorf("currentTenantOf = %q, want acme", got)
	}
}

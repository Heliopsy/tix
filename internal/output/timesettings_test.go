// SPDX-License-Identifier: AGPL-3.0-or-later

package output_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

func table(t *testing.T, data any) string {
	t.Helper()
	var buf bytes.Buffer
	if err := output.NewWithMode(output.FormatTable, output.ModeNever).Format(&buf, data); err != nil {
		t.Fatalf("Format: %v", err)
	}
	return buf.String()
}

// One row per level, the personal one only for a person, and the row in use
// names where each field came from.
func TestTimeSettingsShowEveryLevelAndTheOneInUse(t *testing.T) {
	t.Parallel()
	s := core.ResolveTimePrefs(core.TimePrefs{TimeFormat: "iso", Timezone: "UTC"},
		core.TimePrefs{TimeFormat: "dotted"}, core.TimePrefs{Timezone: "Asia/Tokyo"})
	s.Personal = true
	got := table(t, &s)
	for _, want := range []string{"LEVEL", "global", "tenant", "user", "(inherit)",
		"dotted (from tenant)", "Asia/Tokyo (from user)"} {
		if !strings.Contains(got, want) {
			t.Errorf("the settings table lacks %q:\n%s", want, got)
		}
	}
	s.Personal = false
	if got := table(t, s); strings.Contains(got, "│ user") {
		t.Errorf("an automated caller was shown a personal row:\n%s", got)
	}
}

// A sync source says what it stores, where a run's mapping and credentials
// come from, and whether a run would start.
func TestASyncSourceRowSaysWhereEachPartComesFrom(t *testing.T) {
	t.Parallel()
	stored := core.SyncSource{
		ID: "s1", System: "jira", Name: "platform", MappingPath: "/etc/tix/jira.yaml",
		Config:      map[string]string{"url": "https://jira.example", "project": "PLAT"},
		Credentials: []core.SyncCredentialHint{{Name: "token", Hint: "wxyz"}, {Name: "user"}},
		Resolved: &core.SyncResolution{
			Mapping:     core.SyncOrigin{From: core.SyncFromPath, Detail: "/etc/tix/jira.yaml"},
			Credentials: core.SyncOrigin{From: core.SyncFromStored},
		},
	}
	got := table(t, stored)
	for _, want := range []string{"mapping=/etc/tix/jira.yaml project=PLAT url=", "file /etc/tix/jira.yaml",
		"stored: token ...wxyz, user", "yes"} {
		if !strings.Contains(got, want) {
			t.Errorf("the stored source lacks %q:\n%s", want, got)
		}
	}

	env := core.SyncSource{ID: "s2", System: "github", Name: "gh", Resolved: &core.SyncResolution{
		Mapping:     core.SyncOrigin{From: core.SyncFromEnv, Detail: "TIX_SYNC_GH_MAPPING"},
		Credentials: core.SyncOrigin{From: core.SyncFromEnv, Detail: "TIX_SYNC_GH_TOKEN"},
		Missing:     []string{"url", "token"},
	}}
	stored2 := core.SyncSource{ID: "s3", System: "linear", Name: "lin", Resolved: &core.SyncResolution{
		Mapping: core.SyncOrigin{From: core.SyncFromStored}, Credentials: core.SyncOrigin{From: core.SyncFromMissing},
	}}
	bare := core.SyncSource{ID: "s4", System: "trello", Name: "board",
		Resolved: &core.SyncResolution{Mapping: core.SyncOrigin{From: core.SyncFromNone}}}
	unresolved := core.SyncSource{ID: "s5", System: "jira", Name: "old"}
	got = table(t, []core.SyncSource{env, stored2, bare, unresolved})
	for _, want := range []string{"env TIX_SYNC_GH_MAPPING", "env: TIX_SYNC_GH_TOKEN", "no, 2 missing",
		"stored", "missing", "none"} {
		if !strings.Contains(got, want) {
			t.Errorf("the source list lacks %q:\n%s", want, got)
		}
	}
}

func TestTimeStyleNamesItsZoneAndFormat(t *testing.T) {
	t.Parallel()
	zero := output.TimeStyle{}
	if zero.Name() != output.TimeISO || zero.Location() != time.Local {
		t.Errorf("a zero style is %s in %s", zero.Name(), zero.Location())
	}
	sofia, err := output.NewTimeStyle("dotted", "Europe/Sofia")
	if err != nil {
		t.Fatal(err)
	}
	if key, ok := sofia.CacheKey(); !ok || key != "dotted|Europe/Sofia" {
		t.Errorf("CacheKey = %q, %v", key, ok)
	}
	at := time.Date(2026, 11, 6, 12, 5, 32, 0, time.UTC)
	if got := sofia.Full(at); got != "06.11.2026 14:05:32 EET (UTC+02:00)" {
		t.Errorf("Full = %q", got)
	}
	if sofia.Full(time.Time{}) != "" {
		t.Error("the zero time rendered")
	}
	for format, want := range map[string]string{
		"us": "11/06/2026 12:05:32 PM UTC (UTC+00:00)", "dayfirst": "06/11/2026 12:05:32 UTC (UTC+00:00)",
		"long": "6 Nov 2026 12:05:32 UTC (UTC+00:00)", "iso": "2026-11-06 12:05:32 UTC (UTC+00:00)",
	} {
		s, _ := output.NewTimeStyle(format, "UTC")
		if got := s.Full(at); got != want {
			t.Errorf("%s Full = %q, want %q", format, got, want)
		}
	}
	for format, want := range map[string]string{"dotted": "06.11", "dayfirst": "06/11", "us": "11/06",
		"short": "6 Nov", "iso": "11-06"} {
		s, _ := output.NewTimeStyle(format, "UTC")
		if got := s.DayMonth(at); got != want {
			t.Errorf("%s DayMonth = %q, want %q", format, got, want)
		}
	}
	offsetOnly, _ := output.NewTimeStyle("iso", "America/Sao_Paulo")
	if got := offsetOnly.Full(at); !strings.HasSuffix(got, " (UTC-03:00)") || strings.Contains(got, "-03 ") {
		t.Errorf("a zone named only by its offset repeated it: %q", got)
	}
}

type settingsReader struct {
	s   *core.TimeSettings
	err error
}

func (r settingsReader) GetTimeSettings(context.Context) (*core.TimeSettings, error) {
	return r.s, r.err
}

// The caller's own settings win; anything a service cannot answer leaves the
// fallback in force.
func TestForCallerFallsBackWhenTheServiceCannotSay(t *testing.T) {
	t.Parallel()
	fallback, _ := output.NewTimeStyle("us", "UTC")
	mine := &core.TimeSettings{Effective: core.TimePrefs{TimeFormat: "dotted", Timezone: "Europe/Sofia"}}
	if got := output.ForCaller(context.Background(), settingsReader{s: mine}, fallback); got.Name() != "dotted" ||
		got.Location().String() != "Europe/Sofia" {
		t.Errorf("the caller's settings were not used: %s %s", got.Name(), got.Location())
	}
	broken := &core.TimeSettings{Effective: core.TimePrefs{TimeFormat: "roman"}}
	for name, src := range map[string]output.TimeSettingsReader{
		"nil": nil, "error": settingsReader{err: errors.New("old server")},
		"empty": settingsReader{}, "unknown format": settingsReader{s: broken},
	} {
		if got := output.ForCaller(context.Background(), src, fallback); got.Name() != "us" {
			t.Errorf("%s: got %s, want the fallback", name, got.Name())
		}
	}
}

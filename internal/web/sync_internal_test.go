// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"encoding/json"
	"html/template"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	extsync "github.com/heliopsy/tix/internal/sync"
)

func TestEnvNameBreaksOnlyAtUnderscores(t *testing.T) {
	t.Parallel()
	got := string(envName(extsync.EnvName("ops export", ""), "*"))
	if want := "TIX_<wbr>SYNC_<wbr>OPS_<wbr>EXPORT_<wbr>*"; got != want {
		t.Fatalf("envName = %q, want %q", got, want)
	}
	if strings.ReplaceAll(got, "<wbr>", "") != "TIX_SYNC_OPS_EXPORT_*" {
		t.Fatal("closing the break opportunities does not give the name back")
	}
}

// TestEnvNameEscapesAnythingOutsideItsAlphabet covers the branch that can
// never be reached through the screen: envName returns template.HTML, so a
// value carrying markup would be written to the page unescaped if the
// alphabet check were ever removed or a caller passed something other than an
// extsync.EnvName result.
func TestEnvNameEscapesAnythingOutsideItsAlphabet(t *testing.T) {
	t.Parallel()
	for _, hostile := range []string{
		`<script>alert(1)</script>`,
		`TIX_SYNC_"onmouseover="x`,
		`TIX_SYNC_<img src=x onerror=y>`,
	} {
		got := string(envName(hostile))
		if strings.Contains(got, "<script") || strings.Contains(got, "<img") {
			t.Errorf("envName(%q) = %q, which carries live markup", hostile, got)
		}
		if strings.Contains(got, "<wbr>") {
			t.Errorf("envName(%q) treated a value outside its alphabet as a name", hostile)
		}
	}
}

func TestEnvNameSafeAcceptsOnlyWhatEnvNameProduces(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"ops export", "acme-jira", "n8n", "Ops  Export  2"} {
		if produced := extsync.EnvName(name, extsync.EnvMapping); !envNameSafe(produced) {
			t.Errorf("envNameSafe rejects %q, which extsync.EnvName produced from %q", produced, name)
		}
	}
	for _, bad := range []string{"tix_sync_lower", "TIX SYNC", "TIX_SYNC_<", "TIX_SYNC_&"} {
		if envNameSafe(bad) {
			t.Errorf("envNameSafe accepts %q", bad)
		}
	}
}

func TestSyncSettingsNameOnlyVariablesTheConfigReads(t *testing.T) {
	t.Parallel()
	// Every suffix LoadSourceConfig reads. A setting outside this set would
	// be advice that does nothing.
	read := map[string]bool{
		extsync.EnvMapping: true, extsync.EnvFile: true, extsync.EnvURL: true,
		extsync.EnvToken: true, extsync.EnvUser: true, extsync.EnvPassword: true,
		extsync.EnvQuery: true, extsync.EnvProject: true, extsync.EnvPageSize: true,
	}
	for _, system := range []string{"generic", "jira", "openproject", "something-else"} {
		settings := syncSettingsFor(system)
		if len(settings) == 0 {
			t.Errorf("system %q is offered no setting at all", system)
		}
		for _, s := range settings {
			if !read[s.Name] {
				t.Errorf("system %q is told to set %q, which no source config reads", system, s.Name)
			}
			if strings.TrimSpace(s.Purpose) == "" {
				t.Errorf("system %q names %q with no explanation", system, s.Name)
			}
		}
	}
}

func TestSyncSystemsCoverEveryAdapterTheServiceCanBuild(t *testing.T) {
	t.Parallel()
	described := map[string]bool{}
	for _, s := range syncSystems {
		described[s.Value] = true
		if strings.TrimSpace(s.Note) == "" {
			t.Errorf("system %q is offered with no explanation of when to pick it", s.Value)
		}
	}
	for _, system := range core.SyncSystems {
		if !described[system] {
			t.Errorf("system %q can be registered but the screen does not say what it is", system)
		}
	}
	if len(syncSystems) != len(core.SyncSystems) {
		t.Errorf("the screen describes %d systems but %d can be registered",
			len(syncSystems), len(core.SyncSystems))
	}
}

// envNameHTMLIsAString keeps the compiler honest about the return type the
// escaping argument above depends on.
var _ template.HTML = envName("TIX_SYNC_X_")

func TestSyncRunSurvivesASnapshotItCannotRead(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 9, 24, 20, 27, 0, 0, time.UTC)
	for _, tc := range []struct{ name, after string }{
		{"absent", ""},
		{"not json", "{this is not json"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := syncRunOf(core.AuditEntry{
				OccurredAt: at, ActorID: "actor-1", After: json.RawMessage(tc.after),
			})
			// The row still has to appear: the time and the actor are on the
			// entry itself, and dropping the run because its snapshot did not
			// parse would hide the very thing the screen is for.
			if !got.At.Equal(at) || got.ActorID != "actor-1" {
				t.Fatalf("syncRunOf lost the entry's own fields: %+v", got)
			}
			if got.Source != "" || got.Created != 0 {
				t.Fatalf("syncRunOf invented detail it could not read: %+v", got)
			}
		})
	}
}

func TestSyncRunReadsTheCountsOutOfItsSnapshot(t *testing.T) {
	t.Parallel()
	result := core.SyncResult{
		System: "generic", Source: "ops",
		ImportResult: core.ImportResult{
			Created: map[string]int{"task": 3}, Updated: map[string]int{"task": 1},
			Skipped: map[string]int{"task": 2}, Warnings: []string{"one"},
		},
	}
	snapshot, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshalling: %v", err)
	}
	got := syncRunOf(core.AuditEntry{After: snapshot})
	if got.Created != 3 || got.Updated != 1 || got.Skipped != 2 {
		t.Errorf("counts = %d/%d/%d, want 3/1/2", got.Created, got.Updated, got.Skipped)
	}
	if got.Source != "ops" || got.System != "generic" || len(got.Warnings) != 1 {
		t.Errorf("syncRunOf = %+v, want the source, system and warning from the snapshot", got)
	}
}

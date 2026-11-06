// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	extsync "github.com/heliopsy/tix/internal/sync"
)

func TestPutSyncSourceStoresItsSettings(t *testing.T) {
	f := newSyncFixture(t, "ops")
	src, err := f.local.PutSyncSource(f.ctx, core.SyncSourceInput{
		System: core.SystemJira, Name: "platform", MappingPath: " /etc/tix/platform.yaml ",
		Config: map[string]any{
			core.SyncSettingURL: "https://jira.test/", core.SyncSettingProject: "PLAT",
			core.SyncSettingQuery: "", core.SyncSettingPageSize: float64(25),
		},
	})
	if err != nil {
		t.Fatalf("PutSyncSource: %v", err)
	}
	want := map[string]string{"url": "https://jira.test", "project": "PLAT", "page_size": "25"}
	sources, err := f.local.ListSyncSources(f.ctx)
	if err != nil {
		t.Fatalf("ListSyncSources: %v", err)
	}
	var listed core.SyncSource
	for _, s := range sources {
		if s.ID == src.ID {
			listed = s
		}
	}
	for _, got := range []core.SyncSource{*src, listed} {
		if len(got.Config) != len(want) || got.MappingPath != "/etc/tix/platform.yaml" {
			t.Errorf("source = %+v, want config %v and the trimmed mapping path", got, want)
		}
		for k, v := range want {
			if got.Config[k] != v {
				t.Errorf("config[%s] = %q, want %q", k, got.Config[k], v)
			}
		}
	}

	replaced, err := f.local.PutSyncSource(f.ctx, core.SyncSourceInput{
		ID: src.ID, System: core.SystemJira, Name: "platform",
		Config: map[string]any{core.SyncSettingQuery: "project = OPS"},
	})
	if err != nil {
		t.Fatalf("updating: %v", err)
	}
	if len(replaced.Config) != 1 || replaced.MappingPath != "" {
		t.Errorf("an update kept settings it did not send: %+v", replaced)
	}
}

func TestPutSyncSourceRefusesSettingsItsSystemDoesNotTake(t *testing.T) {
	f := newSyncFixture(t, "ops")
	cases := []struct {
		name   string
		system string
		config map[string]any
		want   string
	}{
		{"url on generic", core.SystemGeneric, map[string]any{"url": "https://x.test"}, "file, page_size"},
		{"file on jira", core.SystemJira, map[string]any{"file": "/tmp/x.csv"}, "url, project, query, page_size"},
		{"mapping as config", core.SystemOpenProject, map[string]any{"mapping_path": "/x.yaml"}, "not one"},
		{"zero page size", core.SystemGeneric, map[string]any{"page_size": "0"}, "positive integer"},
		{"fractional page size", core.SystemGeneric, map[string]any{"page_size": 2.5}, "positive integer"},
		{"not a url", core.SystemJira, map[string]any{"url": "jira.test"}, "http or https"},
		{"not text", core.SystemJira, map[string]any{"project": true}, "must be text"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.local.PutSyncSource(f.ctx, core.SyncSourceInput{
				System: tc.system, Name: "refused", Config: tc.config,
			})
			if !core.IsKind(err, core.KindInvalid) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("PutSyncSource = %v, want invalid naming %q", err, tc.want)
			}
		})
	}
	sources, err := f.local.ListSyncSources(f.ctx)
	if err != nil {
		t.Fatalf("ListSyncSources: %v", err)
	}
	if len(sources) != 1 {
		t.Errorf("a refused source was stored: %d sources", len(sources))
	}
}

func TestPutSyncSourceRefusesCredentialsAsCredentials(t *testing.T) {
	f := newSyncFixture(t, "ops")
	for _, key := range []string{"token", "user", "username", "password", "api_key"} {
		_, err := f.local.PutSyncSource(f.ctx, core.SyncSourceInput{
			System: core.SystemJira, Name: "jira", Config: map[string]any{key: "value"},
		})
		if !core.IsKind(err, core.KindInvalid) || !strings.Contains(err.Error(), "sealed and never returned") {
			t.Errorf("PutSyncSource(%s) = %v, want the credential refusal", key, err)
		}
		if err != nil && strings.Contains(err.Error(), "value") {
			t.Errorf("the refusal of %s repeats the value: %v", key, err)
		}
	}
}

func TestRunSyncStoredSettingsBeatTheEnvironment(t *testing.T) {
	f := newSyncFixture(t, "ops")
	stored := filepath.Join(f.dir, "stored.csv")
	if err := os.WriteFile(stored, []byte("id,title,status,updated\nS-1,from the stored file,To Do,2026-01-03T00:00:00Z\n"), 0o600); err != nil {
		t.Fatalf("writing rows: %v", err)
	}
	if _, err := f.local.PutSyncSource(f.ctx, core.SyncSourceInput{
		ID: f.source.ID, System: core.SystemGeneric, Name: "ops",
		Config: map[string]any{core.SyncSettingFile: stored},
	}); err != nil {
		t.Fatalf("storing the file: %v", err)
	}

	res := f.run(t, core.RunSyncInput{})
	if res.Created["task"] != 1 {
		t.Fatalf("created = %v, want the one row of the stored file", res.Created)
	}
	tasks := f.tasks(t)
	if len(tasks) != 1 || tasks[0].Title != "from the stored file" {
		t.Errorf("imported %+v, want only the stored file's row", tasks)
	}
	if got := f.reloadSource(t); got.Config[core.SyncSettingFile] != stored {
		t.Errorf("after the run the source holds %+v, want its stored file kept", got.Config)
	}
}

func TestRunSyncEnvironmentFillsWhatTheSourceLeavesEmpty(t *testing.T) {
	f := newSyncFixture(t, "ops")
	mapping := filepath.Join(f.dir, "mapping.yaml")
	t.Setenv(extsync.EnvName("ops", extsync.EnvMapping), filepath.Join(f.dir, "missing.yaml"))
	if _, err := f.local.PutSyncSource(f.ctx, core.SyncSourceInput{
		ID: f.source.ID, System: core.SystemGeneric, Name: "ops", MappingPath: mapping,
	}); err != nil {
		t.Fatalf("storing the mapping: %v", err)
	}

	res := f.run(t, core.RunSyncInput{})
	if res.Created["task"] != 2 {
		t.Fatalf("created = %v, want both rows of the file the environment names", res.Created)
	}
}

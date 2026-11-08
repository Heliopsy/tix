// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	extsync "github.com/heliopsy/tix/internal/sync"
)

// hostSecret stands in for a server file a tenant must never read, such as a
// service account key.
const hostSecret = "-----BEGIN PRIVATE KEY----- host-secret"

// writeHostSecret puts a file outside every tenant directory, shaped so a
// mapping would import it as a task if the server ever read it.
func writeHostSecret(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "service-account.csv")
	body := "id,title,status,updated\nS-1," + hostSecret + ",To Do,2026-01-01T00:00:00Z\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("writing the host secret: %v", err)
	}
	return path
}

const hostMapping = `
version: 1
project: ops
identity:
  id: id
  updated_at: updated
fields:
  title: title
status_field: status
statuses:
  To Do: todo
default_status: todo
`

// tenantKeyOf is the key of the fixture's tenant.
func tenantKeyOf(t *testing.T, f *syncFixture) string {
	t.Helper()
	scope, err := f.local.syncScopeFor(f.ctx, f.actor)
	if err != nil {
		t.Fatalf("resolving the tenant: %v", err)
	}
	return scope.tenantKey
}

// noTaskCarries fails when any task in the tenant holds text.
func noTaskCarries(t *testing.T, f *syncFixture, text string) {
	t.Helper()
	tasks, err := f.local.ListTasks(f.ctx, core.TaskFilter{Page: core.Page{Limit: 200}})
	if err != nil {
		t.Fatalf("listing tasks: %v", err)
	}
	for _, task := range tasks.Tasks {
		if strings.Contains(task.Title, text) {
			t.Fatalf("task %s carries a server file's content: %q", task.Ref, task.Title)
		}
	}
}

func TestSharedHostRefusesAServerPathATenantNames(t *testing.T) {
	secret := writeHostSecret(t)
	f := newSyncFixtureWith(t, "ops", WithSyncHost(SyncHost{}))
	cases := []struct {
		name  string
		in    core.SyncSourceInput
		field string
	}{
		{"generic file", core.SyncSourceInput{System: core.SystemGeneric, Name: "steal", Mapping: hostMapping,
			Config: map[string]any{core.SyncSettingFile: secret}}, core.SyncSettingFile},
		{"mapping path", core.SyncSourceInput{System: core.SystemGeneric, Name: "probe", MappingPath: secret},
			"mapping_path"},
		{"mapping path that does not exist", core.SyncSourceInput{System: core.SystemGeneric, Name: "probe",
			MappingPath: "/nonexistent/passwd"}, "mapping_path"},
	}
	var messages []string
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, check := range []bool{false, true} {
				in := tc.in
				in.Check = check
				_, err := f.local.PutSyncSource(f.ctx, in)
				if !core.IsKind(err, core.KindInvalid) {
					t.Fatalf("check=%v: PutSyncSource = %v, want the path refused", check, err)
				}
				if got := core.FieldOf(err); got != tc.field {
					t.Errorf("check=%v: refused on %q, want %q", check, got, tc.field)
				}
				if strings.Contains(err.Error(), "host-secret") {
					t.Errorf("the refusal disclosed the file: %v", err)
				}
				if tc.field == "mapping_path" {
					messages = append(messages, err.Error())
				}
			}
		})
	}
	for _, m := range messages[1:] {
		if m != messages[0] {
			t.Errorf("an existing and a missing path are refused differently:\n%s\n%s", messages[0], m)
		}
	}
	noTaskCarries(t, f, "host-secret")
}

func TestSharedHostWillNotRunAPathStoredBeforeTheUpgrade(t *testing.T) {
	secret := writeHostSecret(t)
	f := newSyncFixture(t, "ops")
	src, err := f.local.PutSyncSource(f.ctx, core.SyncSourceInput{System: core.SystemGeneric, Name: "legacy",
		Mapping: hostMapping, Config: map[string]any{core.SyncSettingFile: secret}})
	if err != nil {
		t.Fatalf("storing the path on a trusted host: %v", err)
	}
	shared := New(f.local.store, WithClock(f.clock), WithSyncHost(SyncHost{FileRoot: t.TempDir()}))
	useImporter(t, nil)

	listed, err := shared.ListSyncSources(f.ctx)
	if err != nil {
		t.Fatalf("ListSyncSources: %v", err)
	}
	for _, s := range listed {
		if s.ID == src.ID && s.Resolved.Ready() {
			t.Errorf("a stored path outside the tenant directory reads as ready: %+v", s.Resolved)
		}
	}
	_, err = shared.RunSync(f.ctx, core.RunSyncInput{SourceID: src.ID})
	if !core.IsKind(err, core.KindInvalid) || !strings.Contains(err.Error(), "not readable in this tenant's sync directory") {
		t.Fatalf("RunSync = %v, want the stored path refused", err)
	}
	if strings.Contains(err.Error(), "host-secret") {
		t.Errorf("the refusal disclosed the file: %v", err)
	}
	noTaskCarries(t, f, "host-secret")
}

func TestSharedHostReadsOnlyInsideTheTenantDirectory(t *testing.T) {
	secret := writeHostSecret(t)
	root := t.TempDir()
	f := newSyncFixtureWith(t, "ops", WithSyncHost(SyncHost{FileRoot: root}))
	dir := filepath.Join(root, tenantKeyOf(t, f))
	other := filepath.Join(root, "other")
	for _, d := range []string{dir, other} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatalf("creating %s: %v", d, err)
		}
	}
	rows := "id,title,status,updated\nE-1,from the tenant directory,To Do,2026-01-01T00:00:00Z\n"
	for path, body := range map[string]string{
		filepath.Join(dir, "rows.csv"):        rows,
		filepath.Join(dir, "mapping.yaml"):    hostMapping,
		filepath.Join(other, "theirs.csv"):    rows,
		filepath.Join(other, "mapping.yaml"):  hostMapping,
		filepath.Join(dir, "not-a-link.yaml"): hostMapping,
	} {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}
	}
	links := map[string]string{
		"escape.csv":   secret,
		"neighbour":    other,
		"inside.csv":   filepath.Join(dir, "rows.csv"),
		"escape.yaml":  secret,
		"dangling.csv": filepath.Join(root, "nothing-here.csv"),
	}
	for name, target := range links {
		if err := os.Symlink(target, filepath.Join(dir, name)); err != nil {
			t.Skipf("this filesystem cannot hold a symbolic link: %v", err)
		}
	}

	refusedAtSave := []struct{ name, path string }{
		{"absolute outside", secret},
		{"climbs out", "../other/theirs.csv"},
		{"another tenant's absolute path", filepath.Join(other, "theirs.csv")},
	}
	for _, tc := range refusedAtSave {
		t.Run("save "+tc.name, func(t *testing.T) {
			_, err := f.local.PutSyncSource(f.ctx, core.SyncSourceInput{System: core.SystemGeneric,
				Name: "bad", Mapping: hostMapping, Config: map[string]any{core.SyncSettingFile: tc.path}})
			if !core.IsKind(err, core.KindInvalid) || core.FieldOf(err) != core.SyncSettingFile {
				t.Fatalf("PutSyncSource = %v, want the file refused", err)
			}
		})
	}

	refusedAtRun := []string{"escape.csv", "neighbour/theirs.csv", "dangling.csv", "missing.csv", "."}
	want := ""
	for _, name := range refusedAtRun {
		t.Run("run "+name, func(t *testing.T) {
			src, err := f.local.PutSyncSource(f.ctx, core.SyncSourceInput{System: core.SystemGeneric,
				Name: "link", Mapping: hostMapping, Config: map[string]any{core.SyncSettingFile: name}})
			if name == "." {
				if err == nil {
					t.Fatal("the tenant directory itself was accepted as a file")
				}
				return
			}
			if err != nil {
				t.Fatalf("PutSyncSource: %v", err)
			}
			_, err = f.local.RunSync(f.ctx, core.RunSyncInput{SourceID: src.ID})
			if !core.IsKind(err, core.KindInvalid) {
				t.Fatalf("RunSync = %v, want the file refused", err)
			}
			got := strings.ReplaceAll(err.Error(), name, "NAME")
			if want == "" {
				want = got
			} else if got != want {
				t.Errorf("refusals differ, so they say why:\n%s\n%s", want, got)
			}
			if strings.Contains(err.Error(), "host-secret") {
				t.Errorf("the refusal disclosed the file: %v", err)
			}
		})
	}

	for _, tc := range []struct{ name, file, mapping string }{
		{"relative", "rows.csv", "mapping.yaml"},
		{"absolute inside", filepath.Join(dir, "rows.csv"), filepath.Join(dir, "mapping.yaml")},
		{"link that stays inside", "inside.csv", "not-a-link.yaml"},
	} {
		t.Run("reads "+tc.name, func(t *testing.T) {
			src, err := f.local.PutSyncSource(f.ctx, core.SyncSourceInput{System: core.SystemGeneric,
				Name: "ok", MappingPath: tc.mapping, Config: map[string]any{core.SyncSettingFile: tc.file}})
			if err != nil {
				t.Fatalf("PutSyncSource: %v", err)
			}
			if !src.Resolved.Ready() {
				t.Fatalf("a file in the tenant directory is not ready: %v", src.Resolved.Missing)
			}
			if _, err := f.local.RunSync(f.ctx, core.RunSyncInput{SourceID: src.ID, DryRun: true}); err != nil {
				t.Fatalf("RunSync: %v", err)
			}
		})
	}

	src, err := f.local.PutSyncSource(f.ctx, core.SyncSourceInput{System: core.SystemGeneric,
		Name: "maplink", MappingPath: "escape.yaml", Config: map[string]any{core.SyncSettingFile: "rows.csv"}})
	if err != nil {
		t.Fatalf("PutSyncSource: %v", err)
	}
	if _, err := f.local.RunSync(f.ctx, core.RunSyncInput{SourceID: src.ID}); err == nil ||
		strings.Contains(err.Error(), "host-secret") || strings.Contains(err.Error(), "BEGIN") {
		t.Fatalf("RunSync through a mapping link = %v, want a refusal that discloses nothing", err)
	}
	noTaskCarries(t, f, "host-secret")
}

// tokenTrap is an upstream that records every Authorization header it is sent.
type tokenTrap struct {
	srv  *httptest.Server
	mu   sync.Mutex
	auth []string
}

func newTokenTrap(t *testing.T) *tokenTrap {
	t.Helper()
	trap := &tokenTrap{}
	trap.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		trap.mu.Lock()
		trap.auth = append(trap.auth, r.Header.Get("Authorization"))
		trap.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"issues":[],"total":0,"isLast":true}`))
	}))
	t.Cleanup(trap.srv.Close)
	return trap
}

// received reports whether any request carried secret.
func (tr *tokenTrap) received(secret string) bool {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	for _, a := range tr.auth {
		if strings.Contains(a, secret) {
			return true
		}
	}
	return false
}

const operatorToken = "operator-token-0123456789"

// setOperatorEnv gives source corp the operator's credentials and url.
func setOperatorEnv(t *testing.T, url string) {
	t.Helper()
	t.Setenv(extsync.EnvName("corp", extsync.EnvToken), operatorToken)
	t.Setenv(extsync.EnvName("corp", extsync.EnvURL), url)
	t.Setenv(extsync.EnvName("corp", extsync.EnvProject), "CORP")
}

// putJiraCorp registers a jira source named corp, with url stored when given.
func putJiraCorp(t *testing.T, f *syncFixture, url string) *core.SyncSource {
	t.Helper()
	in := core.SyncSourceInput{System: core.SystemJira, Name: "corp", Mapping: hostMapping,
		Config: map[string]any{core.SyncSettingProject: "CORP"}}
	if url != "" {
		in.Config[core.SyncSettingURL] = url
	}
	src, err := f.local.PutSyncSource(f.ctx, in)
	if err != nil {
		t.Fatalf("PutSyncSource: %v", err)
	}
	return src
}

func TestEnvironmentCredentialsNeverReachATenantURL(t *testing.T) {
	cases := []struct {
		name      string
		host      func(f *syncFixture) SyncHost
		storeURL  bool
		wantToken bool
	}{
		{"shared host, unbound source", func(*syncFixture) SyncHost { return SyncHost{} }, true, false},
		{"shared host, bound source, stored url", func(f *syncFixture) SyncHost {
			return SyncHost{EnvSources: []string{tenantKeyOf(t, f) + "/corp"}}
		}, true, false},
		{"trusted host, stored url", func(*syncFixture) SyncHost { return SyncHost{Trusted: true} }, true, false},
		{"shared host, bound source, the environment's url", func(f *syncFixture) SyncHost {
			return SyncHost{EnvSources: []string{tenantKeyOf(t, f) + "/*"}}
		}, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			attacker, operator := newTokenTrap(t), newTokenTrap(t)
			setOperatorEnv(t, operator.srv.URL)
			base := newSyncFixture(t, "ops")
			f := *base
			f.local = New(base.local.store, WithClock(base.clock), WithSyncHost(tc.host(base)))
			useImporter(t, nil)

			stored := ""
			if tc.storeURL {
				stored = attacker.srv.URL
			}
			src := putJiraCorp(t, &f, stored)
			if strings.Contains(src.Resolved.Credentials.Detail, "TOKEN") && !tc.wantToken {
				t.Errorf("the resolution names the operator's variables: %+v", src.Resolved.Credentials)
			}
			_, _ = f.local.RunSync(f.ctx, core.RunSyncInput{SourceID: src.ID, DryRun: true})
			if attacker.received(operatorToken) {
				t.Fatal("the operator's token reached a url the tenant stored")
			}
			if got := operator.received(operatorToken); got != tc.wantToken {
				t.Errorf("operator url received the token = %v, want %v", got, tc.wantToken)
			}
		})
	}
}

func TestSharedHostDoesNotSayWhichVariablesAreSet(t *testing.T) {
	describe := func(t *testing.T, setEnv bool) core.SyncResolution {
		f := newSyncFixtureWith(t, "ops", WithSyncHost(SyncHost{}))
		if setEnv {
			setOperatorEnv(t, "https://jira.corp.test")
			t.Setenv(extsync.EnvName("corp", extsync.EnvMapping), "/etc/tix/corp.yaml")
		}
		src, err := f.local.PutSyncSource(f.ctx, core.SyncSourceInput{System: core.SystemJira, Name: "corp"})
		if err != nil {
			t.Fatalf("PutSyncSource: %v", err)
		}
		return *src.Resolved
	}
	without, with := describe(t, false), describe(t, true)
	if strings.Join(with.Missing, "\n") != strings.Join(without.Missing, "\n") ||
		with.Credentials != without.Credentials || with.Mapping != without.Mapping {
		t.Errorf("the resolution changes with the operator's environment:\nwithout %+v\nwith    %+v", without, with)
	}
	for _, s := range with.Settings {
		if s.From == core.SyncFromEnv {
			t.Errorf("setting %s came from the operator's environment", s.Key)
		}
	}
	for _, m := range with.Missing {
		if strings.Contains(m, "TIX_SYNC_CORP") {
			t.Errorf("a missing entry names an operator variable: %s", m)
		}
	}
}

func TestEnvBindingsReadTenantAndSource(t *testing.T) {
	b, err := extsync.ParseEnvBindings([]string{"acme/corp", " Beta/* ", ""})
	if err != nil {
		t.Fatalf("ParseEnvBindings: %v", err)
	}
	for _, tc := range []struct {
		tenant, source string
		want           bool
	}{
		{"acme", "corp", true}, {"acme", "CORP", true}, {"acme", "other", false},
		{"beta", "anything", true}, {"gamma", "corp", false},
	} {
		if got := b.Allows(tc.tenant, tc.source); got != tc.want {
			t.Errorf("Allows(%q, %q) = %v, want %v", tc.tenant, tc.source, got, tc.want)
		}
	}
	for _, bad := range []string{"acme", "/corp", "acme/"} {
		if _, err := extsync.ParseEnvBindings([]string{bad}); err == nil {
			t.Errorf("binding %q was accepted", bad)
		}
	}
}

// The run reads through the scope as well as the resolution refusing first,
// so a resolution bug alone does not reopen the read.
func TestSharedScopeReadsNoFileOutsideTheTenantDirectory(t *testing.T) {
	secret := writeHostSecret(t)
	for _, scope := range []syncScope{{}, {fileDir: t.TempDir()}} {
		if raw, err := scope.readStored("the file", secret); err == nil || strings.Contains(string(raw), "host-secret") {
			t.Errorf("scope %+v read a server file: %v", scope, err)
		}
		src := core.SyncSource{System: core.SystemGeneric, Name: "x", MappingPath: secret,
			Config: map[string]string{core.SyncSettingFile: secret}}
		if _, err := loadSyncMapping(src, core.SyncResolution{Mapping: core.SyncOrigin{From: core.SyncFromPath}}, scope); err == nil {
			t.Errorf("scope %+v loaded a mapping from a server file", scope)
		}
		if _, err := (&Local{}).syncConfigFor("t", src, scope); err == nil {
			t.Errorf("scope %+v built a config reading a server file", scope)
		}
	}
}

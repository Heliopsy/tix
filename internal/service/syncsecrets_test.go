// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	extsync "github.com/heliopsy/tix/internal/sync"
)

// syncSecret is long enough to earn a hint, so the hint can be told apart
// from the value.
const syncSecret = "op-secret-token-0123456789-wxyz"

func sealedSyncFixture(t *testing.T, name string) *syncFixture {
	t.Helper()
	key, err := auth.NewTOTPKey()
	if err != nil {
		t.Fatalf("minting key: %v", err)
	}
	return newSyncFixtureWith(t, name, WithTOTPKey(key))
}

// openProjectSource registers an OpenProject source with its mapping and url
// stored, and the given credentials.
func openProjectSource(t *testing.T, f *syncFixture, name string, in core.SyncSourceInput) *core.SyncSource {
	t.Helper()
	in.System, in.Name, in.Mapping = core.SystemOpenProject, name, syncMapping
	in.Config = map[string]any{core.SyncSettingURL: "https://op.test"}
	src, err := f.local.PutSyncSource(f.ctx, in)
	if err != nil {
		t.Fatalf("PutSyncSource: %v", err)
	}
	return src
}

// captureConfig installs an importer that records the configuration a run
// built and returns no records.
func captureConfig(t *testing.T) *extsync.SourceConfig {
	t.Helper()
	var got extsync.SourceConfig
	useImporter(t, func(_ string, cfg extsync.SourceConfig, _ *extsync.Mapping) (extsync.Importer, error) {
		got = cfg
		return &fakeImporter{failAt: -1}, nil
	})
	return &got
}

func TestSyncRunUsesTheStoredMappingDocument(t *testing.T) {
	f := newSyncFixture(t, "ops")
	t.Setenv(extsync.EnvName("ops", extsync.EnvMapping), "/nonexistent/on/this/server.yaml")
	if _, err := f.local.RunSync(f.ctx, core.RunSyncInput{SourceID: f.source.ID}); err == nil ||
		!strings.Contains(err.Error(), "missing on the server") {
		t.Fatalf("a run with an unreadable mapping file = %v, want it refused as missing on the server", err)
	}
	if _, err := f.local.PutSyncSource(f.ctx, core.SyncSourceInput{
		ID: f.source.ID, System: core.SystemGeneric, Name: "ops", Mapping: syncMapping,
	}); err != nil {
		t.Fatalf("storing the mapping: %v", err)
	}
	sources, err := f.local.ListSyncSources(f.ctx)
	if err != nil {
		t.Fatalf("ListSyncSources: %v", err)
	}
	if got := sources[0].Resolved; len(sources) != 1 || got == nil || got.Mapping.From != core.SyncFromStored || !got.Ready() {
		t.Fatalf("resolved = %+v, want the stored mapping and nothing missing", got)
	}
	result, err := f.local.RunSync(f.ctx, core.RunSyncInput{SourceID: f.source.ID})
	if err != nil {
		t.Fatalf("RunSync: %v", err)
	}
	if result.Created["task"] != 2 {
		t.Errorf("created = %v, want both records through the stored mapping", result.Created)
	}
}

func TestSyncStoredMappingIsKeptUntilCleared(t *testing.T) {
	f := newSyncFixture(t, "ops")
	put := func(in core.SyncSourceInput) *core.SyncSource {
		t.Helper()
		in.ID, in.System, in.Name = f.source.ID, core.SystemGeneric, "ops"
		src, err := f.local.PutSyncSource(f.ctx, in)
		if err != nil {
			t.Fatalf("PutSyncSource: %v", err)
		}
		return src
	}
	put(core.SyncSourceInput{Mapping: syncMapping})
	if got := put(core.SyncSourceInput{Config: map[string]any{"page_size": "5"}}); got.Mapping != syncMapping {
		t.Errorf("an update that sent no mapping dropped the stored one")
	}
	if got := put(core.SyncSourceInput{ClearMapping: true}); got.Mapping != "" {
		t.Errorf("clear_mapping kept the document")
	}
	_, err := f.local.PutSyncSource(f.ctx, core.SyncSourceInput{ID: f.source.ID, System: core.SystemGeneric,
		Name: "ops", Mapping: "version: 1\nidentity: {id: id}\n"})
	if !core.IsKind(err, core.KindInvalid) || core.FieldOf(err) != "mapping" {
		t.Errorf("a mapping naming no project = %v, want refused on the mapping field", err)
	}
}

func TestSyncCredentialsAreWriteOnly(t *testing.T) {
	f := sealedSyncFixture(t, "ops")
	src := openProjectSource(t, f, "op", core.SyncSourceInput{User: "apikey", Password: syncSecret})

	sources, err := f.local.ListSyncSources(f.ctx)
	if err != nil {
		t.Fatalf("ListSyncSources: %v", err)
	}
	var stored core.SyncSource
	if err := f.local.read(f.ctx, f.actor, func(tx store.Tx) error {
		got, err := tx.GetSyncSource(f.ctx, src.ID)
		if got != nil {
			stored = *got
		}
		return err
	}); err != nil {
		t.Fatalf("reading the row: %v", err)
	}
	entries, _, err := f.local.ListAudit(f.ctx, core.AuditFilter{Page: core.Page{Limit: 100}})
	if err != nil {
		t.Fatalf("ListAudit: %v", err)
	}
	surfaces := map[string]any{"put response": src, "list": sources, "audit": entries}
	for name, v := range surfaces {
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("encoding %s: %v", name, err)
		}
		if strings.Contains(string(raw), syncSecret) || strings.Contains(string(raw), stored.SealedCredentials) {
			t.Errorf("the %s carries the credential or its ciphertext: %s", name, raw)
		}
	}
	auditRaw, _ := json.Marshal(entries)
	if strings.Contains(string(auditRaw), "wxyz") {
		t.Errorf("the audit trail carries the credential hint: %s", auditRaw)
	}
	if stored.SealedCredentials == "" || strings.Contains(stored.SealedCredentials, syncSecret) {
		t.Errorf("the row holds %q, want the credential sealed", stored.SealedCredentials)
	}
	want := []core.SyncCredentialHint{{Name: "user"}, {Name: "password", Hint: "wxyz"}}
	for _, s := range sources {
		if s.ID == src.ID && (len(s.Credentials) != 2 || s.Credentials[0] != want[0] || s.Credentials[1] != want[1]) {
			t.Errorf("credentials = %+v, want %+v", s.Credentials, want)
		}
	}

	cfg := captureConfig(t)
	if _, err := f.local.RunSync(f.ctx, core.RunSyncInput{SourceID: src.ID, DryRun: true}); err != nil {
		t.Fatalf("RunSync: %v", err)
	}
	if user, password := cfg.BasicAuth(); user != "apikey" || password != syncSecret {
		t.Errorf("the run authenticated as %q, want the stored credentials", user)
	}
}

func TestSyncCredentialsNeedTheServersSealingKey(t *testing.T) {
	f := newSyncFixture(t, "ops")
	_, err := f.local.PutSyncSource(f.ctx, core.SyncSourceInput{
		System: core.SystemOpenProject, Name: "op", Token: syncSecret,
	})
	if !core.IsKind(err, core.KindPrecondition) || !strings.Contains(err.Error(), "auth.totp_key") ||
		!strings.Contains(err.Error(), "TIX_SYNC_OP_TOKEN") {
		t.Fatalf("storing a credential without a key = %v, want a refusal naming the key and the variable", err)
	}
	if strings.Contains(err.Error(), syncSecret) {
		t.Errorf("the refusal repeats the credential")
	}
}

func TestSyncRunNamesWhatIsMissingOnTheServer(t *testing.T) {
	f := sealedSyncFixture(t, "ops")
	src, err := f.local.PutSyncSource(f.ctx, core.SyncSourceInput{System: core.SystemOpenProject, Name: "op"})
	if err != nil {
		t.Fatalf("PutSyncSource: %v", err)
	}
	built := false
	useImporter(t, func(string, extsync.SourceConfig, *extsync.Mapping) (extsync.Importer, error) {
		built = true
		return &fakeImporter{failAt: -1}, nil
	})
	_, err = f.local.RunSync(f.ctx, core.RunSyncInput{SourceID: src.ID, DryRun: true})
	if !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("RunSync = %v, want invalid", err)
	}
	for _, want := range []string{"missing on the server", "credentials", "TIX_SYNC_OP_TOKEN",
		"a mapping document", "--mapping-file", "the OpenProject url"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not name %q", err, want)
		}
	}
	if built {
		t.Errorf("an adapter was built for a source that cannot run")
	}
}

func TestSyncStoredCredentialsWinOverTheEnvironment(t *testing.T) {
	cases := []struct {
		name      string
		stored    string
		env       string
		wantToken string
		wantFrom  string
	}{
		{"stored wins", syncSecret, "env-token", syncSecret, core.SyncFromStored},
		{"environment fills in", "", "env-token", "env-token", core.SyncFromEnv},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := sealedSyncFixture(t, "ops")
			t.Setenv(extsync.EnvName("op", extsync.EnvToken), tc.env)
			t.Setenv(extsync.EnvName("op", extsync.EnvURL), "https://op.test/")
			src := openProjectSource(t, f, "op", core.SyncSourceInput{Token: tc.stored})
			if src.Resolved.Credentials.From != tc.wantFrom {
				t.Errorf("credentials from %q, want %q", src.Resolved.Credentials.From, tc.wantFrom)
			}
			cfg := captureConfig(t)
			if _, err := f.local.RunSync(f.ctx, core.RunSyncInput{SourceID: src.ID, DryRun: true}); err != nil {
				t.Fatalf("RunSync: %v", err)
			}
			if cfg.Token() != tc.wantToken {
				t.Errorf("the run used a different token than %s", tc.name)
			}
		})
	}
}

func TestSyncCredentialsStayUntilReplacedOrCleared(t *testing.T) {
	f := sealedSyncFixture(t, "ops")
	src := openProjectSource(t, f, "op", core.SyncSourceInput{Token: syncSecret})
	kept, err := f.local.PutSyncSource(f.ctx, core.SyncSourceInput{ID: src.ID, System: core.SystemOpenProject,
		Name: "op", KeepSettings: true})
	if err != nil {
		t.Fatalf("updating: %v", err)
	}
	if kept.Resolved.Credentials.From != core.SyncFromStored || kept.Config["url"] != "https://op.test" {
		t.Errorf("an update without credentials or settings lost them: %+v", kept)
	}
	cleared, err := f.local.PutSyncSource(f.ctx, core.SyncSourceInput{ID: src.ID, System: core.SystemOpenProject,
		Name: "op", KeepSettings: true, ClearCredentials: true})
	if err != nil {
		t.Fatalf("clearing: %v", err)
	}
	if len(cleared.Credentials) != 0 || cleared.Resolved.Credentials.From != core.SyncFromMissing {
		t.Errorf("clear_credentials left %+v", cleared.Credentials)
	}
}

func TestSyncCheckDescribesAndWritesNothing(t *testing.T) {
	f := sealedSyncFixture(t, "ops")
	checked, err := f.local.PutSyncSource(f.ctx, core.SyncSourceInput{System: core.SystemOpenProject,
		Name: "preview", Mapping: syncMapping, Token: syncSecret, Check: true,
		Config: map[string]any{core.SyncSettingURL: "https://op.test"}})
	if err != nil {
		t.Fatalf("checking: %v", err)
	}
	sum := checked.Resolved.Summary
	if sum == nil || sum.Project != "ops" || sum.Identity != "id" || len(sum.Statuses) != 4 {
		t.Errorf("summary = %+v, want the project, identity and four status rows", sum)
	}
	if raw, _ := json.Marshal(checked); strings.Contains(string(raw), syncSecret) {
		t.Errorf("a check returned the credential: %s", raw)
	}
	sources, _ := f.local.ListSyncSources(f.ctx)
	for _, s := range sources {
		if s.Name == "preview" {
			t.Errorf("a check stored the source")
		}
	}
	_, err = f.local.PutSyncSource(f.ctx, core.SyncSourceInput{System: core.SystemOpenProject, Name: "preview",
		Mapping: strings.Replace(syncMapping, "project: ops", "project: nowhere", 1), Check: true})
	if core.FieldOf(err) != "mapping" || !strings.Contains(err.Error(), "nowhere") {
		t.Errorf("checking a mapping into a missing project = %v, want refused on the mapping", err)
	}
}

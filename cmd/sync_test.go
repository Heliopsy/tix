// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
	extsync "github.com/heliopsy/tix/internal/sync"
)

const cliMapping = `
version: 1
project: default
identity:
  id: id
  version: updated
  updated_at: updated
fields:
  title: title
status_field: status
statuses:
  Open: todo
default_status: todo
`

const cliRows = `id,title,status,updated
C-1,imported from a file,Open,2026-01-01T00:00:00Z
`

// configureSource registers a source and points the environment at a mapping
// and a file, returning the source identifier.
func configureSource(t *testing.T, c *cli, name string) string {
	t.Helper()
	dir := t.TempDir()
	mapping := filepath.Join(dir, "mapping.yaml")
	if err := os.WriteFile(mapping, []byte(cliMapping), 0o600); err != nil {
		t.Fatalf("writing mapping: %v", err)
	}
	rows := filepath.Join(dir, "rows.csv")
	if err := os.WriteFile(rows, []byte(cliRows), 0o600); err != nil {
		t.Fatalf("writing rows: %v", err)
	}
	t.Setenv(extsync.EnvName(name, extsync.EnvMapping), mapping)
	t.Setenv(extsync.EnvName(name, extsync.EnvFile), rows)

	got := c.mustRun("-o", "json", "sync", "source", "add", core.SystemGeneric, name)
	var source core.SyncSource
	if err := json.Unmarshal([]byte(got.out), &source); err != nil {
		t.Fatalf("decoding source: %v\n%s", err, got.out)
	}
	if source.ID == "" {
		t.Fatalf("source has no identifier: %s", got.out)
	}
	return source.ID
}

func TestSyncCommandGroupsPrintHelp(t *testing.T) {
	c := newCLI(t)
	for _, args := range [][]string{{"sync"}, {"sync", "source"}} {
		got := c.mustRun(args...)
		if !strings.Contains(got.out, "Available Commands") {
			t.Errorf("tix %s printed %q, want help", strings.Join(args, " "), got.out)
		}
	}
}

func TestSyncSourceLifecycleThroughTheCLI(t *testing.T) {
	c := newCLI(t)
	id := configureSource(t, c, "cliops")

	listed := c.mustRun("-o", "json", "sync", "source", "ls")
	if !strings.Contains(listed.out, "cliops") {
		t.Errorf("listing = %q, want the registered source", listed.out)
	}

	plan := c.mustRun("-o", "json", "sync", "source", "add", core.SystemJira, "planned", "--dry-run")
	if !strings.Contains(plan.out, "planned") {
		t.Errorf("dry run = %q", plan.out)
	}
	if after := c.mustRun("-o", "json", "sync", "source", "ls"); strings.Contains(after.out, "planned") {
		t.Errorf("a dry run registered a source: %q", after.out)
	}

	if got := c.mustRun("-o", "json", "sync", "source", "rm", id, "--dry-run"); !strings.Contains(got.out, id) {
		t.Errorf("dry run removal = %q", got.out)
	}
	c.mustRun("sync", "source", "rm", id)
	if after := c.mustRun("-o", "json", "sync", "source", "ls"); strings.Contains(after.out, "cliops") {
		t.Errorf("the source survived removal: %q", after.out)
	}
}

func TestSyncRunImportsAndSupportsDryRunAndFull(t *testing.T) {
	c := newCLI(t)
	id := configureSource(t, c, "cliops")

	dry := c.mustRun("-o", "json", "sync", "run", id, "--dry-run")
	var planned core.SyncResult
	if err := json.Unmarshal([]byte(dry.out), &planned); err != nil {
		t.Fatalf("decoding dry run: %v\n%s", err, dry.out)
	}
	if !planned.DryRun || planned.Created["task"] != 1 {
		t.Fatalf("dry run = %+v", planned)
	}
	if !strings.Contains(dry.err, "nothing was written") {
		t.Errorf("stderr = %q, want the dry run announced", dry.err)
	}
	if listed := c.mustRun("-o", "json", "task", "ls"); strings.Contains(listed.out, "imported from a file") {
		t.Error("a dry run created a task")
	}

	real := c.mustRun("-o", "json", "sync", "run", id)
	var done core.SyncResult
	if err := json.Unmarshal([]byte(real.out), &done); err != nil {
		t.Fatalf("decoding run: %v\n%s", err, real.out)
	}
	if done.Created["task"] != 1 {
		t.Fatalf("run = %+v", done)
	}
	if listed := c.mustRun("-o", "json", "task", "ls"); !strings.Contains(listed.out, "imported from a file") {
		t.Errorf("the imported task is missing: %q", listed.out)
	}

	again := c.mustRun("-o", "json", "sync", "run", id, "--full")
	var repeat core.SyncResult
	if err := json.Unmarshal([]byte(again.out), &repeat); err != nil {
		t.Fatalf("decoding second run: %v\n%s", err, again.out)
	}
	if repeat.Created["task"] != 0 || repeat.Skipped["task"] != 1 {
		t.Errorf("second run = %+v, want the record skipped and nothing duplicated", repeat)
	}
}

func TestSyncCommandsReportBadInput(t *testing.T) {
	c := newCLI(t)

	bad := c.run("sync", "source", "add", "trello", "x")
	if bad.code != core.KindInvalid.ExitCode() {
		t.Errorf("unknown system exited %d, want %d", bad.code, core.KindInvalid.ExitCode())
	}
	missing := c.run("sync", "run", "no-such-source")
	if missing.code != core.KindNotFound.ExitCode() {
		t.Errorf("unknown source exited %d, want %d", missing.code, core.KindNotFound.ExitCode())
	}
	if got := c.run("sync", "source", "add", "generic"); got.code != core.ExitUsage {
		t.Errorf("missing argument exited %d, want %d", got.code, core.ExitUsage)
	}
}

func TestSyncSourceAddStoresItsSettings(t *testing.T) {
	c := newCLI(t)
	c.mustRun("sync", "source", "add", core.SystemJira, "platform",
		"--url", "https://jira.test/", "--project", "PLAT", "--query", "status = Open",
		"--page-size", "25", "--mapping", "/etc/tix/platform.yaml")

	var sources []core.SyncSource
	listed := c.mustRun("-o", "json", "sync", "source", "ls")
	if err := json.Unmarshal([]byte(listed.out), &sources); err != nil {
		t.Fatalf("decoding sources: %v\n%s", err, listed.out)
	}
	want := map[string]string{"url": "https://jira.test", "project": "PLAT", "query": "status = Open",
		"page_size": "25"}
	if len(sources) != 1 || sources[0].MappingPath != "/etc/tix/platform.yaml" || len(sources[0].Config) != len(want) {
		t.Fatalf("sources = %+v", sources)
	}
	for k, v := range want {
		if sources[0].Config[k] != v {
			t.Errorf("config[%s] = %q, want %q", k, sources[0].Config[k], v)
		}
	}
	table := c.mustRun("sync", "source", "ls")
	if !strings.Contains(table.out, "project=PLAT") || !strings.Contains(table.out, "mapping=/etc/tix/platform.yaml") {
		t.Errorf("the table does not show the stored settings:\n%s", table.out)
	}

	got := c.run("sync", "source", "add", core.SystemGeneric, "ops", "--url", "https://jira.test")
	if got.code != core.ExitUsage {
		t.Errorf("a url on a generic source exited %d, want %d\n%s", got.code, core.ExitUsage, got.err)
	}
}

// runWithKey runs tix with a sealing key configured and stdin fed.
func runWithKey(c *cli, key, stdin string, args ...string) result {
	c.t.Helper()
	var out, errb bytes.Buffer
	code := Run(args, strings.NewReader(stdin), &out, &errb, append(c.environ(), "TIX_AUTH_TOTP_KEY="+key), c.home)
	return result{code: code, out: out.String(), err: errb.String()}
}

func TestSyncSourceSetUploadsTheMappingAndCredentials(t *testing.T) {
	c := newCLI(t)
	key, err := auth.NewTOTPKey()
	if err != nil {
		t.Fatalf("minting key: %v", err)
	}
	const secret = "cli-secret-api-key-0123456789"
	mapping := filepath.Join(t.TempDir(), "laptop.yaml")
	if err := os.WriteFile(mapping, []byte(cliMapping), 0o600); err != nil {
		t.Fatalf("writing mapping: %v", err)
	}
	added := runWithKey(c, key, "", "sync", "source", "add", core.SystemOpenProject, "op", "--url", "https://op.test")
	if added.code != core.ExitOK {
		t.Fatalf("add exited %d: %s", added.code, added.err)
	}
	for _, want := range []string{"sync runs on the server", "missing on the server: a mapping document",
		"missing on the server: credentials"} {
		if !strings.Contains(added.err, want) {
			t.Errorf("add said %q, want it to say %q", added.err, want)
		}
	}

	set := runWithKey(c, key, "", "-o", "json", "sync", "source", "set", "op", "--mapping-file", mapping)
	if set.code != core.ExitOK {
		t.Fatalf("set --mapping-file exited %d: %s", set.code, set.err)
	}
	set = runWithKey(c, key, secret+"\n", "-o", "json", "sync", "source", "set", "op", "--user", "apikey", "--password-stdin")
	if set.code != core.ExitOK {
		t.Fatalf("set --password-stdin exited %d: %s", set.code, set.err)
	}
	var source core.SyncSource
	if err := json.Unmarshal([]byte(set.out), &source); err != nil {
		t.Fatalf("decoding: %v\n%s", err, set.out)
	}
	if source.Mapping != cliMapping || source.Config["url"] != "https://op.test" {
		t.Errorf("set lost the uploaded mapping or the url: %+v", source)
	}
	if source.Resolved == nil || source.Resolved.Credentials.From != core.SyncFromStored || !source.Resolved.Ready() {
		t.Errorf("resolved = %+v, want stored credentials and nothing missing", source.Resolved)
	}
	listed := runWithKey(c, key, "", "sync", "source", "ls")
	for _, got := range []string{set.out, set.err, listed.out, listed.err} {
		if strings.Contains(got, secret) {
			t.Errorf("the credential was printed: %s", got)
		}
	}
	if !strings.Contains(listed.out, "password ...6789") {
		t.Errorf("ls = %q, want the stored password named by its last four characters", listed.out)
	}
}
